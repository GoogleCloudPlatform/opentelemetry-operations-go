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

package metric

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"go.opentelemetry.io/otel/api/global"
	apimetric "go.opentelemetry.io/otel/api/metric"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregator"
	"go.opentelemetry.io/otel/sdk/metric/controller/push"
	integrator "go.opentelemetry.io/otel/sdk/metric/integrator/simple"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	"google.golang.org/api/option"
	googlemetricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

const (
	version                                   = "0.1.0"
	cloudMonitoringMetricDescriptorNameFormat = "custom.googleapis.com/opentelemetry/%s"
)

var (
	errBlankProjectID = errors.New("expecting a non-blank ProjectID")
)

type errUnsupportedAggregation struct {
	agg export.Aggregator
}

func (e errUnsupportedAggregation) Error() string {
	return fmt.Sprintf("currently the aggregator is not supported: %v", e.agg)
}

// key is used to judge the uniqueness of the record descriptor.
type key struct {
	name        string
	libraryname string
}

func keyOf(descriptor *apimetric.Descriptor) key {
	return key{
		name:        descriptor.Name(),
		libraryname: descriptor.LibraryName(),
	}
}

// metricExporter is the implementation of OpenTelemetry metric exporter for
// Google Cloud Monitoring.
type metricExporter struct {
	o *options

	// mdCache is the cache to hold MetricDescriptor to avoid creating duplicate MD.
	// TODO: [ymotongpoo] this map should be goroutine safe with mutex.
	mdCache map[key]*googlemetricpb.MetricDescriptor

	client *monitoring.MetricClient
}

// newMetricExporter returns an exporter that uploads OTel metric data to Google Cloud Monitoring.
func newMetricExporter(o *options) (*metricExporter, error) {
	if strings.TrimSpace(o.ProjectID) == "" {
		return nil, errBlankProjectID
	}

	clientOpts := append(o.MonitoringClientOptions, option.WithUserAgent(userAgent))
	ctx := o.Context
	if ctx == nil {
		ctx = context.Background()
	}
	client, err := monitoring.NewMetricClient(ctx, clientOpts...)
	if err != nil {
		return nil, err
	}
	e := &metricExporter{
		client: client,
		o:      o,
	}
	return e, nil
}

// InstallNewPipeline instantiates a NewExportPipeline and registers it globally.
func InstallNewPipeline(opts []Option, popts ...push.Option) (*push.Controller, error) {
	pusher, err := NewExportPipeline(opts, popts...)
	if err != nil {
		return nil, err
	}
	global.SetMeterProvider(pusher)
	return pusher, err
}

// NewExportPipeline sets up a complete export pipeline with the recommended setup,
// chaining a NewRawExporter into the recommended selectors and integrators.
func NewExportPipeline(opts []Option, popts ...push.Option) (*push.Controller, error) {
	selector := simple.NewWithExactMeasure()
	exporter, err := NewRawExporter(opts...)
	if err != nil {
		return nil, err
	}
	integrator := integrator.New(selector, true)
	period := exporter.metricExporter.o.ReportingInterval
	pusher := push.New(integrator, exporter, period, popts...)
	pusher.Start()
	return pusher, nil
}

// ExportMetrics exports OpenTelemetry Metrics to Google Cloud Monitoring.
func (me *metricExporter) ExportMetrics(ctx context.Context, res *resource.Resource, cps export.CheckpointSet) error {
	if err := me.exportMetricDescriptor(ctx, cps); err != nil {
		return err
	}

	if err := me.exportTimeSeries(ctx, res, cps); err != nil {
		return err
	}
	return nil
}

// exportMetricDescriptor create MetricDescriptor from the record
// if the descriptor is not registered in Cloud Monitoring yet.
func (me *metricExporter) exportMetricDescriptor(ctx context.Context, cps export.CheckpointSet) error {
	mds := make(map[key]*googlemetricpb.MetricDescriptor)
	aggError := cps.ForEach(func(r export.Record) error {
		key := keyOf(r.Descriptor())

		if _, ok := me.mdCache[key]; ok {
			return nil
		}

		if _, localok := mds[key]; !localok {
			md := me.recordToMdpb(&r)
			mds[key] = md
		}
		return nil
	})
	if aggError != nil {
		return aggError
	}
	if len(mds) == 0 {
		return nil
	}

	// TODO: This process is synchronous and blocks longer time if records in cps
	// have many diffrent descriptors. In the cps.ForEach above, it should spawn
	// goroutines to send CreateMetricDescriptorRequest asynchronously in the case
	// the descriptor does not exist in global cache (me.mdCache).
	for key, md := range mds {
		req := &monitoringpb.CreateMetricDescriptorRequest{
			Name:             fmt.Sprintf("projects/%s", me.o.ProjectID),
			MetricDescriptor: md,
		}
		_, err := me.client.CreateMetricDescriptor(ctx, req)
		if err != nil {
			return err
		}
		me.mdCache[key] = md
	}
	return nil
}

