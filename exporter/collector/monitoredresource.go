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
	"strings"

	"go.opentelemetry.io/collector/model/pdata"
	semconv "go.opentelemetry.io/collector/model/semconv/v1.8.0"
	"go.opentelemetry.io/collector/pdata/pcommon"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping"
)

type attributes struct {
	Attrs *pdata.Map
}

func (attrs *attributes) GetString(key string) (string, bool) {
	value, ok := attrs.Attrs.Get(key)
	if ok {
		return value.AsString(), ok
	}
	return "", false
}

// Transforms pdata Resource to a GCM Monitored Resource. Any additional metric labels from the
// resource are also returned.
func (m *metricMapper) resourceToMonitoredResource(
	resource pdata.Resource,
) (*monitoredrespb.MonitoredResource, labels) {
	attrs := resource.Attributes()
	gmr := resourcemapping.ResourceAttributesToMonitoredResource(&attributes{
		Attrs: &attrs,
	})
	newLabels := make(labels, len(gmr.Labels))
	for k, v := range gmr.Labels {
		newLabels[k] = sanitizeUTF8(v)
	}
	mr := &monitoredrespb.MonitoredResource{
		Type:   gmr.Type,
		Labels: newLabels,
	}
	return mr, m.resourceToMetricLabels(resource)
}

// resourceToMetricLabels converts the Resource into metric labels needed to uniquely identify
// the timeseries.
func (m *metricMapper) resourceToMetricLabels(
	resource pdata.Resource,
) labels {
	attrs := pcommon.NewMap()
	resource.Attributes().Range(func(k string, v pdata.Value) bool {
		// Is a service attribute and should be included
		if m.cfg.MetricConfig.ServiceResourceLabels &&
			(k == semconv.AttributeServiceName ||
				k == semconv.AttributeServiceNamespace ||
				k == semconv.AttributeServiceInstanceID) {
			attrs.Insert(k, v)
			return true
		}
		// Matches one of the resource filters
		for _, resourceFilter := range m.cfg.MetricConfig.ResourceFilters {
			if strings.HasPrefix(k, resourceFilter.Prefix) {
				attrs.Insert(k, v)
				return true
			}
		}
		return true
	})
	return attributesToLabels(attrs)
}
