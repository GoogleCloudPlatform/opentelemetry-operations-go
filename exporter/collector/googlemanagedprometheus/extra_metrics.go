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

import "go.opentelemetry.io/collector/pdata/pmetric"

// AddTargetInfo extracts the target_info metric from each ResourceMetric associated with the input pmetric.Metrics
// and inserts it into each ScopeMetric for that resource, as specified in
// https://github.com/open-telemetry/opentelemetry-specification/blob/v1.16.0/specification/compatibility/prometheus_and_openmetrics.md#resource-attributes-1
func AddTargetInfo(m pmetric.Metrics) pmetric.ResourceMetricsSlice {
	// initialize a new ResourceMetricSlice, which will hold our target_info metrics for each RM/Scope
	resourceMetricSlice := pmetric.NewResourceMetricsSlice()
	rms := m.ResourceMetrics()
	// loop over input (original) resource metrics
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)

		// store the attributes for this resource that will be added as metric labels to target_info
		nonSpecialResourceAttributes := make(map[string]string)
		for attributeKey := range rm.Resource().Attributes().AsRaw() {
			special := false
			for _, keys := range promTargetKeys {
				for _, specialKey := range keys {
					if attributeKey == specialKey {
						special = true
						break
					}
				}
				if special {
					break
				}
			}
			if !special {
				value, _ := rm.Resource().Attributes().Get(attributeKey)
				nonSpecialResourceAttributes[attributeKey] = value.AsString()
			}
		}

		// copy the Resource to a new ResourceMetric in our target_info slice, so we don't lose its attributes
		rm.Resource().CopyTo(resourceMetricSlice.AppendEmpty().Resource())

		// loop over this resource's scopeMetrics and copy the scope info to our target_info slice
		scopeMetricSlice := resourceMetricSlice.At(i).ScopeMetrics()
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)

			// copy the scope info to our new slice, so the new target_info metric will be associated with it
			// (same reason we copied the Resource above)
			sm.Scope().CopyTo(scopeMetricSlice.AppendEmpty().Scope())
			targetInfoMetric := scopeMetricSlice.At(j).Metrics().AppendEmpty()

			// create the target_info metric as a Gauge with value 1
			targetInfoMetric.SetName("target_info")
			targetInfoMetric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)

			// copy Resource attributes to the metric except for attributes which will already be present in the MonitoredResource labels
			for key, value := range nonSpecialResourceAttributes {
				targetInfoMetric.Gauge().DataPoints().At(0).Attributes().PutStr(key, value)
			}
		}
	}
	return resourceMetricSlice
}
