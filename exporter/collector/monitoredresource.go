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

package collector

import (
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping"
)

type attributes struct {
	Attrs pcommon.Map
}

func (attrs *attributes) GetString(key string) (string, bool) {
	value, ok := attrs.Attrs.Get(key)
	if ok {
		return value.AsString(), ok
	}
	return "", false
}

// defaultResourceToMonitoredResource pdata Resource to a GCM Monitored Resource.
func defaultResourceToMonitoringMonitoredResource(resource pcommon.Resource) *monitoredrespb.MonitoredResource {
	return resourcemapping.ResourceAttributesToMonitoringMonitoredResource(&attributes{
		Attrs: resource.Attributes(),
	})
}

// defaultResourceToMonitoredResource pdata Resource to a GCM Monitored Resource.
func defaultResourceToLoggingMonitoredResource(resource pcommon.Resource) *monitoredrespb.MonitoredResource {
	return resourcemapping.ResourceAttributesToLoggingMonitoredResource(&attributes{
		Attrs: resource.Attributes(),
	})
}

// resourceToLabels converts the Resource attributes into labels.
// TODO(@damemi): Refactor to pass control-coupling lint check.
//
//nolint:revive
func filterAttributes(
	attributes pcommon.Map,
	serviceResourceLabels bool,
	resourceFilters []ResourceFilter,
) pcommon.Map {
	attrs := pcommon.NewMap()
	attributes.Range(func(k string, v pcommon.Value) bool {
		// Is a service attribute and should be included
		if serviceResourceLabels &&
			(k == string(semconv.ServiceNameKey) ||
				k == string(semconv.ServiceNamespaceKey) ||
				k == string(semconv.ServiceInstanceIDKey)) {
			if len(v.AsString()) > 0 {
				v.CopyTo(attrs.PutEmpty(k))
			}
			return true
		}
		// Matches one of the resource filters
		for _, resourceFilter := range resourceFilters {
			if match, _ := regexp.Match(resourceFilter.Regex, []byte(k)); strings.HasPrefix(k, resourceFilter.Prefix) && match {
				v.CopyTo(attrs.PutEmpty(k))
				return true
			}
		}
		return true
	})
	return attrs
}
