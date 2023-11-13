// Copyright 2023 Google LLC
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

package googlemanagedprometheus

import (
	"time"

	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	// Special attribute key used by Ops Agent prometheus receiver to denote untyped
	// prometheus metric. Internal use only.
	GCPOpsAgentUntypedMetricKey = "prometheus.googleapis.com/internal/untyped_metric"

	gcpUntypedDoubleExportGateKey = "gcp.untypedDoubleExport"
)

var untypedDoubleExportFeatureGate = featuregate.GlobalRegistry().MustRegister(
	gcpUntypedDoubleExportGateKey,
	featuregate.StageAlpha,
	featuregate.WithRegisterFromVersion("v0.77.0"),
	featuregate.WithRegisterDescription("Enable automatically exporting untyped Prometheus metrics as both gauge and cumulative to GCP."),
	featuregate.WithRegisterReferenceURL("https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/pull/668"))

func (c Config) ExtraMetrics(m pmetric.Metrics) {
	addUntypedMetrics(m)
	c.addTargetInfoMetric(m)
	c.addScopeInfoMetric(m)
}

// addUntypedMetrics looks for any Gauge data point with the special Ops Agent untyped metric
// attribute and duplicates that data point to a matching Sum.
func addUntypedMetrics(m pmetric.Metrics) {
	if !untypedDoubleExportFeatureGate.IsEnabled() {
		return
	}
	rms := m.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			mes := sm.Metrics()

			// only iterate up to the current length. new untyped sum metrics
			// will be added to this slice inline.
			mLen := mes.Len()
			for k := 0; k < mLen; k++ {
				metric := mes.At(k)

				// only applies to gauges
				if metric.Type() != pmetric.MetricTypeGauge {
					continue
				}

				// attribute is set on the data point
				gauge := metric.Gauge()
				points := gauge.DataPoints()
				for l := 0; l < points.Len(); l++ {
					point := points.At(l)
					val, ok := point.Attributes().Get(GCPOpsAgentUntypedMetricKey)
					if !(ok && val.AsString() == "true") {
						continue
					}

					// Add the new Sum metric and copy values over from this Gauge
					newMetric := mes.AppendEmpty()
					newMetric.SetName(metric.Name())
					newMetric.SetDescription(metric.Description())
					newMetric.SetUnit(metric.Unit())

					newSum := newMetric.SetEmptySum()
					newSum.SetIsMonotonic(true)
					newSum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

					newDataPoint := newSum.DataPoints().AppendEmpty()
					point.Attributes().CopyTo(newDataPoint.Attributes())
					if point.ValueType() == pmetric.NumberDataPointValueTypeInt {
						newDataPoint.SetIntValue(point.IntValue())
					} else if point.ValueType() == pmetric.NumberDataPointValueTypeDouble {
						newDataPoint.SetDoubleValue(point.DoubleValue())
					}
					newDataPoint.SetFlags(point.Flags())
					newDataPoint.SetTimestamp(point.Timestamp())
					newDataPoint.SetStartTimestamp(point.StartTimestamp())
				}
			}
		}
	}
}

// addTargetInfoMetric inserts target_info for each resource.
// First, it extracts the target_info metric from each ResourceMetric associated with the input pmetric.Metrics
// and inserts it into a new ScopeMetric for that resource, as specified in
// https://github.com/open-telemetry/opentelemetry-specification/blob/v1.16.0/specification/compatibility/prometheus_and_openmetrics.md#resource-attributes-1
func (c Config) addTargetInfoMetric(m pmetric.Metrics) {
	if !c.ExtraMetricsConfig.EnableTargetInfo {
		return
	}
	rms := m.ResourceMetrics()
	// loop over input (original) resource metrics
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)

		// Keep track of the most recent time in this resource's metrics
		// Use that time for the timestamp of the new metric
		latestTime := time.Time{}
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			for k := 0; k < rm.ScopeMetrics().At(j).Metrics().Len(); k++ {
				metric := rm.ScopeMetrics().At(j).Metrics().At(k)

				switch metric.Type() {
				case pmetric.MetricTypeSum:
					sum := metric.Sum()
					points := sum.DataPoints()
					for x := 0; x < points.Len(); x++ {
						if latestTime.Before(points.At(x).Timestamp().AsTime()) {
							latestTime = points.At(x).Timestamp().AsTime()
						}
					}
				case pmetric.MetricTypeGauge:
					gauge := metric.Gauge()
					points := gauge.DataPoints()
					for x := 0; x < points.Len(); x++ {
						if latestTime.Before(points.At(x).Timestamp().AsTime()) {
							latestTime = points.At(x).Timestamp().AsTime()
						}
					}
				case pmetric.MetricTypeSummary:
					summary := metric.Summary()
					points := summary.DataPoints()
					for x := 0; x < points.Len(); x++ {
						if latestTime.Before(points.At(x).Timestamp().AsTime()) {
							latestTime = points.At(x).Timestamp().AsTime()
						}
					}
				case pmetric.MetricTypeHistogram:
					hist := metric.Histogram()
					points := hist.DataPoints()
					for x := 0; x < points.Len(); x++ {
						if latestTime.Before(points.At(x).Timestamp().AsTime()) {
							latestTime = points.At(x).Timestamp().AsTime()
						}
					}
				case pmetric.MetricTypeExponentialHistogram:
					eh := metric.ExponentialHistogram()
					points := eh.DataPoints()
					for x := 0; x < points.Len(); x++ {
						if latestTime.Before(points.At(x).Timestamp().AsTime()) {
							latestTime = points.At(x).Timestamp().AsTime()
						}
					}
				}
			}
		}

		// create the target_info metric as a Gauge with value 1
		targetInfoMetric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		targetInfoMetric.SetName("target_info")

		dataPoint := targetInfoMetric.SetEmptyGauge().DataPoints().AppendEmpty()
		dataPoint.SetIntValue(1)
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(latestTime))

		// copy Resource attributes to the metric except for attributes which will already be present in the MonitoredResource labels
		rm.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
			if !isSpecialAttribute(k) {
				dataPoint.Attributes().PutStr(k, v.AsString())
			}
			return true
		})
	}
}

