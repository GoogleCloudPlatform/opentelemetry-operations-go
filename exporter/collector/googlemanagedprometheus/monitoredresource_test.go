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
			desc: "k8s.pod.name as additional fallback to instance",
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_kubernetes_engine",
				"cloud.availability_zone": "us-central1-c",
				"k8s.cluster.name":        "mycluster",
				"k8s.namespace.name":      "mynamespace",
				"k8s.pod.name":            "mypod",
				"k8s.container.name":      "mycontainer",
				"service.name":            "myservicename",
			},
			expected: &monitoredrespb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"location":  "us-central1-c",
					"cluster":   "mycluster",
					"namespace": "mynamespace",
					"job":       "myservicename",
					"instance":  "mypod/mycontainer",
				},
			},
		},
		{
			desc: "k8s.deployment.name as additional fallback to job",
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_kubernetes_engine",
				"cloud.availability_zone": "us-central1-c",
				"k8s.cluster.name":        "mycluster",
				"k8s.namespace.name":      "mynamespace",
				"k8s.deployment.name":     "mydeployment",
				"k8s.pod.name":            "mypod",
				"k8s.container.name":      "mycontainer",
			},
			expected: &monitoredrespb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"location":  "us-central1-c",
					"cluster":   "mycluster",
					"namespace": "mynamespace",
					"job":       "mydeployment",
					"instance":  "mypod/mycontainer",
				},
			},
		},
		{
			desc: "k8s.statefuleset.name as additional fallback to job",
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_kubernetes_engine",
				"cloud.availability_zone": "us-central1-c",
				"k8s.cluster.name":        "mycluster",
				"k8s.namespace.name":      "mynamespace",
				"k8s.statefulset.name":    "mystatefulset",
				"k8s.pod.name":            "mypod",
				"k8s.container.name":      "mycontainer",
			},
			expected: &monitoredrespb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"location":  "us-central1-c",
					"cluster":   "mycluster",
					"namespace": "mynamespace",
					"job":       "mystatefulset",
					"instance":  "mypod/mycontainer",
				},
			},
		},
		{
			desc: "k8s.daemonset.name as additional fallback to job",
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_kubernetes_engine",
				"cloud.availability_zone": "us-central1-c",
				"k8s.cluster.name":        "mycluster",
				"k8s.namespace.name":      "mynamespace",
				"k8s.daemonset.name":      "mydaemonset",
				"k8s.pod.name":            "mypod",
				"k8s.container.name":      "mycontainer",
			},
			expected: &monitoredrespb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"location":  "us-central1-c",
					"cluster":   "mycluster",
					"namespace": "mynamespace",
					"job":       "mydaemonset",
					"instance":  "mypod/mycontainer",
				},
			},
		},
		{
			desc: "k8s.job.name as additional fallback to job",
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_kubernetes_engine",
				"cloud.availability_zone": "us-central1-c",
				"k8s.cluster.name":        "mycluster",
				"k8s.namespace.name":      "mynamespace",
				"k8s.job.name":            "myjob",
				"k8s.pod.name":            "mypod",
				"k8s.container.name":      "mycontainer",
			},
			expected: &monitoredrespb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"location":  "us-central1-c",
					"cluster":   "mycluster",
					"namespace": "mynamespace",
					"job":       "myjob",
					"instance":  "mypod/mycontainer",
				},
			},
		},
		{
			desc: "k8s.cronjob.name as additional fallback to job",
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_kubernetes_engine",
				"cloud.availability_zone": "us-central1-c",
				"k8s.cluster.name":        "mycluster",
				"k8s.namespace.name":      "mynamespace",
				"k8s.cronjob.name":        "mycronjob",
				"k8s.pod.name":            "mypod",
				"k8s.container.name":      "mycontainer",
			},
			expected: &monitoredrespb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"location":  "us-central1-c",
					"cluster":   "mycluster",
					"namespace": "mynamespace",
					"job":       "mycronjob",
					"instance":  "mypod/mycontainer",
				},
			},
		},
		{
			desc: "k8s.cluster.uid as additional fallback to cluster",
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_kubernetes_engine",
				"cloud.availability_zone": "us-central1-c",
				"k8s.cluster.uid":         "123901490g90fd89080943",
				"k8s.namespace.name":      "mynamespace",
				"k8s.cronjob.name":        "mycronjob",
				"k8s.pod.name":            "mypod",
				"k8s.container.name":      "mycontainer",
			},
			expected: &monitoredrespb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"location":  "us-central1-c",
					"cluster":   "123901490g90fd89080943",
					"namespace": "mynamespace",
					"job":       "mycronjob",
					"instance":  "mypod/mycontainer",
				},
			},
		},
		{
			desc: "overridden attributes",
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_kubernetes_engine",
				"cloud.availability_zone": "us-central1-c",
				"k8s.cluster.name":        "mycluster",
				"k8s.namespace.name":      "mynamespace",
				"k8s.pod.name":            "mypod",
				"k8s.container.name":      "mycontainer",
				"service.name":            "myservicename",
				"service.instance.id":     "myserviceinstanceid",
				"location":                "overridden-location",
				"cluster":                 "overridden-cluster",
				"namespace":               "overridden-namespace",
			},
			expected: &monitoredrespb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"location":  "overridden-location",
					"cluster":   "overridden-cluster",
					"namespace": "overridden-namespace",
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
		{
			desc: "Attributes from cloud run",
			resourceLabels: map[string]string{
				"cloud.region":  "us-central1",
				"service.name":  "unknown_service:go",
				"faas.name":     "my-cloud-run-service",
				"faas.instance": "1234759430923053489543203",
			},
			expected: &monitoredrespb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"location":  "us-central1",
					"cluster":   "",
					"namespace": "",
					"job":       "my-cloud-run-service",
					"instance":  "1234759430923053489543203",
				},
			},
		},
		{
			desc: "Attributes from cloud run with environment label",
			resourceLabels: map[string]string{
				"cloud.platform": "gcp_cloud_run",
				"cloud.region":   "us-central1",
				"service.name":   "unknown_service:go",
				"faas.name":      "my-cloud-run-service",
				"faas.instance":  "1234759430923053489543203",
			},
			expected: &monitoredrespb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"location":  "us-central1",
					"cluster":   "__run__",
					"namespace": "",
					"job":       "my-cloud-run-service",
					"instance":  "1234759430923053489543203",
				},
			},
		},
		{
			desc: "Attributes from GCE with environment label",
			resourceLabels: map[string]string{
				"cloud.platform": "gcp_compute_engine",
				"cloud.region":   "us-central1",
				"service.name":   "service-name",
				"host.id":        "1234759430923053489543203",
			},
			expected: &monitoredrespb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"location":  "us-central1",
					"cluster":   "__gce__",
					"namespace": "",
					"job":       "service-name",
					"instance":  "1234759430923053489543203",
				},
			},
		},
		{
			desc: "Attributes from GCE with instance ID override",
			resourceLabels: map[string]string{
				"cloud.platform":      "gcp_compute_engine",
				"cloud.region":        "us-central1",
				"service.name":        "service-name",
				"service.instance.id": "service-instance-id",
				"host.id":             "1234759430923053489543203",
			},
			expected: &monitoredrespb.MonitoredResource{
				Type: "prometheus_target",
				Labels: map[string]string{
					"location":  "us-central1",
					"cluster":   "__gce__",
					"namespace": "",
					"job":       "service-name",
					"instance":  "service-instance-id",
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			r := pcommon.NewResource()
			for k, v := range tc.resourceLabels {
				r.Attributes().PutStr(k, v)
			}
			got := DefaultConfig().MapToPrometheusTarget(r)
			assert.Equal(t, tc.expected, got)
		})
	}
}
