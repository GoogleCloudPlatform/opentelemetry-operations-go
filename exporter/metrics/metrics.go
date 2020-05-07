// Copyright 2020, Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	"github.com/golang/protobuf/ptypes/timestamp"
	"go.opentelemetry.io/otel/api/core"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/metric"
	otel "go.opentelemetry.io/otel/sdk"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregator"
	"go.opentelemetry.io/otel/sdk/metric/batcher/ungrouped"
	"go.opentelemetry.io/otel/sdk/metric/controller/push"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/api/option"
	labelpb "google.golang.org/genproto/googleapis/api/label"
	googlemetricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

const (
	// maxTimeSeriesPerUpload    = 200
	operationsTaskKey         = "operations_task"
	operationsTaskDescription = "Operations task identifier"
	defaultDisplayNamePrefix  = "CloudMonitoring"
	version                   = "0.2.3" // TODO(ymotongpoo): Sort out how to keep up with the release version automatically.
)

var userAgent = fmt.Sprintf("opentelemetry-go %s; metrics-exporter %s", otel.Version(), version)

// metricsExporter exports OTel metrics to the Google Cloud Monitoring.
type metricsExporter struct {
	o *options

	// protoMu                sync.Mutex
	protoMetricDescriptors map[string]bool // Metric descriptors that were already created remotely

	metricMu          sync.Mutex
	metricDescriptors map[string]bool // Metric descriptors that were already created remotely

	client        *monitoring.MetricClient
	defaultLabels map[string]labelValue
	// ir            *metricexport.IntervalReader

	// initReaderOnce sync.Once
}

var (
	errBlankProjectID = errors.New("expecting a non-blank ProjectID")
)

// InstallNewPipeline instantiates a NewExportPipeline and registers it globally.
func InstallNewPipeline(opts ...Option) (*push.Controller, error) {
	pusher, err := NewExportPipeline(opts...)
	if err != nil {
		return nil, err
	}
	global.SetMeterProvider(pusher)
	return pusher, err
}

// NewExportPipeline sets up a complete export pipeline with the recommended setup,
// chaining a NewRawExporter into the recommended selectors and batchers.
func NewExportPipeline(opts ...Option) (*push.Controller, error) {
	selector := simple.NewWithExactMeasure()
	exporter, err := NewRawExporter()
	if err != nil {
		return nil, err
	}
	batcher := ungrouped.New(selector, true)
	period := exporter.metricsExporter.o.ReportingInterval
	pusher := push.New(batcher, exporter, period)
	return pusher, nil
}

// newMetricsExporter returns an exporter that uploads stats data to Google Cloud Monitoring.
// Only one Cloud Monitoring exporter should be created per ProjectID per process, any subsequent
// invocations of NewExporter with the same ProjectID will return an error.
func newMetricsExporter(o *options) (*metricsExporter, error) {
	if strings.TrimSpace(o.ProjectID) == "" {
		return nil, errBlankProjectID
	}

	opts := append(o.MonitoringClientOptions, option.WithUserAgent(userAgent))
	ctx := o.Context
	if ctx == nil {
		ctx = context.Background()
	}
	client, err := monitoring.NewMetricClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	e := &metricsExporter{
		client:                 client,
		o:                      o,
		protoMetricDescriptors: make(map[string]bool),
		metricDescriptors:      make(map[string]bool),
	}

	var defaultLablesNotSanitized map[string]labelValue
	if o.DefaultMonitoringLabels != nil {
		defaultLablesNotSanitized = o.DefaultMonitoringLabels.m
	} else {
		defaultLablesNotSanitized = map[string]labelValue{
			operationsTaskKey: {val: getTaskValue(), desc: operationsTaskDescription},
		}
	}

	e.defaultLabels = make(map[string]labelValue)
	// Fill in the defaults firstly, irrespective of if the labelKeys and labelValues are mismatched.
	for key, label := range defaultLablesNotSanitized {
		e.defaultLabels[sanitize(key)] = label
	}

	return e, nil
}

