// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file contains the rewritten googlecloud metrics exporter which no longer takes
// dependency on the OpenCensus stackdriver exporter.

package collector

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"
	"unicode"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
	"google.golang.org/genproto/googleapis/api/metric"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// metricsExporter is the GCM exporter that uses pdata directly
type metricsExporter struct {
	cfg    *Config
	client *monitoring.MetricClient
	mapper metricMapper
	// A channel that receives metric descriptor and sends them to GCM once.
	mds chan *metric.MetricDescriptor
}

// metricMapper is the part that transforms metrics. Separate from metricsExporter since it has
// all pure functions.
type metricMapper struct {
	cfg *Config
}

// Known metric domains. Note: This is now configurable for advanced usages.
var domains = []string{"googleapis.com", "kubernetes.io", "istio.io", "knative.dev"}

type labels map[string]string

func (me *metricsExporter) Shutdown(context.Context) error {
	// TODO - will the context given to us at startup be cancelled already?
	return me.client.Close()
}

// Updates config object to include all defaults for metric export.
func (cfg *Config) SetMetricDefaults() {
	if cfg.MetricConfig.KnownDomains == nil || len(cfg.MetricConfig.KnownDomains) == 0 {
		cfg.MetricConfig.KnownDomains = domains
	}
	// TODO - if in legacy mode, this should be
	// external.googleapis.com/OpenCensus/
	if cfg.MetricConfig.Prefix == "" {
		cfg.MetricConfig.Prefix = "workload.googleapis.com"
	}
}

// Updates configuration to include all defaults. Used in unit testing.
func (m *metricMapper) SetMetricDefaults() {
	if m.cfg == nil {
		m.cfg = &Config{}
	}
	m.cfg.SetMetricDefaults()
}

func newGoogleCloudMetricsExporter(
	ctx context.Context,
	cfg *Config,
	set component.ExporterCreateSettings,
) (component.MetricsExporter, error) {
	setVersionInUserAgent(cfg, set.BuildInfo.Version)
	cfg.SetMetricDefaults()

	// map cfg options into metric service client configuration with
	// generateClientOptions()
	clientOpts, err := generateClientOptions(cfg)
	if err != nil {
		return nil, err
	}

	client, err := monitoring.NewMetricClient(ctx, clientOpts...)
	if err != nil {
		return nil, err
	}

	mExp := &metricsExporter{
		cfg:    cfg,
		client: client,
		mapper: metricMapper{cfg},
		mds:    make(chan *metric.MetricDescriptor),
	}

	// Fire up the metric descriptor exporter.
	go mExp.exportMetricDescriptorRunner(ctx)

	return exporterhelper.NewMetricsExporter(
		cfg,
		set,
		mExp.pushMetrics,
		exporterhelper.WithShutdown(mExp.Shutdown),
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: defaultTimeout}),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings))
}

// pushMetrics calls pushes pdata metrics to GCM, creating metric descriptors if necessary
func (me *metricsExporter) pushMetrics(ctx context.Context, m pdata.Metrics) error {
	timeSeries := make([]*monitoringpb.TimeSeries, 0, m.DataPointCount())
	rms := m.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		monitoredResource, extraResourceLabels := me.mapper.resourceMetricsToMonitoredResource(rm.Resource())
		ilms := rm.InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)

			extraLabels := mergeLabels(instrumentationLibraryToLabels(ilm.InstrumentationLibrary()), extraResourceLabels)
			mes := ilm.Metrics()
			for k := 0; k < mes.Len(); k++ {
				metric := mes.At(k)
				me.mds <- me.mapper.metricDescriptor(metric)
				timeSeries = append(timeSeries, me.mapper.metricToTimeSeries(monitoredResource, extraLabels, metric)...)
			}
		}
	}

	// TODO: self observability
	err := me.createTimeSeries(ctx, timeSeries)
	recordPointCount(ctx, len(timeSeries), m.DataPointCount()-len(timeSeries), err)
	return err
}

// Reads metric descriptors from the md channel, and reports them (once) to GCM.
func (me *metricsExporter) exportMetricDescriptorRunner(ctx context.Context) {
	mdCache := make(map[string]*metric.MetricDescriptor)
	for {
		select {
		case md := <-me.mds:
			// Not yet sent, now we sent it.
			if !me.cfg.MetricConfig.SkipCreateMetricDescriptor && md != nil && mdCache[md.Type] == nil {
				err := me.exportMetricDescriptor(ctx, md)
				// TODO: Log-once on error, per metric descriptor?
				if err != nil {
					fmt.Printf("Unable to send metric descriptor: %s, %s", md, err)
				}
				mdCache[md.Type] = md
			}
		case <-ctx.Done():
			// Kill this exporter when context is cancelled.
			return
		}
	}
}

// Helper method to send metric descriptors to GCM.
func (me *metricsExporter) exportMetricDescriptor(ctx context.Context, md *metric.MetricDescriptor) error {
	// export
	req := &monitoringpb.CreateMetricDescriptorRequest{
		// TODO set Name field with project ID from config or ADC
		Name:             fmt.Sprintf("projects/%s", me.cfg.ProjectID),
		MetricDescriptor: md,
	}
	_, err := me.client.CreateMetricDescriptor(ctx, req)
	return err
}

