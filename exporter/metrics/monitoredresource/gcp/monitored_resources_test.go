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

package gcp

import (
	"os"
	"testing"
)

const (
	GCPProjectIDStr     = "gcp-project"
	GCPInstanceIDStr    = "instance"
	GCPZoneStr          = "us-east1"
	GKENamespaceStr     = "namespace"
	GKEPodIDStr         = "pod-id"
	GKEContainerNameStr = "container"
	GKEClusterNameStr   = "cluster"
)

func TestGKEContainerMonitoredResources(t *testing.T) {
	os.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	gcpMetadata := gcpMetadata{
		instanceID:    GCPInstanceIDStr,
		projectID:     GCPProjectIDStr,
		zone:          GCPZoneStr,
		clusterName:   GKEClusterNameStr,
		containerName: GKEContainerNameStr,
		namespaceID:   GKENamespaceStr,
		podID:         GKEPodIDStr,
	}
	autoDetected := detectResourceType(&gcpMetadata)

	if autoDetected == nil {
		t.Fatal("GKEContainerMonitoredResource nil")
	}
	resType, labels := autoDetected.MonitoredResource()
	if resType != "gke_container" ||
		labels["instance_id"] != GCPInstanceIDStr ||
		labels["project_id"] != GCPProjectIDStr ||
		labels["cluster_name"] != GKEClusterNameStr ||
		labels["container_name"] != GKEContainerNameStr ||
		labels["zone"] != GCPZoneStr ||
		labels["namespace_id"] != GKENamespaceStr ||
		labels["pod_id"] != GKEPodIDStr {
		t.Errorf("GKEContainerMonitoredResource Failed: %v", autoDetected)
	}
}

func TestGKEContainerMonitoredResourcesV2(t *testing.T) {
	os.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	gcpMetadata := gcpMetadata{
		instanceID:    GCPInstanceIDStr,
		projectID:     GCPProjectIDStr,
		zone:          GCPZoneStr,
		clusterName:   GKEClusterNameStr,
		containerName: GKEContainerNameStr,
		namespaceID:   GKENamespaceStr,
		podID:         GKEPodIDStr,
		monitoringV2:  true,
	}
	autoDetected := detectResourceType(&gcpMetadata)

	if autoDetected == nil {
		t.Fatal("GKEContainerMonitoredResource nil")
	}
	resType, labels := autoDetected.MonitoredResource()
	if resType != "k8s_container" ||
		labels["project_id"] != GCPProjectIDStr ||
		labels["cluster_name"] != GKEClusterNameStr ||
		labels["container_name"] != GKEContainerNameStr ||
		labels["location"] != GCPZoneStr ||
		labels["namespace_name"] != GKENamespaceStr ||
		labels["pod_name"] != GKEPodIDStr {
		t.Errorf("GKEContainerMonitoredResourceV2 Failed: %v", autoDetected)
	}
}

func TestGCEInstanceMonitoredResources(t *testing.T) {
	os.Setenv("KUBERNETES_SERVICE_HOST", "")
	gcpMetadata := gcpMetadata{
		instanceID: GCPInstanceIDStr,
		projectID:  GCPProjectIDStr,
		zone:       GCPZoneStr,
	}
	autoDetected := detectResourceType(&gcpMetadata)

	if autoDetected == nil {
		t.Fatal("GCEInstanceMonitoredResource nil")
	}
	resType, labels := autoDetected.MonitoredResource()
	if resType != "gce_instance" ||
		labels["instance_id"] != GCPInstanceIDStr ||
		labels["project_id"] != GCPProjectIDStr ||
		labels["zone"] != GCPZoneStr {
		t.Errorf("GCEInstanceMonitoredResource Failed: %v", autoDetected)
	}
}