// ExportMetrics exports OpenTelemetry Metrics to Google Cloud Monitoring.
// TODO: [ymotongpoo] Ignores global *resource.Resource on initial implementation.
func (me *metricsExporter) ExportMetrics(ctx context.Context, _ *resource.Resource, cps export.CheckpointSet) error {
	var aggError error

	ctx, cancel := newContextWithTimeout(me.o.Context, me.o.Timeout)
	defer cancel()

	aggError = cps.ForEach(func(record export.Record) error {
		if err := me.createMetricDescriptorFromRecord(ctx, &record); err != nil {
			return err
		}
		return nil
	})
	if aggError != nil {
		return aggError
	}

	// TODO: [ymotongpoo] add process to add auto detected labels to pb data.
	_ = getAutodetectedLabels()

	timeSeries := []*monitoringpb.TimeSeries{}
	aggError = cps.ForEach(func(record export.Record) error {
		desc := record.Descriptor()
		agg := record.Aggregator()
		kind := desc.NumberKind()
		labels := record.Labels().ToSlice()

		metricKind, _ := recordDescriptorTypeToMetricKind(&record)
		if metricKind == googlemetricpb.MetricDescriptor_METRIC_KIND_UNSPECIFIED {
			// ignore this record. TODO [ymotongpoo] log errors.
			return nil
		}

		var ts *monitoringpb.TimeSeries
		var err error
		if sum, ok := agg.(aggregator.Sum); ok {
			ts, err = me.aggToSumTs(sum, kind, desc, labels)
			if err != nil {
				return fmt.Errorf("exporting sum: %w", err)
			}
		} else if count, ok := agg.(aggregator.Count); ok {
			ts, err = me.aggToCounterTs(count, kind, desc, labels)
			if err != nil {
				return fmt.Errorf("exporting counter: %w", err)
			}
		} else if lv, ok := agg.(aggregator.LastValue); ok {
			ts, err = me.aggToLastValueTs(lv, kind, desc, labels)
			if err != nil {
				return fmt.Errorf("exporting last value: %w", err)
			}
		}
		// TODO: [ymotongpoo] Add logs in the case for other aggregators.
		if ts == nil {
			return nil
		}

		timeSeries = append(timeSeries, ts)

		return nil
	})

	return aggError
}

func (me *metricsExporter) aggToSumTs(sum aggregator.Sum, kind core.NumberKind, desc *metric.Descriptor, labels []core.KeyValue) (*monitoringpb.TimeSeries, error) {
	recordName := desc.Name()
	metricType := me.metricTypeFromProto(recordName)

	mlabels := make(map[string]string)
	for _, kv := range labels {
		mlabels[string(kv.Key)] = kv.Value.AsString()
	}

	// TODO: [ymotongpoo] find *monitoredrespb.MonitoredResource.Type from metric.Descriptor
	rsc := &monitoredrespb.MonitoredResource{
		Type:   "global",
		Labels: nil,
	}

	now := time.Now()
	startTime := &timestamp.Timestamp{
		Seconds: now.Unix(),
		Nanos:   int32(now.Nanosecond()),
	}

	num, err := sum.Sum()
	if err != nil {
		return nil, err
	}
	tv, err := aggValueToMpbTypeValue(num, kind)
	if err != nil {
		return nil, err
	}

	p := &monitoringpb.Point{
		Interval: &monitoringpb.TimeInterval{
			StartTime: startTime,
		},
		Value: tv,
	}

	ts := &monitoringpb.TimeSeries{
		Metric: &googlemetricpb.Metric{
			Type:   metricType,
			Labels: mlabels,
		},
		Resource: rsc,
		Points:   []*monitoringpb.Point{p},
	}
	return ts, nil
}

func (me *metricsExporter) aggToCounterTs(count aggregator.Count, kind core.NumberKind, desc *metric.Descriptor, labels []core.KeyValue) (*monitoringpb.TimeSeries, error) {
	return nil, nil
}