// exportTimeSeriees create TimeSeries from the records in cps.
// res should be the common resource among all TimeSeries, such as instance id, application name and so on.
func (me *metricExporter) exportTimeSeries(ctx context.Context, res *resource.Resource, cps export.CheckpointSet) error {
	tss := []*monitoringpb.TimeSeries{}

	aggError := cps.ForEach(func(r export.Record) error {
		ts, err := me.recordToTspb(&r, res)
		if err != nil {
			return err
		}
		tss = append(tss, ts)
		return nil
	})

	if aggError != nil {
		if me.o.onError != nil {
			me.o.onError(aggError)
		} else {
			log.Printf("Error during exporting TimeSeries: %v", aggError)
		}
	}

	req := &monitoringpb.CreateTimeSeriesRequest{
		Name:       fmt.Sprintf("projects/%s", me.o.ProjectID),
		TimeSeries: tss,
	}

	return me.client.CreateTimeSeries(ctx, req)
}

// recordToTspb converts record to TimeSeries proto type with common resource.
// ref. https://cloud.google.com/monitoring/api/ref_v3/rest/v3/TimeSeries
func (me *metricExporter) recordToTspb(r *export.Record, res *resource.Resource) (*monitoringpb.TimeSeries, error) {
	m := me.recordToMpb(r)
	mr := me.resourceToMonitoredResourcepb(res)

	tv, err := recordToTypedValue(r)
	if err != nil {
		return nil, err
	}

	p := &monitoringpb.Point{
		Value: tv,
	}

	return &monitoringpb.TimeSeries{
		Metric:   m,
		Resource: mr,
		Points:   []*monitoringpb.Point{p},
	}, nil
}

// descToMetricType converts descriptor to MetricType proto type.
// Basically this returns default value ("custom.googleapis.com/opentelemetry/[metric type]")
func (me *metricExporter) descToMetricType(desc *apimetric.Descriptor) string {
	if formatter := me.o.MetricDescriptorTypeFormatter; formatter != nil {
		return formatter(desc)
	}
	return fmt.Sprintf(cloudMonitoringMetricDescriptorNameFormat, desc.Name())
}

// recordToMdpb extracts data and converts them to googlemetricpb.MetricDescriptor.
func (me *metricExporter) recordToMdpb(record *export.Record) *googlemetricpb.MetricDescriptor {
	desc := record.Descriptor()
	name := desc.Name()
	unit := record.Descriptor().Unit()
	kind, typ := recordToMdpbKindType(record)

	// Detailed explanations on MetricDescriptor proto is not documented on
	// generated Go packages. Refer to the original proto file.
	// https://github.com/googleapis/googleapis/blob/50af053/google/api/metric.proto#L33
	return &googlemetricpb.MetricDescriptor{
		Name:        name,
		Type:        me.descToMetricType(desc),
		MetricKind:  kind,
		ValueType:   typ,
		Unit:        string(unit),
		Description: desc.Description(),
	}
}

// resourceToMonitoredResourcepb converts resource in OTel to MonitoredResource
// proto type for Cloud Monitoring.
// NOTE:
// https://cloud.google.com/monitoring/api/ref_v3/rest/v3/projects.monitoredResourceDescriptors
func (me *metricExporter) resourceToMonitoredResourcepb(_ *resource.Resource) *monitoredrespb.MonitoredResource {
	// TODO: Implement the process to convert Resource to MonitoringResoruce.
	// ref. https://cloud.google.com/monitoring/api/resources

	// "global" only accepts "project_id" for label.
	// https://cloud.google.com/monitoring/api/resources#tag_global
	return &monitoredrespb.MonitoredResource{
		Type: "global",
		Labels: map[string]string{
			"project_id": me.o.ProjectID,
		},
	}
}

