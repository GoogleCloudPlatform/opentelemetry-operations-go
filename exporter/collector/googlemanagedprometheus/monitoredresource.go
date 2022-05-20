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
	semconv "go.opentelemetry.io/collector/semconv/v1.8.0"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

func MapToPrometheusTarget(res pcommon.Resource) *monitoredrespb.MonitoredResource {
	attrs := res.Attributes()
	// Prepend namespace if it exists to match what is specified in
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/datamodel.md#resource-attributes-1
	job := getStringOrEmpty(attrs, semconv.AttributeServiceName)
	serviceNamespace := getStringOrEmpty(attrs, semconv.AttributeServiceNamespace)
	if serviceNamespace != "" {
		job = serviceNamespace + "/" + job
	}
	return &monitoredrespb.MonitoredResource{
		Type: "prometheus_target",
		Labels: map[string]string{
			"location":  getStringOrEmpty(attrs, "location", semconv.AttributeCloudAvailabilityZone, semconv.AttributeCloudRegion),
			"cluster":   getStringOrEmpty(attrs, "cluster", semconv.AttributeK8SClusterName),
			"namespace": getStringOrEmpty(attrs, "namespace", semconv.AttributeK8SNamespaceName),
			"job":       job,
			"instance":  getStringOrEmpty(attrs, semconv.AttributeServiceInstanceID),
		},
	}
}

// getStringOrEmpty returns the value of the first key found, or the empty string
func getStringOrEmpty(attributes pcommon.Map, keys ...string) string {
	for _, k := range keys {
		if val, ok := attributes.Get(k); ok {
			return val.StringVal()
		}
	}
	return ""
}
