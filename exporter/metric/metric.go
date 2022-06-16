// Copyright 2021 Google LLC
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
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/export"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/number"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/sdkapi"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"google.golang.org/api/option"
	apioption "google.golang.org/api/option"
	"google.golang.org/genproto/googleapis/api/distribution"
	googlemetricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	cloudMonitoringMetricDescriptorNameFormat = "custom.googleapis.com/opentelemetry/%s"
)

// key is used to judge the uniqueness of the record descriptor.
type key struct {
	name        string
	libraryname string
}

func keyOf(descriptor *sdkapi.Descriptor, library instrumentation.Library) key {
	return key{
		name:        descriptor.Name(),
		libraryname: library.Name,
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

	// startTime is the cache of start time shared among all CUMULATIVE values.
	// c.f. https://cloud.google.com/monitoring/api/ref_v3/rest/v3/projects.metricDescriptors#MetricKind
	//  TODO: Remove this when OTel SDK provides start time for each record specifically for stateful batcher.
	startTime time.Time
}

// Below are maps with monitored resources fields as keys
// and OpenTelemetry resources fields as values
var k8sContainerMap = map[string]string{
	"location":       CloudKeyZone,
	"cluster_name":   K8SKeyClusterName,
	"namespace_name": K8SKeyNamespaceName,
	"pod_name":       K8SKeyPodName,
	"container_name": ContainerKeyName,
}

var k8sNodeMap = map[string]string{
	"location":     CloudKeyZone,
	"cluster_name": K8SKeyClusterName,
	"node_name":    HostKeyName,
}

var k8sClusterMap = map[string]string{
	"location":     CloudKeyZone,
	"cluster_name": K8SKeyClusterName,
}

var k8sPodMap = map[string]string{
	"location":       CloudKeyZone,
	"cluster_name":   K8SKeyClusterName,
	"namespace_name": K8SKeyNamespaceName,
	"pod_name":       K8SKeyPodName,
}

var gceResourceMap = map[string]string{
	"instance_id": HostKeyID,
	"zone":        CloudKeyZone,
}

var awsResourceMap = map[string]string{
	"instance_id": HostKeyID,
	"region":      CloudKeyRegion,
	"aws_account": CloudKeyAccountID,
}

var cloudRunResourceMap = map[string]string{
	"task_id":   ServiceKeyInstanceID,
	"location":  CloudKeyRegion,
	"namespace": ServiceKeyNamespace,
	"job":       ServiceKeyName,
}

// newMetricExporter returns an exporter that uploads OTel metric data to Google Cloud Monitoring.
func newMetricExporter(o *options) (*metricExporter, error) {
	if strings.TrimSpace(o.projectID) == "" {
		return nil, errBlankProjectID
	}

	clientOpts := append([]apioption.ClientOption{option.WithUserAgent(userAgent)}, o.monitoringClientOptions...)
	ctx := o.context
	if ctx == nil {
		ctx = context.Background()
	}
	client, err := monitoring.NewMetricClient(ctx, clientOpts...)
	if err != nil {
		return nil, err
	}

	cache := map[key]*googlemetricpb.MetricDescriptor{}
	e := &metricExporter{
		o:         o,
		mdCache:   cache,
		client:    client,
		startTime: time.Now(),
	}
	return e, nil
}

// InstallNewPipeline instantiates a NewExportPipeline and registers it globally.
func InstallNewPipeline(opts []Option, popts ...controller.Option) (*controller.Controller, error) {
	pusher, err := NewExportPipeline(opts, popts...)
	if err != nil {
		return nil, err
	}
	global.SetMeterProvider(pusher)
	return pusher, err
}

// NewExportPipeline sets up a complete export pipeline with the recommended setup,
// chaining a NewRawExporter into the recommended selectors and integrators.
func NewExportPipeline(opts []Option, popts ...controller.Option) (*controller.Controller, error) {
	selector := simple.NewWithHistogramDistribution()
	exporter, err := NewRawExporter(opts...)
	if err != nil {
		return nil, err
	}
	period := exporter.metricExporter.o.reportingInterval
	checkpointer := processor.NewFactory(selector, exporter)

	pusher := controller.New(
		checkpointer,
		append([]controller.Option{
			controller.WithExporter(exporter),
			controller.WithCollectPeriod(period),
		}, popts...)...,
	)
	pusher.Start(context.Background())
	return pusher, nil
}

// ExportMetrics exports OpenTelemetry Metrics to Google Cloud Monitoring.
func (me *metricExporter) ExportMetrics(ctx context.Context, res *resource.Resource, ilr export.InstrumentationLibraryReader) error {
	if err := me.exportMetricDescriptor(ctx, res, ilr); err != nil {
		return err
	}

	if err := me.exportTimeSeries(ctx, res, ilr); err != nil {
		return err
	}
	return nil
}

// exportMetricDescriptor create MetricDescriptor from the record
// if the descriptor is not registered in Cloud Monitoring yet.
func (me *metricExporter) exportMetricDescriptor(ctx context.Context, res *resource.Resource, ilr export.InstrumentationLibraryReader) error {
	mds := make(map[key]*googlemetricpb.MetricDescriptor)
	aggError := ilr.ForEach(func(library instrumentation.Library, reader export.Reader) error {
		return reader.ForEach(aggregation.CumulativeTemporalitySelector(), func(r export.Record) error {
			k := keyOf(r.Descriptor(), library)

			if _, ok := me.mdCache[k]; ok {
				return nil
			}

			if _, localok := mds[k]; !localok {
				md := me.recordToMdpb(&r)
				mds[k] = md
			}
			return nil
		})

	})
	if aggError != nil {
		return aggError
	}
	if len(mds) == 0 {
		return nil
	}

	// TODO: This process is synchronous and blocks longer time if records in cps
	// have many different descriptors. In the cps.ForEach above, it should spawn
	// goroutines to send CreateMetricDescriptorRequest asynchronously in the case
	// the descriptor does not exist in global cache (me.mdCache).
	// See details in #26.
	for kmd, md := range mds {
		err := me.createMetricDescriptorIfNeeded(ctx, md)
		if err != nil {
			return err
		}
		me.mdCache[kmd] = md
	}
	return nil
}

func (me *metricExporter) createMetricDescriptorIfNeeded(ctx context.Context, md *googlemetricpb.MetricDescriptor) error {
	mdReq := &monitoringpb.GetMetricDescriptorRequest{
		Name: fmt.Sprintf("projects/%s/metricDescriptors/%s", me.o.projectID, md.Type),
	}
	_, err := me.client.GetMetricDescriptor(ctx, mdReq)
	if err == nil {
		// If the metric descriptor already exists, skip the CreateMetricDescriptor call.
		// Metric descriptors cannot be updated without deleting them first, so there
		// isn't anything we can do here:
		// https://cloud.google.com/monitoring/custom-metrics/creating-metrics#md-modify
		return nil
	}
	req := &monitoringpb.CreateMetricDescriptorRequest{
		Name:             fmt.Sprintf("projects/%s", me.o.projectID),
		MetricDescriptor: md,
	}
	_, err = me.client.CreateMetricDescriptor(ctx, req)
	return err
}

// exportTimeSeries create TimeSeries from the records in cps.
// res should be the common resource among all TimeSeries, such as instance id, application name and so on.
func (me *metricExporter) exportTimeSeries(ctx context.Context, res *resource.Resource, ilr export.InstrumentationLibraryReader) error {
	tss := []*monitoringpb.TimeSeries{}

	aggError := ilr.ForEach(func(library instrumentation.Library, reader export.Reader) error {
		return reader.ForEach(aggregation.CumulativeTemporalitySelector(), func(r export.Record) error {
			ts, err := me.recordToTspb(&r, res, library)
			if err != nil {
				return err
			}
			tss = append(tss, ts)
			return nil
		})
	})

	if len(tss) == 0 {
		return nil
	}

	if aggError != nil {
		if me.o.onError != nil {
			me.o.onError(aggError)
		} else {
			log.Printf("Error during exporting TimeSeries: %v", aggError)
		}
	}

	// TODO: When this exporter is rewritten, support writing to multiple
	// projects based on the "gcp.project.id" resource.
	req := &monitoringpb.CreateTimeSeriesRequest{
		Name:       fmt.Sprintf("projects/%s", me.o.projectID),
		TimeSeries: tss,
	}

	return me.client.CreateTimeSeries(ctx, req)
}

// descToMetricType converts descriptor to MetricType proto type.
// Basically this returns default value ("custom.googleapis.com/opentelemetry/[metric type]")
func (me *metricExporter) descToMetricType(desc *sdkapi.Descriptor) string {
	if formatter := me.o.metricDescriptorTypeFormatter; formatter != nil {
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

// refer to the monitored resources fields
// https://cloud.google.com/monitoring/api/resources
func subdivideGCPTypes(labelMap map[string]string) (string, map[string]string) {
	_, hasLocation := labelMap[CloudKeyZone]
	_, hasClusterName := labelMap[K8SKeyClusterName]
	_, hasNamespaceName := labelMap[K8SKeyNamespaceName]
	_, hasPodName := labelMap[K8SKeyPodName]
	_, hasContainerName := labelMap[ContainerKeyName]

	if hasLocation && hasClusterName && hasNamespaceName && hasPodName && hasContainerName {
		return K8SContainer, k8sContainerMap
	}

	_, hasNodeName := labelMap[HostKeyName]

	if hasLocation && hasClusterName && hasNodeName {
		return K8SNode, k8sNodeMap
	}

	if hasLocation && hasClusterName && hasNamespaceName && hasPodName {
		return K8SPod, k8sPodMap
	}

	if hasLocation && hasClusterName {
		return K8SCluster, k8sClusterMap
	}

	if labelMap[ServiceKeyNamespace] == "cloud-run-managed" {
		return GenericTask, cloudRunResourceMap
	}

	return GCEInstance, gceResourceMap
}

// resourceToMonitoredResourcepb converts resource in OTel to MonitoredResource
// proto type for Cloud Monitoring.
//
// https://cloud.google.com/monitoring/api/ref_v3/rest/v3/projects.monitoredResourceDescriptors
func (me *metricExporter) resourceToMonitoredResourcepb(res *resource.Resource) *monitoredrespb.MonitoredResource {

	monitoredRes := &monitoredrespb.MonitoredResource{
		Type: "global",
		Labels: map[string]string{
			"project_id": sanitizeUTF8(me.o.projectID),
		},
	}

	// Return "global" Monitored resources if the input resource is null or empty
	// "global" only accepts "project_id" for label.
	// https://cloud.google.com/monitoring/api/resources#tag_global
	if res == nil || res.Len() == 0 {
		return monitoredRes
	}

	resLabelMap := generateResLabelMap(res)

	resTypeStr := "global"
	match := map[string]string{}

	if resType, found := resLabelMap[CloudKeyProvider]; found {
		switch resType {
		case CloudProviderGCP:
			resTypeStr, match = subdivideGCPTypes(resLabelMap)
		case CloudProviderAWS:
			resTypeStr = AWSEC2Instance
			match = awsResourceMap
		}

		outputMap, isMissing := transformResource(match, resLabelMap)
		if isMissing {
			resTypeStr = "global"
		} else {
			monitoredRes.Labels = outputMap
		}
	}

	monitoredRes.Type = resTypeStr
	monitoredRes.Labels["project_id"] = sanitizeUTF8(me.o.projectID)

	return monitoredRes
}

func generateResLabelMap(res *resource.Resource) map[string]string {
	resLabelMap := make(map[string]string)
	for _, label := range res.Attributes() {
		resLabelMap[string(label.Key)] = sanitizeUTF8(label.Value.Emit())
	}
	return resLabelMap
}

// returns transformed label map and false if all labels in match are found
// in input except optional project_id. It returns true if at least one label
// other than project_id is missing.
func transformResource(match, input map[string]string) (map[string]string, bool) {
	output := make(map[string]string, len(input))
	for dst, src := range match {
		if v, ok := input[src]; ok {
			output[dst] = v
		} else {
			return map[string]string{}, true
		}
	}
	return output, false
}

// recordToMdpbKindType return the mapping from OTel's record descriptor to
// Cloud Monitoring's MetricKind and ValueType.
func recordToMdpbKindType(r *export.Record) (googlemetricpb.MetricDescriptor_MetricKind, googlemetricpb.MetricDescriptor_ValueType) {
	var kind googlemetricpb.MetricDescriptor_MetricKind
	switch r.Aggregation().Kind() {
	case aggregation.LastValueKind:
		kind = googlemetricpb.MetricDescriptor_GAUGE
	case aggregation.SumKind:
		kind = googlemetricpb.MetricDescriptor_CUMULATIVE
	case aggregation.HistogramKind:
		return googlemetricpb.MetricDescriptor_CUMULATIVE, googlemetricpb.MetricDescriptor_DISTRIBUTION
	default:
		kind = googlemetricpb.MetricDescriptor_METRIC_KIND_UNSPECIFIED
	}
	var typ googlemetricpb.MetricDescriptor_ValueType
	switch r.Descriptor().NumberKind() {
	case number.Int64Kind:
		typ = googlemetricpb.MetricDescriptor_INT64
	case number.Float64Kind:
		typ = googlemetricpb.MetricDescriptor_DOUBLE
	default:
		typ = googlemetricpb.MetricDescriptor_VALUE_TYPE_UNSPECIFIED
	}

	// TODO: Add handling for MeasureKind if necessary.

	return kind, typ
}

// recordToMpb converts data from records to Metric proto type for Cloud Monitoring.
func (me *metricExporter) recordToMpb(r *export.Record, library instrumentation.Library) *googlemetricpb.Metric {
	desc := r.Descriptor()
	k := keyOf(desc, library)
	md, ok := me.mdCache[k]
	if !ok {
		md = me.recordToMdpb(r)
	}

	labels := make(map[string]string)
	iter := r.Attributes().Iter()
	for iter.Next() {
		kv := iter.Attribute()
		labels[normalizeLabelKey(string(kv.Key))] = sanitizeUTF8(kv.Value.Emit())
	}

	return &googlemetricpb.Metric{
		Type:   md.Type,
		Labels: labels,
	}
}

// recordToTspb converts record to TimeSeries proto type with common resource.
// ref. https://cloud.google.com/monitoring/api/ref_v3/rest/v3/TimeSeries
//
// TODO: Apply appropriate TimeInterval based upon MetricKind. See details in the doc.
// https://cloud.google.com/monitoring/api/ref_v3/rest/v3/TimeSeries#Point
// See detils in #25.
func (me *metricExporter) recordToTspb(r *export.Record, res *resource.Resource, library instrumentation.Library) (*monitoringpb.TimeSeries, error) {
	m := me.recordToMpb(r, library)
	mr := me.resourceToMonitoredResourcepb(res)

	agg := r.Aggregation()
	nkind := r.Descriptor().NumberKind()
	ikind := r.Descriptor().InstrumentKind()

	// TODO: Ignoring the case for Min, Max and Distribution to simply
	// the first implementation.
	//
	// NOTE: Currently the selector used in the integrator is our own implementation,
	// because none of those in go.opentelemetry.io/otel/sdk/metric/selector/simple
	// gives interfaces to fetch LastValue.
	//
	// Views API should provide better interface that does not require the complicated codition handling
	// done in this function.
	// https://github.com/open-telemetry/opentelemetry-specification/issues/466
	// In OpenCensus, view interface provided the bundle of name, measure, labels and aggregation in one place,
	// and it should return the appropriate value based on the aggregation type specified there.
	switch agg.Kind() {
	case aggregation.LastValueKind:
		lv, ok := agg.(aggregation.LastValue)
		if !ok {
			return nil, errUnsupportedAggregation{agg: agg}
		}
		return lastValueToTypedValueAndTimestamp(lv, nkind, m, mr)
	case aggregation.SumKind:
		// CUMULATIVE measurement should have the same start time and increasing end time.
		// c.f. https://cloud.google.com/monitoring/api/ref_v3/rest/v3/projects.metricDescriptors#MetricKind
		sum, ok := agg.(aggregation.Sum)
		if !ok {
			return nil, errUnsupportedAggregation{agg: agg}
		}
		return sumToTypedValueAndTimestamp(sum, nkind, me.startTime, time.Now(), m, mr)
	case aggregation.HistogramKind:
		hist, ok := agg.(aggregation.Histogram)
		if !ok {
			return nil, errUnsupportedAggregation{agg: agg}
		}
		return histogramToTypedValueAndTimestamp(hist, r, m, mr)
	}
	return nil, errUnexpectedAggregationKind{kind: ikind}
}

func sanitizeUTF8(s string) string {
	return strings.ToValidUTF8(s, "�")
}

func lastValueToTypedValueAndTimestamp(lv aggregation.LastValue, kind number.Kind, m *googlemetricpb.Metric, mr *monitoredrespb.MonitoredResource) (*monitoringpb.TimeSeries, error) {
	value, timestamp, err := lv.LastValue()
	if err != nil {
		return nil, err
	}

	timestampPb := timestamppb.New(timestamp)
	err = timestampPb.CheckValid()
	if err != nil {
		return nil, err
	}

	// TODO: Consider the expression of TimeInterval (#25)
	t := &monitoringpb.TimeInterval{
		StartTime: timestampPb,
		EndTime:   timestampPb,
	}

	tv, err := aggToTypedValue(kind, value)
	if err != nil {
		return nil, err
	}

	p := &monitoringpb.Point{
		Value:    tv,
		Interval: t,
	}

	return &monitoringpb.TimeSeries{
		Metric:   m,
		Resource: mr,
		Points:   []*monitoringpb.Point{p},
	}, nil
}

func sumToTypedValueAndTimestamp(sum aggregation.Sum, kind number.Kind, start, end time.Time, m *googlemetricpb.Metric, mr *monitoredrespb.MonitoredResource) (*monitoringpb.TimeSeries, error) {
	value, err := sum.Sum()
	if err != nil {
		return nil, err
	}

	t, err := toNonemptyTimeIntervalpb(start, end)
	if err != nil {
		return nil, err
	}

	tv, err := aggToTypedValue(kind, value)
	if err != nil {
		return nil, err
	}

	p := &monitoringpb.Point{
		Value:    tv,
		Interval: t,
	}

	return &monitoringpb.TimeSeries{
		Metric:   m,
		Resource: mr,
		Points:   []*monitoringpb.Point{p},
	}, nil
}

func histogramToTypedValueAndTimestamp(hist aggregation.Histogram, r *export.Record, m *googlemetricpb.Metric, mr *monitoredrespb.MonitoredResource) (*monitoringpb.TimeSeries, error) {
	tv, err := histToTypedValue(hist)
	if err != nil {
		return nil, err
	}

	t, err := toNonemptyTimeIntervalpb(r.StartTime(), r.EndTime())
	if err != nil {
		return nil, err
	}
	p := &monitoringpb.Point{
		Value:    tv,
		Interval: t,
	}
	return &monitoringpb.TimeSeries{
		Resource:   mr,
		Unit:       string(r.Descriptor().Unit()),
		MetricKind: googlemetricpb.MetricDescriptor_CUMULATIVE,
		ValueType:  googlemetricpb.MetricDescriptor_DISTRIBUTION,
		Points:     []*monitoringpb.Point{p},
		Metric:     m,
	}, nil
}

func toNonemptyTimeIntervalpb(start, end time.Time) (*monitoringpb.TimeInterval, error) {
	// The end time of a new interval must be at least a millisecond after the end time of the
	// previous interval, for all non-gauge types.
	// https://cloud.google.com/monitoring/api/ref_v3/rpc/google.monitoring.v3#timeinterval
	if end.Sub(start).Milliseconds() <= 1 {
		end = start.Add(time.Millisecond)
	}

	startpb := timestamppb.New(start)
	if err := startpb.CheckValid(); err != nil {
		return nil, err
	}

	endpb := timestamppb.New(end)
	if err := endpb.CheckValid(); err != nil {
		return nil, err
	}

	return &monitoringpb.TimeInterval{
		StartTime: startpb,
		EndTime:   endpb,
	}, nil
}

func histToTypedValue(hist aggregation.Histogram) (*monitoringpb.TypedValue, error) {
	buckets, err := hist.Histogram()
	if err != nil {
		return nil, err
	}
	counts := make([]int64, len(buckets.Counts))
	for i, v := range buckets.Counts {
		counts[i] = int64(v)
	}
	sum, err := hist.Sum()
	if err != nil {
		return nil, err
	}
	count, err := hist.Count()
	if err != nil {
		return nil, err
	}
	var mean float64
	if !math.IsNaN(sum.AsFloat64()) && count > 0 { // Avoid divide-by-zero
		mean = sum.AsFloat64() / float64(count)
	}
	return &monitoringpb.TypedValue{
		Value: &monitoringpb.TypedValue_DistributionValue{
			DistributionValue: &distribution.Distribution{
				Count:        int64(count),
				Mean:         mean,
				BucketCounts: counts,
				BucketOptions: &distribution.Distribution_BucketOptions{
					Options: &distribution.Distribution_BucketOptions_ExplicitBuckets{
						ExplicitBuckets: &distribution.Distribution_BucketOptions_Explicit{
							Bounds: buckets.Boundaries,
						},
					},
				},
				// TODO: support exemplars
			},
		},
	}, nil
}

func aggToTypedValue(kind number.Kind, value number.Number) (*monitoringpb.TypedValue, error) {
	switch kind {
	case number.Int64Kind:
		return &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_Int64Value{
				Int64Value: value.AsInt64(),
			},
		}, nil
	case number.Float64Kind:
		return &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_DoubleValue{
				DoubleValue: value.AsFloat64(),
			},
		}, nil
	}
	return nil, errUnexpectedNumberKind{kind: kind}
}

// https://github.com/googleapis/googleapis/blob/c4c562f89acce603fb189679836712d08c7f8584/google/api/metric.proto#L149
//
// > The label key name must follow:
// >
// > * Only upper and lower-case letters, digits and underscores (_) are
// >   allowed.
// > * Label name must start with a letter or digit.
// > * The maximum length of a label name is 100 characters.
func normalizeLabelKey(s string) string {
	return strings.ReplaceAll(s, ".", "_")
}
