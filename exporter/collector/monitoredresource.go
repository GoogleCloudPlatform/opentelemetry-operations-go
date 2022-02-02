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
	zone           = "zone"
)

// mappingConfig maps from the required fields for one monitored resource to the OTel resource
// attributes that can be used to populate it.
type mappingConfig map[string]keyMatcher

// keyMatcher determines how to select otel resource attribute keys
type keyMatcher struct {
	// OTel resource keys to try and populate the resource label from. For entries with
	// multiple OTel resource keys, the keys' values will be coalesced in order until there
	// is a non-empty value.
	otelKeys []string
	// If none of the otelKeys are present in the Resource, fallback to this literal value
	fallbackLiteral string
}

var (
	// baseMonitoredResourceMappings contains mappings of GCM resource label keys onto mapping config from OTel
	// resource for a given monitored resource type.
	baseMonitoredResourceMappings = map[string]mappingConfig{
		gceInstance: {
			zone:       {otelKeys: []string{semconv.AttributeCloudAvailabilityZone}},
			instanceID: {otelKeys: []string{semconv.AttributeHostID}},
		},
		k8sContainer: {
			location:      {otelKeys: []string{semconv.AttributeCloudAvailabilityZone}},
			clusterName:   {otelKeys: []string{semconv.AttributeK8SClusterName}},
			namespaceName: {otelKeys: []string{semconv.AttributeK8SNamespaceName}},
			podName:       {otelKeys: []string{semconv.AttributeK8SPodName}},
			containerName: {otelKeys: []string{semconv.AttributeK8SContainerName}},
		},
		k8sPod: {
			location:      {otelKeys: []string{semconv.AttributeCloudAvailabilityZone}},
			clusterName:   {otelKeys: []string{semconv.AttributeK8SClusterName}},
			namespaceName: {otelKeys: []string{semconv.AttributeK8SNamespaceName}},
			podName:       {otelKeys: []string{semconv.AttributeK8SPodName}},
		},
		k8sNode: {
			location:    {otelKeys: []string{semconv.AttributeCloudAvailabilityZone}},
			clusterName: {otelKeys: []string{semconv.AttributeK8SClusterName}},
			nodeName:    {otelKeys: []string{semconv.AttributeK8SNodeName}},
		},
		k8sCluster: {
			location:    {otelKeys: []string{semconv.AttributeCloudAvailabilityZone}},
			clusterName: {otelKeys: []string{semconv.AttributeK8SClusterName}},
		},
		awsEc2Instance: {
			instanceID: {otelKeys: []string{semconv.AttributeHostID}},
			region:     {otelKeys: []string{semconv.AttributeCloudAvailabilityZone}},
			awsAccount: {otelKeys: []string{semconv.AttributeCloudAccountID}},
		},
		genericTask: {
			location: {
				otelKeys: []string{
					semconv.AttributeCloudAvailabilityZone,
					semconv.AttributeCloudRegion,
				},
				fallbackLiteral: "global",
			},
			namespace: {otelKeys: []string{semconv.AttributeServiceNamespace}},
			job:       {otelKeys: []string{semconv.AttributeServiceName}},
			taskID:    {otelKeys: []string{semconv.AttributeServiceInstanceID}},
		},
		genericNode: {
			location: {
				otelKeys: []string{
					semconv.AttributeCloudAvailabilityZone,
					semconv.AttributeCloudRegion,
				},
				fallbackLiteral: "global",
			},
			namespace: {otelKeys: []string{semconv.AttributeServiceNamespace}},
			nodeID:    {otelKeys: []string{semconv.AttributeHostID, semconv.AttributeHostName}},
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
		return m.createMonitoredResource(gceInstance, attrs)
	case semconv.AttributeCloudPlatformGCPKubernetesEngine:
		// Try for most to least specific k8s_container, k8s_pod, etc
		if _, ok := attrs.Get(semconv.AttributeK8SContainerName); ok {
			return m.createMonitoredResource(k8sContainer, attrs)
		} else if _, ok := attrs.Get(semconv.AttributeK8SPodName); ok {
			return m.createMonitoredResource(k8sPod, attrs)
		} else if _, ok := attrs.Get(semconv.AttributeK8SNodeName); ok {
			return m.createMonitoredResource(k8sNode, attrs)
		}
		return m.createMonitoredResource(k8sCluster, attrs)
	case semconv.AttributeCloudPlatformAWSEC2:
		return m.createMonitoredResource(awsEc2Instance, attrs)
	default:
		// Fallback to generic_task
		_, hasServiceName := attrs.Get(semconv.AttributeServiceName)
		_, hasServiceInstanceID := attrs.Get(semconv.AttributeServiceInstanceID)
		if hasServiceName && hasServiceInstanceID {
			return m.createMonitoredResource(genericTask, attrs)
		}

		// If not possible, fallback to generic_node
		return m.createMonitoredResource(genericNode, attrs)
	}
}

func (m *metricMapper) createMonitoredResource(
	monitoredResourceType string,
	resourceAttrs pdata.AttributeMap,
) (*monitoredrespb.MonitoredResource, labels) {
	mappings := m.monitoredResourceMappings[monitoredResourceType]
	mrLabels := make(map[string]string, len(mappings))
	// TODO handle extra labels
	extraLabels := labels{}

	for mrKey, mappingConfig := range mappings {
		mrValue := ""
		// Coalesce the possible keys in order
		for _, otelKey := range mappingConfig.otelKeys {
			mrValue = getStringOrEmpty(resourceAttrs, otelKey)
			if mrValue != "" {
				break
			}
		}
		if mrValue == "" {
			mrValue = mappingConfig.fallbackLiteral
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
