// Copyright 2020, OpenCensus Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stackdriver // import "contrib.go.opencensus.io/exporter/stackdriver"

import (
	"fmt"
	"sync"
	"testing"

	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource"
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource/gcp"
	"github.com/google/go-cmp/cmp"
	"go.opencensus.io/resource"
	"go.opencensus.io/resource/resourcekeys"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestDefaultMapResource(t *testing.T) {
	cases := []struct {
		input *resource.Resource
		// used to replace the resource returned by monitoredresource.Autodetect
		autoRes gcp.Interface
		want    *monitoredrespb.MonitoredResource
	}{
		// Verify that the mapping works and that we skip over the
		// first mapping that doesn't apply.
		{
			input: &resource.Resource{
				Type: resourcekeys.ContainerType,
				Labels: map[string]string{
					stackdriverProjectID:             "proj1",
					resourcekeys.K8SKeyClusterName:   "cluster1",
					resourcekeys.K8SKeyPodName:       "pod1",
					resourcekeys.K8SKeyNamespaceName: "namespace1",
					resourcekeys.ContainerKeyName:    "container-name1",
					resourcekeys.CloudKeyAccountID:   "proj1",
					resourcekeys.CloudKeyZone:        "zone1",
					resourcekeys.CloudKeyRegion:      "",
					"extra_key":                      "must be ignored",
				},
			},
			want: &monitoredrespb.MonitoredResource{
				Type: "k8s_container",
				Labels: map[string]string{
					"project_id":     "proj1",
					"location":       "zone1",
					"cluster_name":   "cluster1",
					"namespace_name": "namespace1",
					"pod_name":       "pod1",
					"container_name": "container-name1",
				},
			},
		},
		{
			input: &resource.Resource{
				Type: resourcekeys.K8SType,
				Labels: map[string]string{
					stackdriverProjectID:             "proj1",
					resourcekeys.K8SKeyClusterName:   "cluster1",
					resourcekeys.K8SKeyPodName:       "pod1",
					resourcekeys.K8SKeyNamespaceName: "namespace1",
					resourcekeys.CloudKeyZone:        "zone1",
				},
			},
			want: &monitoredrespb.MonitoredResource{
				Type: "k8s_pod",
				Labels: map[string]string{
					"project_id":     "proj1",
					"location":       "zone1",
					"cluster_name":   "cluster1",
					"namespace_name": "namespace1",
					"pod_name":       "pod1",
				},
			},
		},
		{
			input: &resource.Resource{
				Type: resourcekeys.HostType,
				Labels: map[string]string{
					stackdriverProjectID:           "proj1",
					resourcekeys.K8SKeyClusterName: "cluster1",
					resourcekeys.CloudKeyZone:      "zone1",
					resourcekeys.HostKeyName:       "node1",
				},
			},
			want: &monitoredrespb.MonitoredResource{
				Type: "k8s_node",
				Labels: map[string]string{
					"project_id":   "proj1",
					"location":     "zone1",
					"cluster_name": "cluster1",
					"node_name":    "node1",
				},
			},
		},
		// Don't match to k8s node if either cluster name or host type are not present
		{
			input: &resource.Resource{
				Type: resourcekeys.HostType,
				Labels: map[string]string{
					stackdriverProjectID:      "proj1",
					resourcekeys.CloudKeyZone: "zone1",
				},
			},
			want: &monitoredrespb.MonitoredResource{
				Type: "global",
				Labels: map[string]string{
					"project_id": "proj1",
				},
			},
		},
		{
			input: &resource.Resource{
				Type: resourcekeys.CloudType,
				Labels: map[string]string{
					stackdriverProjectID:          "proj1",
					resourcekeys.CloudKeyProvider: resourcekeys.CloudProviderGCP,
					resourcekeys.HostKeyID:        "inst1",
					resourcekeys.CloudKeyZone:     "zone1",
					"extra_key":                   "must be ignored",
				},
			},
			want: &monitoredrespb.MonitoredResource{
				Type: "gce_instance",
				Labels: map[string]string{
					"project_id":  "proj1",
					"instance_id": "inst1",
					"zone":        "zone1",
				},
			},
		},
		{
			input: &resource.Resource{
				Type: resourcekeys.CloudType,
				Labels: map[string]string{
					stackdriverProjectID:           "proj1",
					resourcekeys.CloudKeyProvider:  resourcekeys.CloudProviderAWS,
					resourcekeys.HostKeyID:         "inst1",
					resourcekeys.CloudKeyRegion:    "region1",
					resourcekeys.CloudKeyAccountID: "account1",
					"extra_key":                    "must be ignored",
				},
			},
			want: &monitoredrespb.MonitoredResource{
				Type: "aws_ec2_instance",
				Labels: map[string]string{
					"project_id":  "proj1",
					"instance_id": "inst1",
					"region":      "aws:region1",
					"aws_account": "account1",
				},
			},
		},
		// Test autodecting missing Resource labels
		{
			input: &resource.Resource{
				Type: resourcekeys.CloudType,
				Labels: map[string]string{
					stackdriverProjectID:          "proj1",
					resourcekeys.CloudKeyProvider: resourcekeys.CloudProviderAWS,
					"extra_key":                   "must be ignored",
				},
			},
			autoRes: &monitoredresource.AWSEC2Instance{
				AWSAccount: "account1",
				InstanceID: "inst1",
				Region:     "region1",
			},
			want: &monitoredrespb.MonitoredResource{
				Type: "aws_ec2_instance",
				Labels: map[string]string{
					"project_id":  "proj1",
					"instance_id": "inst1",
					"region":      "aws:region1",
					"aws_account": "account1",
				},
			},
		},
		// Test autodetecting partial missing Resource labels
		{
			input: &resource.Resource{
				Type: resourcekeys.CloudType,
				Labels: map[string]string{
					stackdriverProjectID:          "proj1",
					resourcekeys.CloudKeyProvider: resourcekeys.CloudProviderGCP,
					resourcekeys.CloudKeyZone:     "zone1",
				},
			},
			autoRes: &monitoredresource.GCEInstance{
				ProjectID:  "proj1",
				InstanceID: "inst2",
				Zone:       "zone2",
			},
			want: &monitoredrespb.MonitoredResource{
				Type: "gce_instance",
				Labels: map[string]string{
					"project_id":  "proj1",
					"instance_id": "inst2",
					"zone":        "zone1", // incoming labels take precedent over autodetected
				},
			},
		},
		// Test GCP "zone" label is accepted for "location"
		{
			input: &resource.Resource{
				Type: resourcekeys.K8SType,
				Labels: map[string]string{
					resourcekeys.K8SKeyPodName:       "pod1",
					resourcekeys.K8SKeyNamespaceName: "namespace1",
				},
			},
			autoRes: &monitoredresource.GKEContainer{
				ProjectID:                  "proj1",
				ClusterName:                "cluster1",
				Zone:                       "zone1",
				LoggingMonitoringV2Enabled: false,
			},
			want: &monitoredrespb.MonitoredResource{
				Type: "k8s_pod",
				Labels: map[string]string{
					"project_id":     "proj1",
					"location":       "zone1",
					"cluster_name":   "cluster1",
					"namespace_name": "namespace1",
					"pod_name":       "pod1",
				},
			},
		},
		// Convert to App Engine Instance.
		{
			input: &resource.Resource{
				Type: appEngineInstanceType,
				Labels: map[string]string{
					stackdriverProjectID:        "proj1",
					resourcekeys.CloudKeyRegion: "region1",
					appEngineService:            "default",
					appEngineVersion:            "version1",
					appEngineInstance:           "inst1",
				},
			},
			want: &monitoredrespb.MonitoredResource{
				Type: "gae_instance",
				Labels: map[string]string{
					"project_id":  "proj1",
					"location":    "region1",
					"module_id":   "default",
					"version_id":  "version1",
					"instance_id": "inst1",
				},
			},
		},
		// Convert to Global.
		{
			input: &resource.Resource{
				Type: "",
				Labels: map[string]string{
					stackdriverProjectID:            "proj1",
					resourcekeys.CloudKeyZone:       "zone1",
					stackdriverGenericTaskNamespace: "namespace1",
					stackdriverGenericTaskJob:       "job1",
					stackdriverGenericTaskID:        "task_id1",
					resourcekeys.HostKeyID:          "inst1",
				},
			},
			want: &monitoredrespb.MonitoredResource{
				Type: "generic_task",
				Labels: map[string]string{
					"project_id": "proj1",
					"location":   "zone1",
					"namespace":  "namespace1",
					"job":        "job1",
					"task_id":    "task_id1",
				},
			},
		},
		// nil to Global.
		{
			input: nil,
			want: &monitoredrespb.MonitoredResource{
				Type:   "global",
				Labels: nil,
			},
		},
		// no label to Global.
		{
			input: &resource.Resource{
				Type: resourcekeys.K8SType,
			},
			want: &monitoredrespb.MonitoredResource{
				Type:   "global",
				Labels: nil,
			},
		},
		// mapping for knative_revision with autodetected GCP metadata labels
		{
			input: &resource.Resource{
				Type: "knative_revision",
				Labels: map[string]string{
					knativeServiceName:       "helloworld-go",
					knativeRevisionName:      "helloworld-go-hfc7j",
					knativeConfigurationName: "helloworld-go",
					knativeNamespaceName:     "namespace1",
				},
			},
			autoRes: &monitoredresource.GKEContainer{
				ProjectID:   "proj1",
				Zone:        "zone1",
				ClusterName: "cluster1",
			},
			want: &monitoredrespb.MonitoredResource{
				Type: "knative_revision",
				Labels: map[string]string{
					"project_id":         "proj1",
					"service_name":       "helloworld-go",
					"revision_name":      "helloworld-go-hfc7j",
					"location":           "zone1",
					"configuration_name": "helloworld-go",
					"cluster_name":       "cluster1",
					"namespace_name":     "namespace1",
				},
			},
		},
		// mapping for knative_revision with explicit GCP metadata labels
		{
			input: &resource.Resource{
				Type: "knative_revision",
				Labels: map[string]string{
					stackdriverProjectID:           "proj1",
					resourcekeys.CloudKeyZone:      "zone1",
					resourcekeys.K8SKeyClusterName: "cluster1",
					knativeServiceName:             "helloworld-go",
					knativeRevisionName:            "helloworld-go-hfc7j",
					knativeConfigurationName:       "helloworld-go",
					knativeNamespaceName:           "namespace1",
				},
			},
			autoRes: &monitoredresource.GKEContainer{},
			want: &monitoredrespb.MonitoredResource{
				Type: "knative_revision",
				Labels: map[string]string{
					"project_id":         "proj1",
					"service_name":       "helloworld-go",
					"revision_name":      "helloworld-go-hfc7j",
					"location":           "zone1",
					"configuration_name": "helloworld-go",
					"cluster_name":       "cluster1",
					"namespace_name":     "namespace1",
				},
			},
		},
		// Mapping for knative_broker with autodetected GCP metadata labels.
		{
			input: &resource.Resource{
				Type: "knative_broker",
				Labels: map[string]string{
					knativeNamespaceName: "namespace1",
					knativeBrokerName:    "default",
				},
			},
			autoRes: &monitoredresource.GKEContainer{
				ProjectID:   "proj1",
				Zone:        "zone1",
				ClusterName: "cluster1",
			},
			want: &monitoredrespb.MonitoredResource{
				Type: "knative_broker",
				Labels: map[string]string{
					"project_id":     "proj1",
					"location":       "zone1",
					"cluster_name":   "cluster1",
					"namespace_name": "namespace1",
					"broker_name":    "default",
				},
			},
		},
		// Mapping for knative_broker with explicit GCP metadata labels.
		{
			input: &resource.Resource{
				Type: "knative_broker",
				Labels: map[string]string{
					stackdriverProjectID:           "proj1",
					resourcekeys.CloudKeyZone:      "zone1",
					resourcekeys.K8SKeyClusterName: "cluster1",
					knativeNamespaceName:           "namespace1",
					knativeBrokerName:              "default",
				},
			},
			autoRes: &monitoredresource.GKEContainer{},
			want: &monitoredrespb.MonitoredResource{
				Type: "knative_broker",
				Labels: map[string]string{
					"project_id":     "proj1",
					"location":       "zone1",
					"cluster_name":   "cluster1",
					"namespace_name": "namespace1",
					"broker_name":    "default",
				},
			},
		},
		// Mapping for knative_trigger with autodetected GCP metadata labels.
		{
			input: &resource.Resource{
				Type: "knative_trigger",
				Labels: map[string]string{
					knativeNamespaceName: "namespace1",
					knativeBrokerName:    "default",
					knativeTriggerName:   "trigger-storage",
				},
			},
			autoRes: &monitoredresource.GKEContainer{
				ProjectID:   "proj1",
				Zone:        "zone1",
				ClusterName: "cluster1",
			},
			want: &monitoredrespb.MonitoredResource{
				Type: "knative_trigger",
				Labels: map[string]string{
					"project_id":     "proj1",
					"location":       "zone1",
					"cluster_name":   "cluster1",
					"namespace_name": "namespace1",
					"broker_name":    "default",
					"trigger_name":   "trigger-storage",
				},
			},
		},
		// Mapping for knative_trigger with explicit GCP metadata labels.
		{
			input: &resource.Resource{
				Type: "knative_trigger",
				Labels: map[string]string{
					stackdriverProjectID:           "proj1",
					resourcekeys.CloudKeyZone:      "zone1",
					resourcekeys.K8SKeyClusterName: "cluster1",
					knativeNamespaceName:           "namespace1",
					knativeBrokerName:              "default",
					knativeTriggerName:             "trigger-storage",
				},
			},
			autoRes: &monitoredresource.GKEContainer{},
			want: &monitoredrespb.MonitoredResource{
				Type: "knative_trigger",
				Labels: map[string]string{
					"project_id":     "proj1",
					"location":       "zone1",
					"cluster_name":   "cluster1",
					"namespace_name": "namespace1",
					"broker_name":    "default",
					"trigger_name":   "trigger-storage",
				},
			},
		},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("case-%d", i), func(t *testing.T) {
			// defer test cleanup
			defer func() {
				autodetectOnce = new(sync.Once)
				autodetectFunc = dummyAutodetect
			}()

			if c.autoRes != nil {
				autodetectFunc = func() gcp.Interface { return c.autoRes }
			}

			got := DefaultMapResource(c.input)
			if diff := cmp.Diff(got, c.want, protocmp.Transform()); diff != "" {
				t.Errorf("Values differ -got +want: %s", diff)
			}
		})
	}
}