func (me *metricsExporter) aggToLastValueTs(lv aggregator.LastValue, kind core.NumberKind, desc *metric.Descriptor, labels []core.KeyValue) (*monitoringpb.TimeSeries, error) {
	return nil, nil
}

// createMetricDescriptorFromRecord creates a metric descriptor from the OpenTelemetry Record
// and then creates it remotely using Cloud Monitoring's API.
func (me *metricsExporter) createMetricDescriptorFromRecord(ctx context.Context, record *export.Record) error {
	// Skip create metric descriptor if configured
	if me.o.SkipCMD {
		return nil
	}

	me.metricMu.Lock()
	defer me.metricMu.Unlock()

	name := record.Descriptor().Name()
	if _, created := me.metricDescriptors[name]; created {
		return nil
	}

	if builtinMetric(me.metricTypeFromProto(name)) {
		me.metricDescriptors[name] = true
		return nil
	}

	// Otherwise, we encountered a cache-miss and
	// should create the metric descriptor remotely.
	inMD, err := me.recordToMpbMetricDescriptor(record)
	if err != nil {
		return err
	}

	if err = me.createMetricDescriptor(ctx, inMD); err != nil {
		return err
	}

	// Now record the metric as having been created.
	me.metricDescriptors[name] = true
	return nil
}

func (me *metricsExporter) recordToMpbMetricDescriptor(record *export.Record) (*googlemetricpb.MetricDescriptor, error) {
	if record == nil {
		return nil, errNilMetricOrMetricDescriptor
	}

	name := record.Descriptor().Name()
	metricType := me.metricTypeFromProto(name)
	displayName := me.displayName(name)
	metricKind, valueType := recordDescriptorTypeToMetricKind(record)

	sdm := &googlemetricpb.MetricDescriptor{
		Name:        fmt.Sprintf("projects/%s/metricDescriptors/%s", me.o.ProjectID, metricType),
		DisplayName: displayName,
		Description: record.Descriptor().Description(),
		Unit:        string(record.Descriptor().Unit()),
		Type:        metricType,
		MetricKind:  metricKind,
		ValueType:   valueType,
		Labels:      recordKeysToLabels(me.defaultLabels, record),
	}

	return sdm, nil
}

func recordKeysToLabels(defaults map[string]labelValue, record *export.Record) []*labelpb.LabelDescriptor {
	labels := record.Labels()
	labelDescriptors := make([]*labelpb.LabelDescriptor, 0, len(defaults)+labels.Len())

	// Fill in the defaults first.
	for key, lbl := range defaults {
		labelDescriptors = append(labelDescriptors, &labelpb.LabelDescriptor{
			Key:         sanitize(key),
			Description: lbl.desc,
			ValueType:   labelpb.LabelDescriptor_STRING,
		})
	}

	// Now fill in those from the key.
	iter := labels.Iter()
	for iter.Next() {
		k := iter.Label().Key
		labelDescriptors = append(labelDescriptors, &labelpb.LabelDescriptor{
			Key:         sanitize(string(k)),
			Description: "",                             // core.Key doesn't have descriptions so leave this as empty string
			ValueType:   labelpb.LabelDescriptor_STRING, // We only use string tags
		})
	}
	return labelDescriptors
}

