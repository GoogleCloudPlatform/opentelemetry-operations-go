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
	"go.opentelemetry.io/collector/pdata/pcommon"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

func TestResourceMetricsToMonitoredResource(t *testing.T) {
	tests := []struct {
		resourceLabels map[string]string
		expectMr       *monitoredrespb.MonitoredResource
		name           string
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
		},
		{
			name: "GCE with OTel service attribs",
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_compute_engine",
				"cloud.availability_zone": "us-central1",
				"host.id":                 "abc123",
				"service.namespace":       "myservicenamespace",
				"service.name":            "myservicename",
				"service.instance.id":     "myserviceinstanceid",
			},
			expectMr: &monitoredrespb.MonitoredResource{
				Type:   "gce_instance",
				Labels: map[string]string{"instance_id": "abc123", "zone": "us-central1"},
			},
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
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := pcommon.NewResource()
			for k, v := range test.resourceLabels {
				r.Attributes().PutStr(k, v)
			}
			mr := defaultResourceToMonitoredResource(r)
			assert.Equal(t, test.expectMr, mr)
		})
	}
}

func TestResourceToMetricLabels(t *testing.T) {
	tests := []struct {
		resourceLabels    map[string]string
		expectExtraLabels labels
		updateMapper      func(mapper *metricMapper)
		name              string
	}{
		{
			name: "No Extra labels",
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_compute_engine",
				"cloud.availability_zone": "us-central1",
				"host.id":                 "abc123",
			},
			expectExtraLabels: labels{},
		},
		{
			name: "GCE with OTel service attribs",
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_compute_engine",
				"cloud.availability_zone": "us-central1",
				"host.id":                 "abc123",
				"service.namespace":       "myservicenamespace",
				"service.name":            "myservicename",
				"service.instance.id":     "myserviceinstanceid",
			},
			expectExtraLabels: labels{
				"service_namespace":   "myservicenamespace",
				"service_name":        "myservicename",
				"service_instance_id": "myserviceinstanceid",
			},
		},
		{
			name: "GCE disable OTel service attribs",
			updateMapper: func(mapper *metricMapper) {
				mapper.cfg.MetricConfig.ServiceResourceLabels = false
			},
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_compute_engine",
				"cloud.availability_zone": "us-central1",
				"host.id":                 "abc123",
				"service.namespace":       "myservicenamespace",
				"service.name":            "myservicename",
				"service.instance.id":     "myserviceinstanceid",
			},
			expectExtraLabels: labels{},
		},
		{
			name: "GCE with resource filter config",
			updateMapper: func(mapper *metricMapper) {
				mapper.cfg.MetricConfig.ResourceFilters = []ResourceFilter{{Prefix: "cloud."}}
			},
			resourceLabels: map[string]string{
				"cloud.platform":          "gcp_compute_engine",
				"cloud.availability_zone": "us-central1",
				"host.id":                 "abc123",
			},
			expectExtraLabels: labels{
				"cloud_platform":          "gcp_compute_engine",
				"cloud_availability_zone": "us-central1",
			},
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
			expectExtraLabels: labels{
				"service_namespace":   "myservicenamespace",
				"service_name":        "myservicename",
				"service_instance_id": "myserviceinstanceid",
			},
		},
		{
			name: "Generic task no location",
			resourceLabels: map[string]string{
				"service.namespace":   "myservicenamespace",
				"service.name":        "myservicename",
				"service.instance.id": "myserviceinstanceid",
			},
			expectExtraLabels: labels{
				"service_namespace":   "myservicenamespace",
				"service_name":        "myservicename",
				"service_instance_id": "myserviceinstanceid",
			},
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
			expectExtraLabels: labels{
				"service_namespace":   "myservicenamespace",
				"service_name":        "myservicename",
				"service_instance_id": "myserviceinstanceid",
			},
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
			expectExtraLabels: labels{
				"service_namespace":   "myservicenamespace",
				"service_name":        "myservicename",
				"service_instance_id": "myserviceinstanceid",
			},
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
			expectExtraLabels: labels{
				"service_namespace": "myservicenamespace",
				"service_name":      "myservicename",
			},
		},
		{
			name: "Generic node without cloud.availability_zone",
			resourceLabels: map[string]string{
				"cloud.region":      "my-region",
				"service.namespace": "myservicenamespace",
				"host.id":           "myhostid",
				"host.name":         "myhostname",
			},
			expectExtraLabels: labels{
				"service_namespace": "myservicenamespace",
			},
		},
		{
			name: "Generic node without host.id",
			resourceLabels: map[string]string{
				"cloud.region":      "my-region",
				"service.namespace": "myservicenamespace",
				"host.name":         "myhostname",
			},
			expectExtraLabels: labels{
				"service_namespace": "myservicenamespace",
			},
		},
		{
			name: "Resource filter regex config + prefix",
			updateMapper: func(mapper *metricMapper) {
				mapper.cfg.MetricConfig.ResourceFilters = []ResourceFilter{
					{Prefix: "cloud.", Regex: ".*availability.*"},
					{Regex: "host.*"},
				}
			},
			resourceLabels: map[string]string{
				"cloud.platform":            "gcp_compute_engine",
				"cloud.availability_zone":   "us-central1",
				"cloud.availability_region": "us-central",
				"host.id":                   "abc123",
				"random.fake":               "woot",
			},
			expectExtraLabels: labels{
				"cloud_availability_zone":   "us-central1",
				"cloud_availability_region": "us-central",
				"host_id":                   "abc123",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mapper := metricMapper{cfg: DefaultConfig()}
			if test.updateMapper != nil {
				test.updateMapper(&mapper)
			}
			r := pcommon.NewResource()
			for k, v := range test.resourceLabels {
				r.Attributes().PutStr(k, v)
			}
			extraLabels := resourceToLabels(r, mapper.cfg.MetricConfig.ServiceResourceLabels, mapper.cfg.MetricConfig.ResourceFilters, nil)
			assert.Equal(t, test.expectExtraLabels, extraLabels)
		})
	}
}

