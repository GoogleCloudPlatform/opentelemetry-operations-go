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
	"strings"

	"go.opentelemetry.io/otel/api/global"
	apimetric "go.opentelemetry.io/otel/api/metric"
	export "go.opentelemetry.io/otel/sdk/export/metric"
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

// metricExporter is the implementation of OpenTelemetry metric exporter for
// Google Cloud Monitoring.
type metricExporter struct {
	o *options

	// mdCache is the cache to hold MetricDescriptor to avoid creating duplicate MD.
	// TODO: [ymotongpoo] this map should be goroutine safe with mutex.
	mdCache map[string]*googlemetricpb.MetricDescriptor

	client *monitoring.MetricClient
}

// newMetricExporter returns an exporter that uploads OTel metric data to Google Cloud Monitoring.
// Only one Cloud Monitoring exporter should be created per ProjectID per process, any subsequent
// invocations of NewExporter with the same ProjectID will return an error.
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
func InstallNewPipeline(popts ...push.Option) (*push.Controller, error) {
	pusher, err := NewExportPipeline(popts...)
	if err != nil {
		return nil, err
	}
	global.SetMeterProvider(pusher)
	return pusher, err
}

// NewExportPipeline sets up a complete export pipeline with the recommended setup,
// chaining a NewRawExporter into the recommended selectors and batchers.
func NewExportPipeline(popts ...push.Option) (*push.Controller, error) {
	selector := simple.NewWithExactMeasure()
	exporter, err := NewRawExporter()
	if err != nil {
		return nil, err
	}
	integrator := integrator.New(selector, true)
	period := exporter.metricExporter.o.ReportingInterval
	pusher := push.New(integrator, exporter, period, popts...)
	return pusher, nil
}

// ExportMetrics exports OpenTelemetry Metrics to Google Cloud Monitoring.
func (me *metricExporter) ExportMetrics(ctx context.Context, res *resource.Resource, cps export.CheckpointSet) error {
	// TODO: [ymotongpoo] implement this function to create TimeSeries from records.
	// 1. check all records if they have new MetricDescriptor that are not cached.
	// 2. convert record to TimeSeries
	if err := me.exportMetricDescriptor(ctx, cps); err != nil {
		return err
	}

	aggError := cps.ForEach(func(r export.Record) error {
		return nil
	})
	return aggError
}

// exportMetricDescriptor create MetricDescriptor from the record
// if the descriptor is not registerd in Cloud Monitoring yet.
func (me *metricExporter) exportMetricDescriptor(ctx context.Context, cps export.CheckpointSet) error {
	mds := make(map[string]*googlemetricpb.MetricDescriptor)
	cps.ForEach(func(r export.Record) error {
		desc := r.Descriptor()
		name := desc.Name()

		if _, ok := me.mdCache[name]; !ok {
			if _, localok := mds[name]; !localok {
				md := me.recordToMdpb(&r)
				mds[name] = md
			}
		}
		return nil
	})
	if len(mds) == 0 {
		return nil
	}

	for _, md := range mds {
		req := monitoringpb.CreateMetricDescriptorRequest{
			Name:             fmt.Sprintf("projects/%s", me.o.ProjectID),
			MetricDescriptor: md,
		}
		_, err := me.client.CreateMetricDescriptor(ctx, req)
		if err != nil {
			return err
		}
		me.mdCache[md.Name] = md
	}
	return nil
}

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
// https://github.com/googleapis/googleapis/blob/50af053073/google/api/monitored_resource.proto#L86
func (me *metricExporter) resourceToMonitoredResourcepb(res *resource.Resource) *monitoredrespb.MonitoredResource {
	labels := make(map[string]string)
	iter := res.Iter()
	for iter.Next() {
		kv := iter.Label()
		labels[string(kv.Key)] = kv.Value.AsString()
	}

	// TODO: Replace this part with more flexible. Idea is to accept fixed key in res
	// such as "monitored_resource_type" and use its value.
	typ := "global"
	if me.o.MonitoredResource != nil {
		typ = me.o.MonitoredResource.Type
	}

	return &monitoredrespb.MonitoredResource{
		Type:   typ,
		Labels: labels,
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
	case apimetric.MeasureKind:
		kind = googlemetricpb.MetricDescriptor_GAUGE
	case apimetric.CounterKind:
		kind = googlemetricpb.MetricDescriptor_CUMULATIVE
	case apimetric.ObserverKind:
		kind = googlemetricpb.MetricDescriptor_CUMULATIVE
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

	// TODO: [ymotongpoo] Temporarily fix ObserverKind for DISTRIBUTION.
	if mkind == apimetric.ObserverKind {
		typ = googlemetricpb.MetricDescriptor_DISTRIBUTION
	}

	return kind, typ
}

// recordToMpb converts data from records to Metric proto type for Cloud Monitoring.
func (me *metricExporter) recordToMpb(r *export.Record) *googlemetricpb.Metric {
	desc := r.Descriptor()
	name := desc.Name()
	md, ok := me.mdCache[name]
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
