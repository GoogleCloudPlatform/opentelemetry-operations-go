// Copyright 2020, Google Inc.
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

package metric

// TODO: remove this file when the constants are ready in the Go SDK

// Mappings for the well-known OpenTelemetry resource label keys
// to applicable Monitored Resource label keys.
// A uniquely identifying name for the Kubernetes cluster. Kubernetes
// does not have cluster names as an internal concept so this may be
// set to any meaningful value within the environment. For example,
// GKE clusters have a name which can be used for this label.
const (
	CloudKeyProvider  = "cloud.provider"
	CloudKeyAccountID = "cloud.account.id"
	CloudKeyRegion    = "cloud.region"
	CloudKeyZone      = "cloud.zone"

	HostType = "host"
	// A uniquely identifying name for the host.
	HostKeyName = "host.name"
	// A hostname as returned by the 'hostname' command on host machine.
	HostKeyHostName = "host.hostname"
	HostKeyID       = "host.id"
	HostKeyType     = "host.type"

	// A uniquely identifying name for the Container.
	ContainerKeyName      = "container.name"
	ContainerKeyImageName = "container.image.name"
	ContainerKeyImageTag  = "container.image.tag"

	// Cloud Providers
	CloudProviderAWS   = "aws"
	CloudProviderGCP   = "gcp"
	CloudProviderAZURE = "azure"

	K8S                  = "k8s"
	K8SKeyClusterName    = "k8s.cluster.name"
	K8SKeyNamespaceName  = "k8s.namespace.name"
	K8SKeyPodName        = "k8s.pod.name"
	K8SKeyDeploymentName = "k8s.deployment.name"

	// Monitored Resources types
	K8SContainer   = "k8s_container"
	K8SNode        = "k8s_node"
	K8SPod         = "k8s_pod"
	K8SCluster     = "k8s_cluster"
	GCEInstance    = "gce_instance"
	AWSEC2Instance = "aws_ec2_instance"
)
