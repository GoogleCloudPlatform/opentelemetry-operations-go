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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

func TestResourceMetricsToMonitoredResource(t *testing.T) {
	tests := []struct {
		name              string
		resourceLabels    map[string]string
		expectMr          *monitoredrespb.MonitoredResource
		expectExtraLabels labels
	}{
		{
			name: "GCE instance",
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_compute_engine",
				"cloud.availability_zone": "us-central1",
				"host.id":                 "abc123",
			},
			expectMr: &monitoredrespb.MonitoredResource{
				Type:   "gce_instance",
				Labels: map[string]string{"instance_id": "abc123", "zone": "us-central1"},
			},
			expectExtraLabels: labels{},
		},
		{
			name: "K8s container",
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_kubernetes_engine",
				"cloud.availability_zone": "us-central1",
				"k8s.cluster.name":        "mycluster",
				"k8s.namespace.name":      "mynamespace",
				"k8s.pod.name":            "mypod",
				"k8s.container.name":      "mycontainer",
			},
			expectMr: &monitoredrespb.MonitoredResource{
				Type: "k8s_container",
				Labels: map[string]string{
					"cluster_name":   "mycluster",
					"container_name": "mycontainer",
					"location":       "us-central1",
					"namespace_name": "mynamespace",
					"pod_name":       "mypod",
				},
			},
			expectExtraLabels: labels{},
		},
		{
			name: "K8s pod",
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_kubernetes_engine",
				"cloud.availability_zone": "us-central1",
				"k8s.cluster.name":        "mycluster",
				"k8s.namespace.name":      "mynamespace",
				"k8s.pod.name":            "mypod",
			},
			expectMr: &monitoredrespb.MonitoredResource{
				Type: "k8s_pod",
				Labels: map[string]string{
					"cluster_name":   "mycluster",
					"location":       "us-central1",
					"namespace_name": "mynamespace",
					"pod_name":       "mypod",
				},
			},
			expectExtraLabels: labels{},
		},
		{
			name: "K8s node",
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_kubernetes_engine",
				"cloud.availability_zone": "us-central1",
				"k8s.cluster.name":        "mycluster",
				"k8s.node.name":           "mynode",
			},
			expectMr: &monitoredrespb.MonitoredResource{
				Type: "k8s_node",
				Labels: map[string]string{
					"cluster_name": "mycluster",
					"location":     "us-central1",
					"node_name":    "mynode",
				},
			},
			expectExtraLabels: labels{},
		},
		{
			name: "K8s cluster",
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_kubernetes_engine",
				"cloud.availability_zone": "us-central1",
				"k8s.cluster.name":        "mycluster",
			},
			expectMr: &monitoredrespb.MonitoredResource{
				Type: "k8s_cluster",
				Labels: map[string]string{
					"cluster_name": "mycluster",
					"location":     "us-central1",
				},
			},
			expectExtraLabels: labels{},
		},
		{
			name: "AWS ec2 instance",
			resourceLabels: map[string]string{
				"cloud.platform":          "aws_ec2",
				"cloud.availability_zone": "us-central1",
				"host.id":                 "abc123",
				"cloud.account.id":        "myawsaccount",
			},
			expectMr: &monitoredrespb.MonitoredResource{
				Type: "aws_ec2_instance",
				Labels: map[string]string{
					"aws_account": "myawsaccount",
					"instance_id": "abc123",
					"region":      "us-central1",
				},
			},
			expectExtraLabels: labels{},
		},
		{
			name: "Generic task no cloud.platform",
			resourceLabels: map[string]string{
				"cloud.availability_zone": "us-central1",
				"cloud.region":            "my-region",
				"service.namespace":       "myservicenamespace",
				"service.name":            "myservicename",
				"service.instance.id":     "myserviceinstanceid",
			},
			expectMr: &monitoredrespb.MonitoredResource{
				Type: "generic_task",
				Labels: map[string]string{
					"job":       "myservicename",
					"location":  "us-central1",
					"namespace": "myservicenamespace",
					"task_id":   "myserviceinstanceid",
				},
			},
			expectExtraLabels: labels{},
		},
		{
			name: "Generic task no location",
			resourceLabels: map[string]string{
				"service.namespace":   "myservicenamespace",
				"service.name":        "myservicename",
				"service.instance.id": "myserviceinstanceid",
			},
			expectMr: &monitoredrespb.MonitoredResource{
				Type: "generic_task",
				Labels: map[string]string{
					"job":       "myservicename",
					"location":  "global",
					"namespace": "myservicenamespace",
					"task_id":   "myserviceinstanceid",
				},
			},
			expectExtraLabels: labels{},
		},
		{
			name: "Generic task unrecognized cloud.platform",
			resourceLabels: map[string]string{
				"cloud.platform":          "fooprovider_kubernetes_service",
				"cloud.availability_zone": "us-central1",
				"service.namespace":       "myservicenamespace",
				"service.name":            "myservicename",
				"service.instance.id":     "myserviceinstanceid",
			},
			expectMr: &monitoredrespb.MonitoredResource{
				Type: "generic_task",
				Labels: map[string]string{
					"job":       "myservicename",
					"location":  "us-central1",
					"namespace": "myservicenamespace",
					"task_id":   "myserviceinstanceid",
				},
			},
			expectExtraLabels: labels{},
		},
		{
			name: "Generic task without cloud.availability_zone region",
			resourceLabels: map[string]string{
				"cloud.platform":      "fooprovider_kubernetes_service",
				"cloud.region":        "my-region",
				"service.namespace":   "myservicenamespace",
				"service.name":        "myservicename",
				"service.instance.id": "myserviceinstanceid",
			},
			expectMr: &monitoredrespb.MonitoredResource{
				Type: "generic_task",
				Labels: map[string]string{
					"job":       "myservicename",
					"location":  "my-region",
					"namespace": "myservicenamespace",
					"task_id":   "myserviceinstanceid",
				},
			},
			expectExtraLabels: labels{},
		},
		{
			name: "Generic node",
			resourceLabels: map[string]string{
				"cloud.platform":          "fooprovider_kubernetes_service",
				"cloud.availability_zone": "us-central1",
				"cloud.region":            "my-region",
				"service.namespace":       "myservicenamespace",
				"service.name":            "myservicename",
				"host.id":                 "myhostid",
				"host.name":               "myhostname",
			},
			expectMr: &monitoredrespb.MonitoredResource{
				Type: "generic_node",
				Labels: map[string]string{
					"location":  "us-central1",
					"namespace": "myservicenamespace",
					"node_id":   "myhostid",
				},
			},
			expectExtraLabels: labels{},
		},
		{
			name: "Generic node without cloud.availability_zone",
			resourceLabels: map[string]string{
				"cloud.region":      "my-region",
				"service.namespace": "myservicenamespace",
				"host.id":           "myhostid",
				"host.name":         "myhostname",
			},
			expectMr: &monitoredrespb.MonitoredResource{
				Type: "generic_node",
				Labels: map[string]string{
					"location":  "my-region",
					"namespace": "myservicenamespace",
					"node_id":   "myhostid",
				},
			},
			expectExtraLabels: labels{},
		},
		{
			name: "Generic node without host.id",
			resourceLabels: map[string]string{
				"cloud.region":      "my-region",
				"service.namespace": "myservicenamespace",
				"host.name":         "myhostname",
			},
			expectMr: &monitoredrespb.MonitoredResource{
				Type: "generic_node",
				Labels: map[string]string{
					"location":  "my-region",
					"namespace": "myservicenamespace",
					"node_id":   "myhostname",
				},
			},
			expectExtraLabels: labels{},
		},
		{
			name:           "Generic node with no labels",
			resourceLabels: map[string]string{},
			expectMr: &monitoredrespb.MonitoredResource{
				Type: "generic_node",
				Labels: map[string]string{
					"location":  "global",
					"namespace": "",
					"node_id":   "",
				},
			},
			expectExtraLabels: labels{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mapper := metricMapper{cfg: Config{}}
			r := pdata.NewResource()
			for k, v := range test.resourceLabels {
				r.Attributes().InsertString(k, v)
			}
			mr, extraLabels := mapper.resourceMetricsToMonitoredResource(r)
			assert.Equal(t, test.expectMr, mr)
			assert.Equal(t, test.expectExtraLabels, extraLabels)
		})
	}
}
