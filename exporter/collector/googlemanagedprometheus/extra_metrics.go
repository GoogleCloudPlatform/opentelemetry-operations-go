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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// AddTargetInfoMetric inserts target_info for each resource.
// First, it extracts the target_info metric from each ResourceMetric associated with the input pmetric.Metrics
// and inserts it into each ScopeMetric for that resource, as specified in
// https://github.com/open-telemetry/opentelemetry-specification/blob/v1.16.0/specification/compatibility/prometheus_and_openmetrics.md#resource-attributes-1
func AddTargetInfoMetric(m pmetric.Metrics) pmetric.ResourceMetricsSlice {
	// initialize a new ResourceMetricSlice, which will hold our target_info metrics for each RM/Scope
	resourceMetricSlice := pmetric.NewResourceMetricsSlice()
	rms := m.ResourceMetrics()
	// loop over input (original) resource metrics
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)

		// copy the Resource to a new ResourceMetric in our target_info slice, so we don't lose its attributes
		rm.Resource().CopyTo(resourceMetricSlice.AppendEmpty().Resource())

		// create the target_info metric as a Gauge with value 1
		targetInfoMetric := resourceMetricSlice.At(i).ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		targetInfoMetric.SetName("target_info")

		dataPoint := targetInfoMetric.SetEmptyGauge().DataPoints().AppendEmpty()
		dataPoint.SetIntValue(1)

		// copy Resource attributes to the metric except for attributes which will already be present in the MonitoredResource labels
		rm.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
			if !isSpecialAttribute(k) {
				dataPoint.Attributes().PutStr(k, v.AsString())
			}
			return true
		})
	}
	return resourceMetricSlice
}

// AddScopeInfoMetric adds the otel_scope_info metric to a Metrics slice as specified in
// https://github.com/open-telemetry/opentelemetry-specification/blob/v1.16.0/specification/compatibility/prometheus_and_openmetrics.md#instrumentation-scope-1
func AddScopeInfoMetric(m pmetric.Metrics) pmetric.ResourceMetricsSlice {
	resourceMetricSlice := pmetric.NewResourceMetricsSlice()
	rms := m.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		rm.Resource().CopyTo(resourceMetricSlice.AppendEmpty().Resource())

		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)

			scopeInfoMetric := resourceMetricSlice.At(i).ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			scopeInfoMetric.SetName("otel_scope_info")

			dataPoint := scopeInfoMetric.SetEmptyGauge().DataPoints().AppendEmpty()
			dataPoint.SetIntValue(1)

			dataPoint.Attributes().PutStr("otel_scope_name", sm.Scope().Name())
			dataPoint.Attributes().PutStr("otel_scope_version", sm.Scope().Version())
			sm.Scope().Attributes().Range(func(k string, v pcommon.Value) bool {
				dataPoint.Attributes().PutStr(k, v.AsString())
				return true
			})
		}
	}
	return resourceMetricSlice
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
