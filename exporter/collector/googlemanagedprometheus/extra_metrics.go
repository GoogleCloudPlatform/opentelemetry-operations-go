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

	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

var intToDoubleFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"exporter.googlemanagedprometheus.intToDouble",
	featuregate.StageAlpha,
	featuregate.WithRegisterFromVersion("v0.100.0"),
	featuregate.WithRegisterDescription("Convert all int metrics to double metrics to avoid incompatible value types."),
	featuregate.WithRegisterReferenceURL("https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/issues/798"))

const prometheusMetricMetadataTypeKey = "prometheus.type"

func (c Config) ExtraMetrics(m pmetric.Metrics) {
	addUntypedMetrics(m)
	c.addTargetInfoMetric(m)
	c.addScopeInfoMetric(m)
	convertIntToDouble(m)
}

// convertIntToDouble converts all counter and gauge int values to double.
func convertIntToDouble(m pmetric.Metrics) {
	if !intToDoubleFeatureGate.IsEnabled() {
		return
	}
	rms := m.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)

				var points pmetric.NumberDataPointSlice
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					points = metric.Sum().DataPoints()
				case pmetric.MetricTypeGauge:
					points = metric.Gauge().DataPoints()
				default:
					continue
				}
				for x := 0; x < points.Len(); x++ {
					point := points.At(x)
					if point.ValueType() == pmetric.NumberDataPointValueTypeInt {
						point.SetDoubleValue(float64(point.IntValue()))
					}
				}
			}
		}
	}
}

// addUntypedMetrics looks for any Gauge data point with the special Ops Agent untyped metric
// attribute and duplicates that data point to a matching Sum.
func addUntypedMetrics(m pmetric.Metrics) {
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

				if !isUnknown(metric) {
					continue
				}

				// attribute is set on the data point
				gauge := metric.Gauge()
				points := gauge.DataPoints()
				for l := 0; l < points.Len(); l++ {
					// Add the new Sum metric and copy values over from this Gauge
					point := points.At(l)
					newMetric := mes.AppendEmpty()
					newMetric.SetName(metric.Name())
					newMetric.SetDescription(metric.Description())
					newMetric.SetUnit(metric.Unit())
					newMetric.Metadata().PutStr(prometheusMetricMetadataTypeKey, string(model.MetricTypeUnknown))

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

func isUnknown(metric pmetric.Metric) bool {
	originalType, ok := metric.Metadata().Get(prometheusMetricMetadataTypeKey)
	return ok && originalType.Str() == string(model.MetricTypeUnknown)
}

// resourceID identifies a resource. We only send one target info for each
// resourceID.
type resourceID struct {
	serviceName, serviceInstanceID, serviceNamespace string
}

// addTargetInfoMetric inserts target_info for each resource.
// First, it extracts the target_info metric from each ResourceMetric associated with the input pmetric.Metrics
// and inserts it into a new ScopeMetric for that resource, as specified in
// https://github.com/open-telemetry/opentelemetry-specification/blob/v1.16.0/specification/compatibility/prometheus_and_openmetrics.md#resource-attributes-1
func (c Config) addTargetInfoMetric(m pmetric.Metrics) {
	if !c.ExtraMetricsConfig.EnableTargetInfo {
		return
	}
	ids := make(map[resourceID]struct{})
	rms := m.ResourceMetrics()
	// loop over input (original) resource metrics
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		getResourceAttr := func(attr string) string {
			if v, ok := rm.Resource().Attributes().Get(attr); ok {
				return v.AsString()
			}
			return ""
		}

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

		id := resourceID{
			serviceName:       getResourceAttr(string(semconv.ServiceNameKey)),
			serviceNamespace:  getResourceAttr(string(semconv.ServiceNamespaceKey)),
			serviceInstanceID: getResourceAttr(string(semconv.ServiceInstanceIDKey)),
		}
		if _, ok := ids[id]; ok {
			// We've already added a resource with the same ID before, so skip this one.
			continue
		}
		ids[id] = struct{}{}

		// create the target_info metric as a Gauge with value 1
		targetInfoMetric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		targetInfoMetric.SetName("target_info")

		dataPoint := targetInfoMetric.SetEmptyGauge().DataPoints().AppendEmpty()
		dataPoint.SetIntValue(1)
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(latestTime))

		// copy Resource attributes to the metric except for service.name, service.namespace, and service.instance.id
		// because those three attributes (if present) will be copied to resource attributes.
		// See https://opentelemetry.io/docs/specs/otel/compatibility/prometheus_and_openmetrics/#resource-attributes-1
		// Also drop reserved GMP labels (location, cluster, namespace, job, service_namespace, instance).
		// Other "fallback" attributes which could become `job` or `instance` in the absence of those three
		// (such as k8s.pod.name or faas.name) will be duplicated as a resource attribute and metric label.
		rm.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
			if k != string(semconv.ServiceNameKey) &&
				k != string(semconv.ServiceNamespaceKey) &&
				k != string(semconv.ServiceInstanceIDKey) &&
				k != locationLabel &&
				k != clusterLabel &&
				k != namespaceLabel &&
				k != jobLabel &&
				k != serviceNamespaceLabel &&
				k != instanceLabel {
				dataPoint.Attributes().PutStr(k, v.AsString())
			}
			return true
		})
	}
}

// scopeID identifies a scope. We only send one scope info for each scopeID within a unique resource.
type scopeID struct {
	resource      resourceID
	name, version string
}

// addScopeInfoMetric adds the otel_scope_info metric to a Metrics slice as specified in
// https://github.com/open-telemetry/opentelemetry-specification/blob/v1.16.0/specification/compatibility/prometheus_and_openmetrics.md#instrumentation-scope-1
// It also updates all other metrics with the corresponding scope_name and scope_version attributes, if they are present.
func (c Config) addScopeInfoMetric(m pmetric.Metrics) {
	if !c.ExtraMetricsConfig.EnableScopeInfo {
		return
	}
	ids := make(map[scopeID]struct{})
	rms := m.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		getResourceAttr := func(attr string) string {
			if v, ok := rm.Resource().Attributes().Get(attr); ok {
				return v.AsString()
			}
			return ""
		}
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			// If not present, skip this scope
			if len(sm.Scope().Name()) == 0 && len(sm.Scope().Version()) == 0 {
				continue
			}

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
			id := scopeID{
				resource: resourceID{
					serviceName:       getResourceAttr(string(semconv.ServiceNameKey)),
					serviceNamespace:  getResourceAttr(string(semconv.ServiceNamespaceKey)),
					serviceInstanceID: getResourceAttr(string(semconv.ServiceInstanceIDKey)),
				},
				name:    sm.Scope().Name(),
				version: sm.Scope().Version(),
			}
			if _, ok := ids[id]; ok {
				// We've already added a scope with the same ID before, so skip this one.
				continue
			}
			ids[id] = struct{}{}

			// Add otel_scope_info metric
			scopeInfoMetric := sm.Metrics().AppendEmpty()
			scopeInfoMetric.SetName("otel_scope_info")
			dataPoint := scopeInfoMetric.SetEmptyGauge().DataPoints().AppendEmpty()
			dataPoint.SetIntValue(1)
			sm.Scope().Attributes().Range(func(k string, v pcommon.Value) bool {
				dataPoint.Attributes().PutStr(k, v.AsString())
				return true
			})
			dataPoint.Attributes().PutStr("otel_scope_name", sm.Scope().Name())
			dataPoint.Attributes().PutStr("otel_scope_version", sm.Scope().Version())
			dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(latestTime))
		}
	}
}