func TestResourceMetricsToMonitoredResourceUTF8(t *testing.T) {
	invalidUtf8TwoOctet := string([]byte{0xc3, 0x28})   // Invalid 2-octet sequence
	invalidUtf8SequenceID := string([]byte{0xa0, 0xa1}) // Invalid sequence identifier

	resourceLabels := map[string]string{
		"cloud.platform":          "gcp_kubernetes_engine",
		"cloud.availability_zone": "abcdefg", // valid ascii
		"k8s.cluster.name":        "שלום",    // valid utf8
		"k8s.namespace.name":      invalidUtf8TwoOctet,
		"k8s.pod.name":            invalidUtf8SequenceID,
		"valid_ascii":             "abcdefg",
		"valid_utf8":              "שלום",
		"invalid_two_octet":       invalidUtf8TwoOctet,
		"invalid_sequence_id":     invalidUtf8SequenceID,
	}
	expectMr := &monitoredrespb.MonitoredResource{
		Type: "k8s_pod",
		Labels: map[string]string{
			"cluster_name":   "שלום",
			"location":       "abcdefg",
			"namespace_name": "�(",
			"pod_name":       "�",
		},
	}
	expectExtraLabels := labels{
		"valid_ascii":         "abcdefg",
		"valid_utf8":          "שלום",
		"invalid_two_octet":   "�(",
		"invalid_sequence_id": "�",
	}

	mapper := metricMapper{cfg: DefaultConfig()}
	mapper.cfg.MetricConfig.ResourceFilters = []ResourceFilter{
		{Prefix: "valid_"},
		{Prefix: "invalid_"},
	}
	r := pcommon.NewResource()
	for k, v := range resourceLabels {
		r.Attributes().PutStr(k, v)
	}
	mr := defaultResourceToMonitoredResource(r)
	assert.Equal(t, expectMr, mr)
	extraLabels := resourceToLabels(r, mapper.cfg.MetricConfig.ServiceResourceLabels, mapper.cfg.MetricConfig.ResourceFilters, nil)
	assert.Equal(t, expectExtraLabels, extraLabels)
}
