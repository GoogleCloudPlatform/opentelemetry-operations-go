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

package resourcemapping

import (
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
)

const (
	ProjectIDAttributeKey = "gcp.project.id"

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

var (
	// monitoredResourceMappings contains mappings of GCM resource label keys onto mapping config from OTel
	// resource for a given monitored resource type.
	monitoredResourceMappings = map[string]map[string]struct {
		// If none of the otelKeys are present in the Resource, fallback to this literal value
		fallbackLiteral string
		// OTel resource keys to try and populate the resource label from. For entries with
		// multiple OTel resource keys, the keys' values will be coalesced in order until there
		// is a non-empty value.
		otelKeys []string
	}{
		gceInstance: {
			zone:       {otelKeys: []string{string(semconv.CloudAvailabilityZoneKey)}},
			instanceID: {otelKeys: []string{string(semconv.HostIDKey)}},
		},
		k8sContainer: {
			location: {otelKeys: []string{
				string(semconv.CloudAvailabilityZoneKey),
				string(semconv.CloudRegionKey),
			}},
			clusterName:   {otelKeys: []string{string(semconv.K8SClusterNameKey)}},
			namespaceName: {otelKeys: []string{string(semconv.K8SNamespaceNameKey)}},
			podName:       {otelKeys: []string{string(semconv.K8SPodNameKey)}},
			containerName: {otelKeys: []string{string(semconv.K8SContainerNameKey)}},
		},
		k8sPod: {
			location: {otelKeys: []string{
				string(semconv.CloudAvailabilityZoneKey),
				string(semconv.CloudRegionKey),
			}},
			clusterName:   {otelKeys: []string{string(semconv.K8SClusterNameKey)}},
			namespaceName: {otelKeys: []string{string(semconv.K8SNamespaceNameKey)}},
			podName:       {otelKeys: []string{string(semconv.K8SPodNameKey)}},
		},
		k8sNode: {
			location: {otelKeys: []string{
				string(semconv.CloudAvailabilityZoneKey),
				string(semconv.CloudRegionKey),
			}},
			clusterName: {otelKeys: []string{string(semconv.K8SClusterNameKey)}},
			nodeName:    {otelKeys: []string{string(semconv.K8SNodeNameKey)}},
		},
		k8sCluster: {
			location: {otelKeys: []string{
				string(semconv.CloudAvailabilityZoneKey),
				string(semconv.CloudRegionKey),
			}},
			clusterName: {otelKeys: []string{string(semconv.K8SClusterNameKey)}},
		},
		awsEc2Instance: {
			instanceID: {otelKeys: []string{string(semconv.HostIDKey)}},
			region:     {otelKeys: []string{string(semconv.CloudAvailabilityZoneKey)}},
			awsAccount: {otelKeys: []string{string(semconv.CloudAccountIDKey)}},
		},
		genericTask: {
			location: {
				otelKeys: []string{
					string(semconv.CloudAvailabilityZoneKey),
					string(semconv.CloudRegionKey),
				},
				fallbackLiteral: "global",
			},
			namespace: {otelKeys: []string{string(semconv.ServiceNamespaceKey)}},
			job:       {otelKeys: []string{string(semconv.ServiceNameKey)}},
			taskID:    {otelKeys: []string{string(semconv.ServiceInstanceIDKey)}},
		},
		genericNode: {
			location: {
				otelKeys: []string{
					string(semconv.CloudAvailabilityZoneKey),
					string(semconv.CloudRegionKey),
				},
				fallbackLiteral: "global",
			},
			namespace: {otelKeys: []string{string(semconv.ServiceNamespaceKey)}},
			nodeID:    {otelKeys: []string{string(semconv.HostIDKey), string(semconv.HostNameKey)}},
		},
	}
)

type GceResource struct {
	Labels map[string]string
	Type   string
}

// ReadOnlyAttributes is an interface to abstract between pulling attributes from PData library or OTEL SDK.
type ReadOnlyAttributes interface {
	GetString(string) (string, bool)
}

// ResourceAttributesToMonitoredResource converts from a set of OTEL resource attributes into a
// GCP monitored resource type and label set.
// E.g.
// This may output `gce_instance` type with appropriate labels.
func ResourceAttributesToMonitoredResource(attrs ReadOnlyAttributes) *GceResource {
	cloudPlatform, _ := attrs.GetString(string(semconv.CloudPlatformKey))
	var mr *GceResource
	switch cloudPlatform {
	case semconv.CloudPlatformGCPComputeEngine.Value.AsString():
		mr = createMonitoredResource(gceInstance, attrs)
	case semconv.CloudPlatformGCPKubernetesEngine.Value.AsString():
		// Try for most to least specific k8s_container, k8s_pod, etc
		if _, ok := attrs.GetString(string(semconv.K8SContainerNameKey)); ok {
			mr = createMonitoredResource(k8sContainer, attrs)
		} else if _, ok := attrs.GetString(string(semconv.K8SPodNameKey)); ok {
			mr = createMonitoredResource(k8sPod, attrs)
		} else if _, ok := attrs.GetString(string(semconv.K8SNodeNameKey)); ok {
			mr = createMonitoredResource(k8sNode, attrs)
		} else {
			mr = createMonitoredResource(k8sCluster, attrs)
		}
	case semconv.CloudPlatformAWSEC2.Value.AsString():
		mr = createMonitoredResource(awsEc2Instance, attrs)
	default:
		// Fallback to generic_task
		_, hasServiceName := attrs.GetString(string(semconv.ServiceNameKey))
		_, hasServiceInstanceID := attrs.GetString(string(semconv.ServiceInstanceIDKey))
		if hasServiceName && hasServiceInstanceID {
			mr = createMonitoredResource(genericTask, attrs)
		} else {
			// If not possible, fallback to generic_node
			mr = createMonitoredResource(genericNode, attrs)
		}
	}
	return mr
}

func createMonitoredResource(
	monitoredResourceType string,
	resourceAttrs ReadOnlyAttributes,
) *GceResource {
	mappings := monitoredResourceMappings[monitoredResourceType]
	mrLabels := make(map[string]string, len(mappings))

	for mrKey, mappingConfig := range mappings {
		mrValue := ""
		ok := false
		// Coalesce the possible keys in order
		for _, otelKey := range mappingConfig.otelKeys {
			mrValue, ok = resourceAttrs.GetString(otelKey)
			if mrValue != "" {
				break
			}
		}
		if !ok || mrValue == "" {
			mrValue = mappingConfig.fallbackLiteral
		}
		mrLabels[mrKey] = mrValue
	}
	return &GceResource{
		Type:   monitoredResourceType,
		Labels: mrLabels,
	}
}
