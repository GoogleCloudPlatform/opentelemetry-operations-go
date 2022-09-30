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
	"reflect"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"go.uber.org/multierr"
	"google.golang.org/api/option"
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

func keyOf(metrics metricdata.Metrics, library instrumentation.Library) key {
	return key{
		name:        metrics.Name,
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
}

// ForceFlush does nothing, the exporter holds no state.
func (e *metricExporter) ForceFlush(ctx context.Context) error { return ctx.Err() }

// Shutdown shuts down the client connections
func (e *metricExporter) Shutdown(ctx context.Context) error {
	return multierr.Combine(ctx.Err(), e.client.Close())
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

	clientOpts := append([]option.ClientOption{option.WithUserAgent(userAgent)}, o.monitoringClientOptions...)
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
		o:       o,
		mdCache: cache,
		client:  client,
	}
	return e, nil
}

// Export exports OpenTelemetry Metrics to Google Cloud Monitoring.
func (me *metricExporter) Export(ctx context.Context, rm metricdata.ResourceMetrics) error {
	if err := me.exportMetricDescriptor(ctx, rm); err != nil {
		return err
	}

	if err := me.exportTimeSeries(ctx, rm); err != nil {
		return err
	}
	return nil
}

// exportMetricDescriptor create MetricDescriptor from the record
// if the descriptor is not registered in Cloud Monitoring yet.
func (me *metricExporter) exportMetricDescriptor(ctx context.Context, rm metricdata.ResourceMetrics) error {
	mds := make(map[key]*googlemetricpb.MetricDescriptor)
	for _, scope := range rm.ScopeMetrics {
		for _, metrics := range scope.Metrics {
			k := keyOf(metrics, scope.Scope)

			if _, ok := me.mdCache[k]; ok {
				continue
			}

			if _, localok := mds[k]; !localok {
				md := me.recordToMdpb(metrics)
				mds[k] = md
			}
		}
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
func (me *metricExporter) exportTimeSeries(ctx context.Context, rm metricdata.ResourceMetrics) error {
	tss := []*monitoringpb.TimeSeries{}
	mr := me.resourceToMonitoredResourcepb(rm.Resource)
	var errs []error

	for _, scope := range rm.ScopeMetrics {
		for _, metrics := range scope.Metrics {
			ts, err := me.recordToTspb(metrics, mr, scope.Scope)
			if err != nil {
				errs = append(errs, err)
			} else {
				tss = append(tss, ts...)
			}
		}
	}

	var aggError error
	if len(errs) > 0 {
		aggError = multierr.Combine(errs...)
	}

	if aggError != nil {
		if me.o.onError != nil {
			me.o.onError(aggError)
		} else {
			log.Printf("Error during exporting TimeSeries: %v", aggError)
		}
	}

	if len(tss) == 0 {
		return nil
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
func (me *metricExporter) descToMetricType(desc metricdata.Metrics) string {
	if formatter := me.o.metricDescriptorTypeFormatter; formatter != nil {
		return formatter(desc)
	}
	return fmt.Sprintf(cloudMonitoringMetricDescriptorNameFormat, desc.Name)
}

// recordToMdpb extracts data and converts them to googlemetricpb.MetricDescriptor.
func (me *metricExporter) recordToMdpb(metrics metricdata.Metrics) *googlemetricpb.MetricDescriptor {
	name := metrics.Name
	kind, valueType := recordToMdpbKindType(metrics.Data)

	// Detailed explanations on MetricDescriptor proto is not documented on
	// generated Go packages. Refer to the original proto file.
	// https://github.com/googleapis/googleapis/blob/50af053/google/api/metric.proto#L33
	return &googlemetricpb.MetricDescriptor{
		Name:        name,
		Type:        me.descToMetricType(metrics),
		MetricKind:  kind,
		ValueType:   valueType,
		Unit:        string(metrics.Unit),
		Description: metrics.Description,
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
func recordToMdpbKindType(a metricdata.Aggregation) (googlemetricpb.MetricDescriptor_MetricKind, googlemetricpb.MetricDescriptor_ValueType) {
	switch a.(type) {
	case metricdata.Gauge[int64]:
		return googlemetricpb.MetricDescriptor_GAUGE, googlemetricpb.MetricDescriptor_INT64
	case metricdata.Gauge[float64]:
		return googlemetricpb.MetricDescriptor_GAUGE, googlemetricpb.MetricDescriptor_DOUBLE
	case metricdata.Sum[int64]:
		return googlemetricpb.MetricDescriptor_CUMULATIVE, googlemetricpb.MetricDescriptor_INT64
	case metricdata.Sum[float64]:
		return googlemetricpb.MetricDescriptor_CUMULATIVE, googlemetricpb.MetricDescriptor_DOUBLE
	case metricdata.Histogram:
		return googlemetricpb.MetricDescriptor_CUMULATIVE, googlemetricpb.MetricDescriptor_DISTRIBUTION
	default:
		return googlemetricpb.MetricDescriptor_METRIC_KIND_UNSPECIFIED, googlemetricpb.MetricDescriptor_VALUE_TYPE_UNSPECIFIED
	}
}

// recordToMpb converts data from records to Metric proto type for Cloud Monitoring.
func (me *metricExporter) recordToMpb(metrics metricdata.Metrics, attributes attribute.Set, library instrumentation.Library) *googlemetricpb.Metric {
	k := keyOf(metrics, library)
	md, ok := me.mdCache[k]
	if !ok {
		md = me.recordToMdpb(metrics)
	}

	labels := make(map[string]string)
	iter := attributes.Iter()
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
func (me *metricExporter) recordToTspb(m metricdata.Metrics, mr *monitoredrespb.MonitoredResource, library instrumentation.Scope) ([]*monitoringpb.TimeSeries, error) {
	var tss []*monitoringpb.TimeSeries
	errs := []error{}
	switch a := m.Data.(type) {
	case metricdata.Gauge[int64]:
		for _, point := range a.DataPoints {
			ts, err := gaugeToTimeSeries[int64](point, m, mr)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			ts.Metric = me.recordToMpb(m, point.Attributes, library)
			tss = append(tss, ts)
		}
	case metricdata.Gauge[float64]:
		for _, point := range a.DataPoints {
			ts, err := gaugeToTimeSeries[float64](point, m, mr)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			ts.Metric = me.recordToMpb(m, point.Attributes, library)
			tss = append(tss, ts)
		}
	case metricdata.Sum[int64]:
		for _, point := range a.DataPoints {
			var ts *monitoringpb.TimeSeries
			var err error
			if a.IsMonotonic {
				ts, err = sumToTimeSeries[int64](point, m, mr)
			} else {
				// Send non-monotonic sums as gauges
				ts, err = gaugeToTimeSeries[int64](point, m, mr)
			}
			if err != nil {
				errs = append(errs, err)
				continue
			}
			ts.Metric = me.recordToMpb(m, point.Attributes, library)
			tss = append(tss, ts)
		}
	case metricdata.Sum[float64]:
		for _, point := range a.DataPoints {
			var ts *monitoringpb.TimeSeries
			var err error
			if a.IsMonotonic {
				ts, err = sumToTimeSeries[float64](point, m, mr)
			} else {
				// Send non-monotonic sums as gauges
				ts, err = gaugeToTimeSeries[float64](point, m, mr)
			}
			if err != nil {
				errs = append(errs, err)
				continue
			}
			ts.Metric = me.recordToMpb(m, point.Attributes, library)
			tss = append(tss, ts)
		}
	case metricdata.Histogram:
		for _, point := range a.DataPoints {
			ts, err := histogramToTimeSeries(point, m, mr)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			ts.Metric = me.recordToMpb(m, point.Attributes, library)
			tss = append(tss, ts)
		}
	default:
		errs = append(errs, errUnexpectedAggregationKind{kind: reflect.TypeOf(m.Data).String()})
	}
	return tss, multierr.Combine(errs...)
}

func sanitizeUTF8(s string) string {
	return strings.ToValidUTF8(s, "�")
}

func gaugeToTimeSeries[N int64 | float64](point metricdata.DataPoint[N], metrics metricdata.Metrics, mr *monitoredrespb.MonitoredResource) (*monitoringpb.TimeSeries, error) {
	value, valueType := numberDataPointToValue(point)
	timestamp := timestamppb.New(point.Time)
	if err := timestamp.CheckValid(); err != nil {
		return nil, err
	}
	return &monitoringpb.TimeSeries{
		Resource:   mr,
		Unit:       string(metrics.Unit),
		MetricKind: googlemetricpb.MetricDescriptor_GAUGE,
		ValueType:  valueType,
		Points: []*monitoringpb.Point{{
			Interval: &monitoringpb.TimeInterval{
				StartTime: timestamp,
				EndTime:   timestamp,
			},
			Value: value,
		}},
	}, nil
}

func sumToTimeSeries[N int64 | float64](point metricdata.DataPoint[N], metrics metricdata.Metrics, mr *monitoredrespb.MonitoredResource) (*monitoringpb.TimeSeries, error) {
	interval, err := toNonemptyTimeIntervalpb(point.StartTime, point.Time)
	if err != nil {
		return nil, err
	}
	value, valueType := numberDataPointToValue[N](point)
	return &monitoringpb.TimeSeries{
		Resource:   mr,
		Unit:       string(metrics.Unit),
		MetricKind: googlemetricpb.MetricDescriptor_CUMULATIVE,
		ValueType:  valueType,
		Points: []*monitoringpb.Point{{
			Interval: interval,
			Value:    value,
		}},
	}, nil
}

func histogramToTimeSeries(point metricdata.HistogramDataPoint, metrics metricdata.Metrics, mr *monitoredrespb.MonitoredResource) (*monitoringpb.TimeSeries, error) {
	interval, err := toNonemptyTimeIntervalpb(point.StartTime, point.Time)
	if err != nil {
		return nil, err
	}
	return &monitoringpb.TimeSeries{
		Resource:   mr,
		Unit:       string(metrics.Unit),
		MetricKind: googlemetricpb.MetricDescriptor_CUMULATIVE,
		ValueType:  googlemetricpb.MetricDescriptor_DISTRIBUTION,
		Points: []*monitoringpb.Point{{
			Interval: interval,
			Value:    histToTypedValue(point),
		}},
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

func histToTypedValue(hist metricdata.HistogramDataPoint) *monitoringpb.TypedValue {
	counts := make([]int64, len(hist.BucketCounts))
	for i, v := range hist.BucketCounts {
		counts[i] = int64(v)
	}
	var mean float64
	if !math.IsNaN(hist.Sum) && hist.Count > 0 { // Avoid divide-by-zero
		mean = hist.Sum / float64(hist.Count)
	}
	return &monitoringpb.TypedValue{
		Value: &monitoringpb.TypedValue_DistributionValue{
			DistributionValue: &distribution.Distribution{
				Count:        int64(hist.Count),
				Mean:         mean,
				BucketCounts: counts,
				BucketOptions: &distribution.Distribution_BucketOptions{
					Options: &distribution.Distribution_BucketOptions_ExplicitBuckets{
						ExplicitBuckets: &distribution.Distribution_BucketOptions_Explicit{
							Bounds: hist.Bounds,
						},
					},
				},
				// TODO: support exemplars
			},
		},
	}
}

func numberDataPointToValue[N int64 | float64](
	point metricdata.DataPoint[N],
) (*monitoringpb.TypedValue, googlemetricpb.MetricDescriptor_ValueType) {
	switch v := any(point.Value).(type) {
	case int64:
		return &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_Int64Value{
				Int64Value: v,
			}},
			googlemetricpb.MetricDescriptor_INT64
	case float64:
		return &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{
				DoubleValue: v,
			}},
			googlemetricpb.MetricDescriptor_DOUBLE
	}
	// It is impossible to reach this statement
	return nil, googlemetricpb.MetricDescriptor_INT64
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
