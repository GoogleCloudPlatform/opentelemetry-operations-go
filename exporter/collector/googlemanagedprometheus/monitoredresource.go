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
	"go.opentelemetry.io/collector/model/pdata"
	semconv "go.opentelemetry.io/collector/model/semconv/v1.8.0"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

func MapToPrometheusTarget(res pdata.Resource) *monitoredrespb.MonitoredResource {
	attrs := res.Attributes()
	// Prepend namespace if it exists to match what is specified in
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/datamodel.md#resource-attributes-1
	job := getStringOrEmpty(attrs, semconv.AttributeServiceName)
	serviceNamespace := getStringOrEmpty(attrs, semconv.AttributeServiceNamespace)
	if serviceNamespace != "" {
		job = serviceNamespace + "/" + job
	}
	location := getStringOrEmpty(attrs, semconv.AttributeCloudAvailabilityZone)
	if location == "" {
		// fall back to region if we don't have a zone.
		location = getStringOrEmpty(attrs, semconv.AttributeCloudRegion)
	}
	return &monitoredrespb.MonitoredResource{
		Type: "prometheus_target",
		Labels: map[string]string{
			"location":  location,
			"cluster":   getStringOrEmpty(attrs, semconv.AttributeK8SClusterName),
			"namespace": getStringOrEmpty(attrs, semconv.AttributeK8SNamespaceName),
			"job":       job,
			"instance":  getStringOrEmpty(attrs, semconv.AttributeServiceInstanceID),
		},
	}
}

func getStringOrEmpty(attributes pdata.Map, key string) string {
	if val, ok := attributes.Get(key); ok {
		return val.StringVal()
	}
	return ""
}
