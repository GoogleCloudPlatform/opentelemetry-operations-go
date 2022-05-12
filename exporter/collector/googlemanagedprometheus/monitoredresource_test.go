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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

func TestMapToPrometheusTarget(t *testing.T) {
	for _, tc := range []struct {
		resourceLabels map[string]string
		expected       *monitoredrespb.MonitoredResource
		desc           string
	}{
		{
			desc:           "no attributes",
			resourceLabels: map[string]string{},
			expected: &monitoredrespb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"location":  "",
					"cluster":   "",
					"namespace": "",
					"job":       "",
					"instance":  "",
				},
			},
		},
		{
			desc: "with gke container attributes",
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_kubernetes_engine",
				"cloud.availability_zone": "us-central1-c",
				"k8s.cluster.name":        "mycluster",
				"k8s.namespace.name":      "mynamespace",
				"k8s.pod.name":            "mypod",
				"k8s.container.name":      "mycontainer",
				"service.name":            "myservicename",
				"service.instance.id":     "myserviceinstanceid",
			},
			expected: &monitoredrespb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"location":  "us-central1-c",
					"cluster":   "mycluster",
					"namespace": "mynamespace",
					"job":       "myservicename",
					"instance":  "myserviceinstanceid",
				},
			},
		},
		{
			desc: "Attributes from prometheus receiver, and zone",
			resourceLabels: map[string]string{
				"cloud.availability_zone": "us-central1-c",
				"service.name":            "myservicename",
				"service.instance.id":     "myserviceinstanceid",
			},
			expected: &monitoredrespb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"location":  "us-central1-c",
					"cluster":   "",
					"namespace": "",
					"job":       "myservicename",
					"instance":  "myserviceinstanceid",
				},
			},
		},
		{
			desc: "Attributes from prometheus receiver, and region",
			resourceLabels: map[string]string{
				"cloud.region":        "us-central1",
				"service.name":        "myservicename",
				"service.instance.id": "myserviceinstanceid",
			},
			expected: &monitoredrespb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"location":  "us-central1",
					"cluster":   "",
					"namespace": "",
					"job":       "myservicename",
					"instance":  "myserviceinstanceid",
				},
			},
		},
		{
			desc: "Attributes service from opentelemetry sdk and zone",
			resourceLabels: map[string]string{
				"cloud.availability_zone": "us-central1-c",
				"service.name":            "myservicename",
				"service.namespace":       "myservicenamespace",
				"service.instance.id":     "myserviceinstanceid",
			},
			expected: &monitoredrespb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"location":  "us-central1-c",
					"cluster":   "",
					"namespace": "",
					"job":       "myservicenamespace/myservicename",
					"instance":  "myserviceinstanceid",
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			r := pcommon.NewResource()
			for k, v := range tc.resourceLabels {
				r.Attributes().InsertString(k, v)
			}
			got := MapToPrometheusTarget(r)
			assert.Equal(t, tc.expected, got)
		})
	}
}