func recordDescriptorTypeToMetricKind(record *export.Record) (googlemetricpb.MetricDescriptor_MetricKind, googlemetricpb.MetricDescriptor_ValueType) {
	if record == nil {
		return googlemetricpb.MetricDescriptor_METRIC_KIND_UNSPECIFIED, googlemetricpb.MetricDescriptor_VALUE_TYPE_UNSPECIFIED
	}

	switch record.Descriptor().MetricKind() {
	case metric.MeasureKind:
		switch record.Descriptor().NumberKind() {
		case core.Int64NumberKind:
			return googlemetricpb.MetricDescriptor_GAUGE, googlemetricpb.MetricDescriptor_INT64
		case core.Float64NumberKind:
			return googlemetricpb.MetricDescriptor_GAUGE, googlemetricpb.MetricDescriptor_DOUBLE
		}
	case metric.CounterKind:
		switch record.Descriptor().NumberKind() {
		case core.Int64NumberKind:
			return googlemetricpb.MetricDescriptor_CUMULATIVE, googlemetricpb.MetricDescriptor_INT64
		case core.Float64NumberKind:
			return googlemetricpb.MetricDescriptor_CUMULATIVE, googlemetricpb.MetricDescriptor_DOUBLE
		}
	case metric.ObserverKind:
		// TODO: [ymotongpoo] find the best way to match distribution.
		fallthrough
	default:
		// TODO: [rghetia] after upgrading to proto version3, retrun UNRECOGNIZED instead of UNSPECIFIED
		return googlemetricpb.MetricDescriptor_METRIC_KIND_UNSPECIFIED, googlemetricpb.MetricDescriptor_VALUE_TYPE_UNSPECIFIED
	}
	// TODO: [ymotongpoo] gopls returns error with no return statement
	return googlemetricpb.MetricDescriptor_METRIC_KIND_UNSPECIFIED, googlemetricpb.MetricDescriptor_VALUE_TYPE_UNSPECIFIED
}

func (me *metricsExporter) metricRscToMpbRsc(rs *resource.Resource) *monitoredrespb.MonitoredResource {
	if rs == nil {
		rscField := me.o.Resource
		if rscField == nil {
			rscField = &monitoredrespb.MonitoredResource{
				Type: "global",
			}
		}
		return rscField
	}

	// TODO: [ymotongpoo] OTel's resource doesn't have type, so currently leave "global"
	// as default value
	var typ string
	if typ == "" {
		typ = "global"
	}
	mrsp := &monitoredrespb.MonitoredResource{
		Type: typ,
	}
	attributes := rs.Attributes()
	if len(attributes) > 0 {
		mrsp.Labels = make(map[string]string, len(attributes))
		for _, kv := range attributes {
			// TODO: [rghetia] add mapping between OC Labels and SD Labels.
			mrsp.Labels[string(kv.Key)] = kv.Value.AsString()
		}
	}
	return mrsp
}

func (me *metricsExporter) createMetricDescriptor(ctx context.Context, md *googlemetricpb.MetricDescriptor) error {
	ctx, cancel := newContextWithTimeout(ctx, me.o.Timeout)
	defer cancel()
	cmrdesc := &monitoringpb.CreateMetricDescriptorRequest{
		Name:             fmt.Sprintf("projects/%s", me.o.ProjectID),
		MetricDescriptor: md,
	}
	_, err := createMetricDescriptor(ctx, me.client, cmrdesc)
	return err
}

func createMetricDescriptor(ctx context.Context, c *monitoring.MetricClient, mdr *monitoringpb.CreateMetricDescriptorRequest) (*googlemetricpb.MetricDescriptor, error) {
	return c.CreateMetricDescriptor(ctx, mdr)
}

func (me *metricsExporter) displayName(suffix string) string {
	return path.Join(defaultDisplayNamePrefix, suffix)
}

// getTaskValue returns a task label value in the format of
// "go-<pid>@<hostname>".
func getTaskValue() string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}
	return "go-" + strconv.Itoa(os.Getpid()) + "@" + hostname
}

var knownExternalMetricPrefixes = []string{
	"custom.googleapis.com/",
	"external.googleapis.com/",
}

// builtinMetric returns true if a MetricType is a heuristically known
// built-in Stackdriver metric
func builtinMetric(metricType string) bool {
	for _, knownExternalMetric := range knownExternalMetricPrefixes {
		if strings.HasPrefix(metricType, knownExternalMetric) {
			return false
		}
	}
	return true
}