// recordToMdpbKindType return the mapping from OTel's record descriptor to
// Cloud Monitoring's MetricKind and ValueType.
func recordToMdpbKindType(r *export.Record) (googlemetricpb.MetricDescriptor_MetricKind, googlemetricpb.MetricDescriptor_ValueType) {
	// TODO: [ymotongpoo] Remove tentative implementation
	mkind := r.Descriptor().MetricKind()
	nkind := r.Descriptor().NumberKind()

	var kind googlemetricpb.MetricDescriptor_MetricKind
	switch mkind {
	case apimetric.CounterKind:
		kind = googlemetricpb.MetricDescriptor_CUMULATIVE
	case apimetric.ObserverKind:
		kind = googlemetricpb.MetricDescriptor_GAUGE
	// TODO: Add support for MeasureKind.
	default:
		kind = googlemetricpb.MetricDescriptor_METRIC_KIND_UNSPECIFIED
	}

	var typ googlemetricpb.MetricDescriptor_ValueType
	switch nkind {
	case apimetric.Int64NumberKind:
		typ = googlemetricpb.MetricDescriptor_INT64
	case apimetric.Float64NumberKind:
		typ = googlemetricpb.MetricDescriptor_DOUBLE
	case apimetric.Uint64NumberKind:
		// TODO: [ymotongpoo] Confirm if INT64 is ok here.
		typ = googlemetricpb.MetricDescriptor_INT64
	default:
		typ = googlemetricpb.MetricDescriptor_VALUE_TYPE_UNSPECIFIED
	}

	// TODO: Add handling for MeasureKind if necessary.

	return kind, typ
}

// recordToMpb converts data from records to Metric proto type for Cloud Monitoring.
func (me *metricExporter) recordToMpb(r *export.Record) *googlemetricpb.Metric {
	desc := r.Descriptor()
	key := keyOf(desc)
	md, ok := me.mdCache[key]
	if !ok {
		md = me.recordToMdpb(r)
	}

	labels := make(map[string]string)
	iter := r.Labels().Iter()
	for iter.Next() {
		kv := iter.Label()
		labels[string(kv.Key)] = kv.Value.AsString()
	}

	return &googlemetricpb.Metric{
		Type:   md.Type,
		Labels: labels,
	}
}

func recordToTypedValue(r *export.Record) (*monitoringpb.TypedValue, error) {
	desc := r.Descriptor()
	agg := r.Aggregator()
	kind := desc.NumberKind()

	// TODO: Ignoring the case for Min, Max, Count and Distribution to simply
	// the first implementation.
	if lv, ok := agg.(aggregator.LastValue); ok {
		value, _, err := lv.LastValue()
		if err != nil {
			return nil, err
		}
		switch kind {
		case apimetric.Int64NumberKind:
			return &monitoringpb.TypedValue{
				Value: &monitoringpb.TypedValue_Int64Value{
					Int64Value: value.AsInt64(),
				},
			}, nil
		case apimetric.Float64NumberKind:
			return &monitoringpb.TypedValue{
				Value: &monitoringpb.TypedValue_DoubleValue{
					DoubleValue: value.AsFloat64(),
				},
			}, nil
		case apimetric.Uint64NumberKind:
			return &monitoringpb.TypedValue{
				Value: &monitoringpb.TypedValue_Int64Value{
					Int64Value: value.AsInt64(),
				},
			}, nil
		}
	} else if sum, ok := agg.(aggregator.Sum); ok {
		value, err := sum.Sum()
		if err != nil {
			return nil, err
		}
		switch kind {
		case apimetric.Int64NumberKind:
			return &monitoringpb.TypedValue{
				Value: &monitoringpb.TypedValue_Int64Value{
					Int64Value: value.AsInt64(),
				},
			}, nil
		case apimetric.Float64NumberKind:
			return &monitoringpb.TypedValue{
				Value: &monitoringpb.TypedValue_DoubleValue{
					DoubleValue: value.AsFloat64(),
				},
			}, nil
		case apimetric.Uint64NumberKind:
			return &monitoringpb.TypedValue{
				Value: &monitoringpb.TypedValue_Int64Value{
					Int64Value: value.AsInt64(),
				},
			}, nil
		}
	}

	return nil, errUnsupportedAggregation{agg: agg}
}