func (me *metricsExporter) createTimeSeries(ctx context.Context, ts []*monitoringpb.TimeSeries) error {
	// TODO: me.client.CreateServiceTimeSeries(
	return me.client.CreateTimeSeries(
		ctx,
		&monitoringpb.CreateTimeSeriesRequest{
			TimeSeries: ts,
			// TODO set Name field with project ID from config or ADC
			Name: fmt.Sprintf("projects/%s", me.cfg.ProjectID),
		},
	)
}

// Transforms pdata Resource to a GCM Monitored Resource. Any resource attributes not accounted
// for in the monitored resource which should be merged into metric labels are also returned.
func (m *metricMapper) resourceMetricsToMonitoredResource(
	resource pdata.Resource,
) (*monitoredrespb.MonitoredResource, labels) {
	// TODO
	return nil, nil
}

func instrumentationLibraryToLabels(il pdata.InstrumentationLibrary) labels {
	// TODO
	return nil
}

func (m *metricMapper) metricToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pdata.Metric,
) []*monitoringpb.TimeSeries {
	timeSeries := []*monitoringpb.TimeSeries{}

	switch metric.DataType() {
	case pdata.MetricDataTypeSum:
		sum := metric.Sum()
		points := sum.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.sumPointToTimeSeries(resource, extraLabels, metric, sum, points.At(i))
			timeSeries = append(timeSeries, ts)
		}
	case pdata.MetricDataTypeGauge:
		gauge := metric.Gauge()
		points := gauge.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.gaugePointToTimeSeries(resource, extraLabels, metric, gauge, points.At(i))
			timeSeries = append(timeSeries, ts)
		}
	// TODO: add cases for other metric data types
	default:
		// TODO: log unsupported metric
	}

	return timeSeries
}

func (m *metricMapper) sumPointToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pdata.Metric,
	sum pdata.Sum,
	point pdata.NumberDataPoint,
) *monitoringpb.TimeSeries {
	metricKind := metricpb.MetricDescriptor_CUMULATIVE
	startTime := timestamppb.New(point.StartTimestamp().AsTime())
	if !sum.IsMonotonic() {
		metricKind = metricpb.MetricDescriptor_GAUGE
		startTime = nil
	}
	value, valueType := numberDataPointToValue(point)

	return &monitoringpb.TimeSeries{
		Resource:   resource,
		Unit:       metric.Unit(),
		MetricKind: metricKind,
		ValueType:  valueType,
		Points: []*monitoringpb.Point{{
			Interval: &monitoringpb.TimeInterval{
				StartTime: startTime,
				EndTime:   timestamppb.New(point.Timestamp().AsTime()),
			},
			Value: value,
		}},
		Metric: &metricpb.Metric{
			Type: m.metricNameToType(metric.Name()),
			Labels: mergeLabels(
				attributesToLabels(point.Attributes()),
				extraLabels,
			),
		},
	}
}

func (m *metricMapper) gaugePointToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pdata.Metric,
	gauge pdata.Gauge,
	point pdata.NumberDataPoint,
) *monitoringpb.TimeSeries {
	metricKind := metricpb.MetricDescriptor_GAUGE
	value, valueType := numberDataPointToValue(point)

	return &monitoringpb.TimeSeries{
		Resource:   resource,
		Unit:       metric.Unit(),
		MetricKind: metricKind,
		ValueType:  valueType,
		Points: []*monitoringpb.Point{{
			Interval: &monitoringpb.TimeInterval{
				EndTime: timestamppb.New(point.Timestamp().AsTime()),
			},
			Value: value,
		}},
		Metric: &metricpb.Metric{
			Type: m.metricNameToType(metric.Name()),
			Labels: mergeLabels(
				attributesToLabels(point.Attributes()),
				extraLabels,
			),
		},
	}
}

// Returns any configured prefix to add to unknown metric name.
func (me *metricMapper) getMetricNamePrefix(name string) *string {
	for _, domain := range me.cfg.MetricConfig.KnownDomains {
		if strings.Contains(name, domain) {
			return nil
		}
	}
	result := me.cfg.MetricConfig.Prefix
	return &result
}

// metricNameToType maps OTLP metric name to GCM metric type (aka name)
func (m *metricMapper) metricNameToType(name string) string {
	prefix := m.getMetricNamePrefix(name)
	if prefix != nil {
		return path.Join(*prefix, name)
	}
	return name
}

func numberDataPointToValue(
	point pdata.NumberDataPoint,
) (*monitoringpb.TypedValue, metricpb.MetricDescriptor_ValueType) {
	if point.Type() == pdata.MetricValueTypeInt {
		return &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_Int64Value{
				Int64Value: point.IntVal(),
			}},
			metricpb.MetricDescriptor_INT64
	}
	return &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{
			DoubleValue: point.DoubleVal(),
		}},
		metricpb.MetricDescriptor_DOUBLE
}

