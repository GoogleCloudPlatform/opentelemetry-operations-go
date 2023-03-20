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

	"go.uber.org/zap"

	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
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
func defaultResourceToMonitoredResource(resource pcommon.Resource) *monitoredrespb.MonitoredResource {
	attrs := resource.Attributes()
	gmr := resourcemapping.ResourceAttributesToMonitoredResource(&attributes{
		Attrs: attrs,
	})
	newLabels := make(labels, len(gmr.Labels))
	for k, v := range gmr.Labels {
		newLabels[k] = sanitizeUTF8(v)
	}
	mr := &monitoredrespb.MonitoredResource{
		Type:   gmr.Type,
		Labels: newLabels,
	}
	return mr
}

// resourceToLabels converts the Resource attributes into labels.
// TODO(@damemi): Refactor to pass control-coupling lint check.
//
//nolint:revive
func resourceToLabels(
	resource pcommon.Resource,
	serviceResourceLabels bool,
	resourceFilters []ResourceFilter,
	log *zap.Logger,
) labels {
	attrs := pcommon.NewMap()
	resource.Attributes().Range(func(k string, v pcommon.Value) bool {
		// Is a service attribute and should be included
		if serviceResourceLabels &&
			(k == semconv.AttributeServiceName ||
				k == semconv.AttributeServiceNamespace ||
				k == semconv.AttributeServiceInstanceID) {
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
	return attributesToLabels(attrs)
}
