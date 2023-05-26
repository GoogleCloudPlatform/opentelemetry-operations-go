// Copyright 2022 Google LLC
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

package testcases

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

// ConvertResourceMetrics converts a (collector) pdata metrics to an SDK ResourceMetrics
// This is useful for testing the SDK with the same data used to test the collector.
func ConvertResourceMetrics(pdataMetrics pmetric.Metrics) []*metricdata.ResourceMetrics {
	resourceMetrics := make([]*metricdata.ResourceMetrics, pdataMetrics.ResourceMetrics().Len())
	for i := 0; i < pdataMetrics.ResourceMetrics().Len(); i++ {
		rm := pdataMetrics.ResourceMetrics().At(i)
		resourceMetrics[i] = &metricdata.ResourceMetrics{
			Resource:     resource.NewSchemaless(convertAttributes(rm.Resource().Attributes())...),
			ScopeMetrics: convertScopeMetrics(rm.ScopeMetrics()),
		}
	}
	return resourceMetrics
}

func convertScopeMetrics(sms pmetric.ScopeMetricsSlice) []metricdata.ScopeMetrics {
	scopeMetrics := make([]metricdata.ScopeMetrics, sms.Len())
	for i := 0; i < sms.Len(); i++ {
		sm := sms.At(i)
		scopeMetrics[i] = metricdata.ScopeMetrics{
			Scope: instrumentation.Scope{
				Name:    sm.Scope().Name(),
				Version: sm.Scope().Version(),
				// TODO scope attributes?
			},
			Metrics: convertMetrics(sm.Metrics()),
		}
	}
	return scopeMetrics
}

func convertMetrics(ms pmetric.MetricSlice) []metricdata.Metrics {
	metrics := make([]metricdata.Metrics, ms.Len())
	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)
		metric := metricdata.Metrics{
			Name:        m.Name(),
			Description: m.Description(),
			Unit:        m.Unit(),
		}

		switch m.Type() {
		case pmetric.MetricTypeGauge:
			metric.Data = convertGauge(m.Gauge())
		case pmetric.MetricTypeSum:
			metric.Data = convertSum(m.Sum())
		case pmetric.MetricTypeHistogram:
			metric.Data = convertHistogram(m.Histogram())
		case pmetric.MetricTypeSummary:
			// Skip summary metrics
			continue
		case pmetric.MetricTypeExponentialHistogram:
			// Skip exponential histogram metrics
			continue
		}
		metrics[i] = metric
	}
	return metrics
}

func convertGauge(g pmetric.Gauge) metricdata.Aggregation {
	if g.DataPoints().Len() == 0 {
		return nil
	}
	switch g.DataPoints().At(0).ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		return metricdata.Gauge[float64]{
			DataPoints: convertFloatDataPoints(g.DataPoints()),
		}
	case pmetric.NumberDataPointValueTypeInt:
		return metricdata.Gauge[int64]{
			DataPoints: convertIntDataPoints(g.DataPoints()),
		}
	}
	return nil
}

func convertSum(s pmetric.Sum) metricdata.Aggregation {
	if s.DataPoints().Len() == 0 {
		return nil
	}
	switch s.DataPoints().At(0).ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		return metricdata.Sum[float64]{
			Temporality: convertTemporality(s.AggregationTemporality()),
			IsMonotonic: s.IsMonotonic(),
			DataPoints:  convertFloatDataPoints(s.DataPoints()),
		}
	case pmetric.NumberDataPointValueTypeInt:
		return metricdata.Sum[int64]{
			Temporality: convertTemporality(s.AggregationTemporality()),
			IsMonotonic: s.IsMonotonic(),
			DataPoints:  convertIntDataPoints(s.DataPoints()),
		}
	}
	return nil
}

func convertFloatDataPoints(pts pmetric.NumberDataPointSlice) []metricdata.DataPoint[float64] {
	dataPoints := make([]metricdata.DataPoint[float64], pts.Len())
	for i := 0; i < pts.Len(); i++ {
		pt := pts.At(i)
		dataPoints[i] = metricdata.DataPoint[float64]{
			Attributes: attribute.NewSet(convertAttributes(pt.Attributes())...),
			StartTime:  pt.StartTimestamp().AsTime(),
			Time:       pt.Timestamp().AsTime(),
			Value:      pt.DoubleValue(),
		}
	}
	return dataPoints
}

func convertIntDataPoints(pts pmetric.NumberDataPointSlice) []metricdata.DataPoint[int64] {
	dataPoints := make([]metricdata.DataPoint[int64], pts.Len())
	for i := 0; i < pts.Len(); i++ {
		pt := pts.At(i)
		dataPoints[i] = metricdata.DataPoint[int64]{
			Attributes: attribute.NewSet(convertAttributes(pt.Attributes())...),
			StartTime:  pt.StartTimestamp().AsTime(),
			Time:       pt.Timestamp().AsTime(),
			Value:      pt.IntValue(),
		}
	}
	return dataPoints
}

func convertHistogram(h pmetric.Histogram) metricdata.Aggregation {
	if h.DataPoints().Len() == 0 {
		return nil
	}
	agg := metricdata.Histogram[float64]{
		Temporality: convertTemporality(h.AggregationTemporality()),
		DataPoints:  make([]metricdata.HistogramDataPoint[float64], h.DataPoints().Len()),
	}
	for i := 0; i < h.DataPoints().Len(); i++ {
		pt := h.DataPoints().At(i)
		agg.DataPoints[i] = metricdata.HistogramDataPoint[float64]{
			Attributes:   attribute.NewSet(convertAttributes(pt.Attributes())...),
			StartTime:    pt.StartTimestamp().AsTime(),
			Time:         pt.Timestamp().AsTime(),
			Count:        pt.Count(),
			Sum:          pt.Sum(),
			Bounds:       pt.ExplicitBounds().AsRaw(),
			BucketCounts: pt.BucketCounts().AsRaw(),
		}
	}
	return agg
}

func convertAttributes(attrs pcommon.Map) []attribute.KeyValue {
	var kvs []attribute.KeyValue
	attrs.Range(func(k string, v pcommon.Value) bool {
		switch v.Type() {
		case pcommon.ValueTypeStr:
			kvs = append(kvs, attribute.String(k, v.Str()))
		case pcommon.ValueTypeBool:
			kvs = append(kvs, attribute.Bool(k, v.Bool()))
		case pcommon.ValueTypeInt:
			kvs = append(kvs, attribute.Int64(k, v.Int()))
		case pcommon.ValueTypeDouble:
			kvs = append(kvs, attribute.Float64(k, v.Double()))
		default:
			kvs = append(kvs, attribute.String(k, v.AsString()))
		}
		return true
	})
	return kvs
}

func convertTemporality(t pmetric.AggregationTemporality) metricdata.Temporality {
	var temp metricdata.Temporality
	switch t {
	case pmetric.AggregationTemporalityDelta:
		return metricdata.DeltaTemporality
	case pmetric.AggregationTemporalityCumulative:
		return metricdata.CumulativeTemporality
	}
	return temp
}
