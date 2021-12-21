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
	"log"
	"math"
	"strings"
	"unicode"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
	"google.golang.org/genproto/googleapis/api/distribution"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// metricsExporter is the GCM exporter that uses pdata directly
type metricsExporter struct {
	cfg    *Config
	client *monitoring.MetricClient
	mapper metricMapper
}

// metricMapper is the part that transforms metrics. Separate from metricsExporter since it has
// all pure functions.
type metricMapper struct {
	cfg *Config
}

type labels map[string]string

func (me *metricsExporter) Shutdown(context.Context) error {
	return me.client.Close()
}

func newGoogleCloudMetricsExporter(
	ctx context.Context,
	cfg *Config,
	set component.ExporterCreateSettings,
) (component.MetricsExporter, error) {
	setVersionInUserAgent(cfg, set.BuildInfo.Version)

	// TODO: map cfg options into metric service client configuration with
	// generateClientOptions()
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		return nil, err
	}

	mExp := &metricsExporter{
		cfg:    cfg,
		client: client,
		mapper: metricMapper{cfg},
	}

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
				timeSeries = append(timeSeries, me.mapper.metricToTimeSeries(monitoredResource, extraLabels, metric)...)
			}
		}
	}

	// TODO: self observability
	err := me.createTimeSeries(ctx, timeSeries)
	recordPointCount(ctx, len(timeSeries), m.DataPointCount()-len(timeSeries), err)
	return err
}

func (me *metricsExporter) createTimeSeries(ctx context.Context, ts []*monitoringpb.TimeSeries) error {
	return me.client.CreateServiceTimeSeries(
		ctx,
		&monitoringpb.CreateTimeSeriesRequest{
			TimeSeries: ts,
			// TODO set Name field with project ID from config or ADC
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
	case pdata.MetricDataTypeExponentialHistogram:
		eh := metric.ExponentialHistogram()
		points := eh.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.exponentialHistogramToTimeSeries(resource, extraLabels, metric, eh, points.At(i))
			timeSeries = append(timeSeries, ts)
		}
	// TODO: add cases for other metric data types
	default:
		// TODO: log unsupported metric
	}

	return timeSeries
}

func (m *metricMapper) exemplar(ex pdata.Exemplar) *distribution.Distribution_Exemplar {
	attachments := []*anypb.Any{}
	// TODO: Look into still sending exemplars with no span.
	if !ex.TraceID().IsEmpty() && !ex.SpanID().IsEmpty() {
		sctx, err := anypb.New(&monitoringpb.SpanContext{
			// TODO - make sure project id is correct.
			SpanName: fmt.Sprintf("projects/%s/traces/%s/spans/%s", m.cfg.ProjectID, ex.TraceID().HexString(), ex.SpanID().HexString()),
		})
		if err == nil {
			attachments = append(attachments, sctx)
		} else {
			log.Printf("Failure to write span context: %v", err)
		}
	}
	if ex.FilteredAttributes().Len() > 0 {
		attr, err := anypb.New(&monitoringpb.DroppedLabels{
			Label: attributesToLabels(ex.FilteredAttributes()),
		})
		if err == nil {
			attachments = append(attachments, attr)
		} else {
			log.Printf("Failure to write filtered_attributes: %v", err)
		}
	}
	return &distribution.Distribution_Exemplar{
		Value:       ex.DoubleVal(),
		Timestamp:   timestamppb.New(ex.Timestamp().AsTime()),
		Attachments: attachments,
	}
}

// Maps an exponential distribution into a GCM point.
func (m *metricMapper) exponentialPoint(point pdata.ExponentialHistogramDataPoint) *monitoringpb.TypedValue {

	growth := math.Exp2(math.Exp2(-float64(point.Scale())))
	scale := math.Pow(growth, float64(point.Positive().Offset()))
	// First calculate underflow bucket with all negatives + zeros.
	underflow := point.ZeroCount()
	for _, v := range point.Negative().BucketCounts() {
		underflow += v
	}

	// Next, pull in remaining buckets.
	counts := []int64{int64(underflow)}
	for _, v := range point.Positive().BucketCounts() {
		counts = append(counts, int64(v))
	}

	// Overflow bucket is always empty
	counts = append(counts, 0)

	// TODO - error handling?
	if len(counts) < 3 {
		// Cannot report an exponential distribution with no buckets.
		return nil
	}
	exemplars := []*distribution.Distribution_Exemplar{}
	exs := point.Exemplars()
	for i := 0; i < exs.Len(); i++ {
		exemplars = append(exemplars, m.exemplar(exs.At(i)))
	}
	return &monitoringpb.TypedValue{
		Value: &monitoringpb.TypedValue_DistributionValue{
			DistributionValue: &distribution.Distribution{
				Count:        int64(point.Count()),
				Mean:         float64(point.Sum() / float64(point.Count())),
				BucketCounts: counts,
				BucketOptions: &distribution.Distribution_BucketOptions{
					Options: &distribution.Distribution_BucketOptions_ExponentialBuckets{
						ExponentialBuckets: &distribution.Distribution_BucketOptions_Exponential{
							GrowthFactor:     growth,
							Scale:            scale,
							NumFiniteBuckets: int32(len(counts) - 2),
						},
					},
				},
				Exemplars: exemplars,
			},
		},
	}
}

func (m *metricMapper) exponentialHistogramToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pdata.Metric,
	sum pdata.ExponentialHistogram,
	point pdata.ExponentialHistogramDataPoint,
) *monitoringpb.TimeSeries {
	// We treat deltas as cumulatives w/ resets.
	metricKind := metricpb.MetricDescriptor_CUMULATIVE
	startTime := timestamppb.New(point.StartTimestamp().AsTime())
	value := m.exponentialPoint(point)
	return &monitoringpb.TimeSeries{
		Resource:   resource,
		Unit:       metric.Unit(),
		MetricKind: metricKind,
		ValueType:  metricpb.MetricDescriptor_DISTRIBUTION,
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

// metricNameToType maps OTLP metric name to GCM metric type (aka name)
func (m *metricMapper) metricNameToType(name string) string {
	// TODO
	return "workload.googleapis.com/" + name
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