// addScopeInfoMetric adds the otel_scope_info metric to a Metrics slice as specified in
// https://github.com/open-telemetry/opentelemetry-specification/blob/v1.16.0/specification/compatibility/prometheus_and_openmetrics.md#instrumentation-scope-1
// It also updates all other metrics with the corresponding scope_name and scope_version attributes, if they are present.
func (c Config) addScopeInfoMetric(m pmetric.Metrics) {
	if !c.ExtraMetricsConfig.EnableScopeInfo {
		return
	}
	rms := m.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			// If not present, skip this scope
			if len(sm.Scope().Name()) == 0 && len(sm.Scope().Version()) == 0 {
				continue
			}

			// Add otel_scope_info metric
			scopeInfoMetric := sm.Metrics().AppendEmpty()
			scopeInfoMetric.SetName("otel_scope_info")
			dataPoint := scopeInfoMetric.SetEmptyGauge().DataPoints().AppendEmpty()
			dataPoint.SetIntValue(1)
			sm.Scope().Attributes().Range(func(k string, v pcommon.Value) bool {
				dataPoint.Attributes().PutStr(k, v.AsString())
				return true
			})

			// Keep track of the most recent time in this scope's metrics
			// Use that time for the timestamp of the new metric
			latestTime := time.Time{}
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					sum := metric.Sum()
					points := sum.DataPoints()
					for x := 0; x < points.Len(); x++ {
						point := points.At(x)
						point.Attributes().PutStr("otel_scope_name", sm.Scope().Name())
						point.Attributes().PutStr("otel_scope_version", sm.Scope().Version())
						if latestTime.Before(points.At(x).Timestamp().AsTime()) {
							latestTime = points.At(x).Timestamp().AsTime()
						}
					}
				case pmetric.MetricTypeGauge:
					gauge := metric.Gauge()
					points := gauge.DataPoints()
					for x := 0; x < points.Len(); x++ {
						point := points.At(x)
						point.Attributes().PutStr("otel_scope_name", sm.Scope().Name())
						point.Attributes().PutStr("otel_scope_version", sm.Scope().Version())
						if latestTime.Before(points.At(x).Timestamp().AsTime()) {
							latestTime = points.At(x).Timestamp().AsTime()
						}
					}
				case pmetric.MetricTypeSummary:
					summary := metric.Summary()
					points := summary.DataPoints()
					for x := 0; x < points.Len(); x++ {
						point := points.At(x)
						point.Attributes().PutStr("otel_scope_name", sm.Scope().Name())
						point.Attributes().PutStr("otel_scope_version", sm.Scope().Version())
						if latestTime.Before(points.At(x).Timestamp().AsTime()) {
							latestTime = points.At(x).Timestamp().AsTime()
						}
					}
				case pmetric.MetricTypeHistogram:
					hist := metric.Histogram()
					points := hist.DataPoints()
					for x := 0; x < points.Len(); x++ {
						point := points.At(x)
						point.Attributes().PutStr("otel_scope_name", sm.Scope().Name())
						point.Attributes().PutStr("otel_scope_version", sm.Scope().Version())
						if latestTime.Before(points.At(x).Timestamp().AsTime()) {
							latestTime = points.At(x).Timestamp().AsTime()
						}
					}
				case pmetric.MetricTypeExponentialHistogram:
					eh := metric.ExponentialHistogram()
					points := eh.DataPoints()
					for x := 0; x < points.Len(); x++ {
						point := points.At(x)
						point.Attributes().PutStr("otel_scope_name", sm.Scope().Name())
						point.Attributes().PutStr("otel_scope_version", sm.Scope().Version())
						if latestTime.Before(points.At(x).Timestamp().AsTime()) {
							latestTime = points.At(x).Timestamp().AsTime()
						}
					}
				}
			}

			dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(latestTime))
		}
	}
}

func isSpecialAttribute(attributeKey string) bool {
	for _, keys := range promTargetKeys {
		for _, specialKey := range keys {
			if attributeKey == specialKey {
				return true
			}
		}
	}
	return false
}
