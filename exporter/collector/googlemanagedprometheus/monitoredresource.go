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

package googlemanagedprometheus

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/collector/semconv/v1.8.0"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

const (
	// In GMP, location, cluster, and namespace labels can be overridden by
	// users to set corresponding fields in the monitored resource. To
	// replicate that behavior in the collector, we expect these labels
	// to be moved from metric labels to resource labels using the groupbyattrs
	// processor. If these resource labels are present, use them to set the MR.
	locationLabel  = "location"
	clusterLabel   = "cluster"
	namespaceLabel = "namespace"
)

// promTargetKeys are attribute keys which are used in the prometheus_target monitored resource.
// It is also used by GMP to exclude these keys from the target_info metric.
var promTargetKeys = map[string][]string{
	locationLabel:       {locationLabel, semconv.AttributeCloudAvailabilityZone, semconv.AttributeCloudRegion},
	clusterLabel:        {clusterLabel, semconv.AttributeK8SClusterName},
	namespaceLabel:      {namespaceLabel, semconv.AttributeK8SNamespaceName},
	"job":               {semconv.AttributeServiceName},
	"service_namespace": {semconv.AttributeServiceNamespace},
	"instance":          {semconv.AttributeServiceInstanceID},
}

func MapToPrometheusTarget(res pcommon.Resource) *monitoredrespb.MonitoredResource {
	attrs := res.Attributes()
	// Prepend namespace if it exists to match what is specified in
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/datamodel.md#resource-attributes-1
	job := getStringOrEmpty(attrs, promTargetKeys["job"]...)
	serviceNamespace := getStringOrEmpty(attrs, promTargetKeys["service_namespace"]...)
	if serviceNamespace != "" {
		job = serviceNamespace + "/" + job
	}
	return &monitoredrespb.MonitoredResource{
		Type: "prometheus_target",
		Labels: map[string]string{
			locationLabel:  getStringOrEmpty(attrs, promTargetKeys[locationLabel]...),
			clusterLabel:   getStringOrEmpty(attrs, promTargetKeys[clusterLabel]...),
			namespaceLabel: getStringOrEmpty(attrs, promTargetKeys[namespaceLabel]...),
			"job":          job,
			"instance":     getStringOrEmpty(attrs, promTargetKeys["instance"]...),
		},
	}
}

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

// getStringOrEmpty returns the value of the first key found, or the empty string.
func getStringOrEmpty(attributes pcommon.Map, keys ...string) string {
	for _, k := range keys {
		if val, ok := attributes.Get(k); ok {
			return val.Str()
		}
	}
	return ""
}