func attributesToLabels(
	attrs pdata.AttributeMap,
) labels {
	// TODO
	ls := make(labels, attrs.Len())
	attrs.Range(func(k string, v pdata.AttributeValue) bool {
		ls[sanitizeKey(k)] = v.AsString()
		return true
	})
	return ls
}

// Replaces non-alphanumeric characters to underscores. Note, this does not truncate label keys
// longer than 100 characters or prepend "key" when the first character is "_" like OpenCensus
// did.
func sanitizeKey(s string) string {
	if len(s) == 0 {
		return s
	}
	s = strings.Map(sanitizeRune, s)
	if unicode.IsDigit(rune(s[0])) {
		s = "key_" + s
	}
	return s
}

// converts anything that is not a letter or digit to an underscore
func sanitizeRune(r rune) rune {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return r
	}
	// Everything else turns into an underscore
	return '_'
}

func mergeLabels(mergeInto labels, others ...labels) labels {
	if mergeInto == nil {
		mergeInto = labels{}
	}
	for _, ls := range others {
		for k, v := range ls {
			mergeInto[k] = v
		}
	}

	return mergeInto
}

// Takes a GCM metric type, like (workload.googleapis.com/MyCoolMetric) and returns the display name.
func (m *metricMapper) metricTypeToDisplayName(mUrl string) string {
	// TODO - user configuration around display name?
	// Default: strip domain, keep path after domain.
	u, err := url.Parse(fmt.Sprintf("metrics://%s", mUrl))
	if err != nil {
		return mUrl
	}
	return strings.TrimLeft(u.Path, "/")
}

// Extract the metric descriptor from a metric data point.
func (me *metricMapper) metricDescriptor(m pdata.Metric) *metric.MetricDescriptor {
	kind, typ := mapMetricPointKind(m)
	metricType := me.metricNameToType(m.Name())
	// Return nil for unsupported types.
	if kind == metric.MetricDescriptor_METRIC_KIND_UNSPECIFIED {
		return nil
	}
	return &metric.MetricDescriptor{
		Name:        m.Name(),
		DisplayName: me.metricTypeToDisplayName(metricType),
		Type:        metricType,
		MetricKind:  kind,
		ValueType:   typ,
		Unit:        m.Unit(),
		Description: m.Description(),
		// TODO: Labels for non-wlm
	}
}

func metricPointValueType(pt pdata.MetricValueType) metric.MetricDescriptor_ValueType {
	switch pt {
	case pdata.MetricValueTypeInt:
		return metric.MetricDescriptor_INT64
	case pdata.MetricValueTypeDouble:
		return metric.MetricDescriptor_DOUBLE
	default:
		return metric.MetricDescriptor_VALUE_TYPE_UNSPECIFIED
	}
}

func mapMetricPointKind(m pdata.Metric) (metric.MetricDescriptor_MetricKind, metric.MetricDescriptor_ValueType) {
	var kind metric.MetricDescriptor_MetricKind
	var typ metric.MetricDescriptor_ValueType
	switch m.DataType() {
	case pdata.MetricDataTypeGauge:
		kind = metric.MetricDescriptor_GAUGE
		if m.Gauge().DataPoints().Len() > 0 {
			typ = metricPointValueType(m.Gauge().DataPoints().At(0).Type())
		}
	case pdata.MetricDataTypeSum:
		if !m.Sum().IsMonotonic() {
			kind = metric.MetricDescriptor_GAUGE
		} else if m.Sum().AggregationTemporality() == pdata.MetricAggregationTemporalityDelta {
			// We report fake-deltas for now.
			kind = metric.MetricDescriptor_CUMULATIVE
		} else {
			kind = metric.MetricDescriptor_CUMULATIVE
		}
		if m.Sum().DataPoints().Len() > 0 {
			typ = metricPointValueType(m.Sum().DataPoints().At(0).Type())
		}
	case pdata.MetricDataTypeSummary:
		kind = metric.MetricDescriptor_GAUGE
	case pdata.MetricDataTypeHistogram:
		typ = metric.MetricDescriptor_DISTRIBUTION
		if m.Histogram().AggregationTemporality() == pdata.MetricAggregationTemporalityDelta {
			// We report fake-deltas for now.
			kind = metric.MetricDescriptor_CUMULATIVE
		} else {
			kind = metric.MetricDescriptor_CUMULATIVE
		}
	case pdata.MetricDataTypeExponentialHistogram:
		typ = metric.MetricDescriptor_DISTRIBUTION
		if m.ExponentialHistogram().AggregationTemporality() == pdata.MetricAggregationTemporalityDelta {
			// We report fake-deltas for now.
			kind = metric.MetricDescriptor_CUMULATIVE
		} else {
			kind = metric.MetricDescriptor_CUMULATIVE
		}
	default:
		kind = metric.MetricDescriptor_METRIC_KIND_UNSPECIFIED
		typ = metric.MetricDescriptor_VALUE_TYPE_UNSPECIFIED
	}
	return kind, typ
}
