// Copyright 2021 Google LLC
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
	"go.opentelemetry.io/collector/model/pdata"
	semconv "go.opentelemetry.io/collector/model/semconv/v1.8.0"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

const (
	awsAccount     = "aws_account"
	awsEc2Instance = "aws_ec2_instance"
	clusterName    = "cluster_name"
	containerName  = "container_name"
	gceInstance    = "gce_instance"
	genericNode    = "generic_node"
	genericTask    = "generic_task"
	instanceID     = "instance_id"
	job            = "job"
	k8sCluster     = "k8s_cluster"
	k8sContainer   = "k8s_container"
	k8sNode        = "k8s_node"
	k8sPod         = "k8s_pod"
	location       = "location"
	namespace      = "namespace"
	namespaceName  = "namespace_name"
	nodeID         = "node_id"
	nodeName       = "node_name"
	podName        = "pod_name"
	region         = "region"
	taskID         = "task_id"
)

var (
	// monitoredResourceMappings contains mappings of GCM resource label keys onto OTel
	// resource keys for a given monitored resource type. For entries with multiple OTel
	// resource keys, the keys' values will be coalesced in order until there is a non-empty
	// value.
	monitoredResourceMappings = map[string]map[string][]string{
		gceInstance: {
			location:   {semconv.AttributeCloudAvailabilityZone},
			instanceID: {semconv.AttributeHostID},
		},
		k8sContainer: {
			location:      {semconv.AttributeCloudAvailabilityZone},
			clusterName:   {semconv.AttributeK8SClusterName},
			namespaceName: {semconv.AttributeK8SNamespaceName},
			podName:       {semconv.AttributeK8SPodName},
			containerName: {semconv.AttributeK8SContainerName},
		},
		k8sPod: {
			location:      {semconv.AttributeCloudAvailabilityZone},
			clusterName:   {semconv.AttributeK8SClusterName},
			namespaceName: {semconv.AttributeK8SNamespaceName},
			podName:       {semconv.AttributeK8SPodName},
		},
		k8sNode: {
			location:    {semconv.AttributeCloudAvailabilityZone},
			clusterName: {semconv.AttributeK8SClusterName},
			nodeName:    {semconv.AttributeK8SNodeName},
		},
		k8sCluster: {
			location:    {semconv.AttributeCloudAvailabilityZone},
			clusterName: {semconv.AttributeK8SClusterName},
		},
		awsEc2Instance: {
			instanceID: {semconv.AttributeHostID},
			region:     {semconv.AttributeCloudAvailabilityZone},
			awsAccount: {semconv.AttributeCloudAccountID},
		},
		genericTask: {
			location:  {semconv.AttributeCloudAvailabilityZone, semconv.AttributeCloudRegion},
			namespace: {semconv.AttributeServiceNamespace},
			job:       {semconv.AttributeServiceName},
			taskID:    {semconv.AttributeServiceInstanceID},
		},
		genericNode: {
			location:  {semconv.AttributeCloudAvailabilityZone, semconv.AttributeCloudRegion},
			namespace: {semconv.AttributeServiceNamespace},
			nodeID:    {semconv.AttributeHostID, semconv.AttributeHostName},
		},
	}
)

// Transforms pdata Resource to a GCM Monitored Resource. Any resource attributes not accounted
// for in the monitored resource which should be merged into metric labels are also returned.
func (m *metricMapper) resourceMetricsToMonitoredResource(
	resource pdata.Resource,
) (*monitoredrespb.MonitoredResource, labels) {
	attrs := resource.Attributes()
	cloudPlatform := getStringOrEmpty(attrs, semconv.AttributeCloudPlatform)

	switch cloudPlatform {
	case semconv.AttributeCloudPlatformGCPComputeEngine:
		return createMonitoredResource(gceInstance, attrs)
	case semconv.AttributeCloudPlatformGCPKubernetesEngine:
		// Try for most to least specific k8s_container, k8s_pod, etc
		if _, ok := attrs.Get(semconv.AttributeK8SContainerName); ok {
			return createMonitoredResource(k8sContainer, attrs)
		} else if _, ok := attrs.Get(semconv.AttributeK8SPodName); ok {
			return createMonitoredResource(k8sPod, attrs)
		} else if _, ok := attrs.Get(semconv.AttributeK8SNodeName); ok {
			return createMonitoredResource(k8sNode, attrs)
		}
		return createMonitoredResource(k8sCluster, attrs)
	case semconv.AttributeCloudPlatformAWSEC2:
		return createMonitoredResource(awsEc2Instance, attrs)
	default:
		// Fallback to generic_task
		_, hasServiceName := attrs.Get(semconv.AttributeServiceName)
		_, hasServiceInstanceID := attrs.Get(semconv.AttributeServiceInstanceID)
		if hasServiceName && hasServiceInstanceID {
			return createMonitoredResource(genericTask, attrs)
		}

		// If not possible, fallback to generic_node
		return createMonitoredResource(genericNode, attrs)
	}
}

func createMonitoredResource(
	monitoredResourceType string,
	resourceAttrs pdata.AttributeMap,
) (*monitoredrespb.MonitoredResource, labels) {
	mappings := monitoredResourceMappings[monitoredResourceType]
	mrLabels := make(map[string]string, len(mappings))
	// TODO handle extra labels
	extraLabels := labels{}

	for mrKey, otelKeys := range mappings {
		mrValue := ""
		// Coalesce the possible keys in order
		for _, otelKey := range otelKeys {
			mrValue = getStringOrEmpty(resourceAttrs, otelKey)
			if mrValue != "" {
				break
			}
		}
		mrLabels[mrKey] = mrValue
	}
	return &monitoredrespb.MonitoredResource{
			Type:   monitoredResourceType,
			Labels: mrLabels,
		},
		extraLabels
}

func getStringOrEmpty(attributes pdata.AttributeMap, key string) string {
	if val, ok := attributes.Get(key); ok {
		return val.StringVal()
	}
	return ""
}
