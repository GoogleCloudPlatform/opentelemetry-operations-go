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
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

const (
	// In GMP, location, cluster, and namespace labels can be overridden by
	// users to set corresponding fields in the monitored resource. To
	// replicate that behavior in the collector, we expect these labels
	// to be moved from metric labels to resource labels using the groupbyattrs
	// processor. If these resource labels are present, use them to set the MR.
	locationLabel         = "location"
	clusterLabel          = "cluster"
	namespaceLabel        = "namespace"
	jobLabel              = "job"
	serviceNamespaceLabel = "service_namespace"
	instanceLabel         = "instance"
	unknownServicePrefix  = "unknown_service"
)

// promTargetKeys are attribute keys which are used in the prometheus_target monitored resource.
// It is also used by GMP to exclude these keys from the target_info metric.
var promTargetKeys = map[string][]string{
	locationLabel: {
		locationLabel,
		string(semconv.CloudAvailabilityZoneKey),
		string(semconv.CloudRegionKey),
	},
	clusterLabel: {
		clusterLabel,
		string(semconv.K8SClusterNameKey),
		string(semconv.K8SClusterUIDKey),
	},
	namespaceLabel: {
		namespaceLabel,
		string(semconv.K8SNamespaceNameKey),
	},
	jobLabel: {
		jobLabel,
		string(semconv.ServiceNameKey),
		string(semconv.FaaSNameKey),
		string(semconv.K8SDeploymentNameKey),
		string(semconv.K8SStatefulSetNameKey),
		string(semconv.K8SDaemonSetNameKey),
		string(semconv.K8SJobNameKey),
		string(semconv.K8SCronJobNameKey),
	},
	serviceNamespaceLabel: {
		string(semconv.ServiceNamespaceKey),
	},
	instanceLabel: {
		instanceLabel,
		string(semconv.ServiceInstanceIDKey),
		string(semconv.FaaSInstanceKey),
		string(semconv.K8SPodNameKey),
		string(semconv.HostIDKey),
	},
}

func (c Config) MapToPrometheusTarget(res pcommon.Resource) *monitoredrespb.MonitoredResource {
	attrs := res.Attributes()
	// Prepend namespace if it exists to match what is specified in
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/datamodel.md#resource-attributes-1
	job := getStringOrEmpty(attrs, promTargetKeys[jobLabel]...)
	serviceNamespace := getStringOrEmpty(attrs, promTargetKeys[serviceNamespaceLabel]...)
	if serviceNamespace != "" {
		job = serviceNamespace + "/" + job
	}
	// Append k8s.container.name to instance if we are using the pod name
	instanceKey, instance := getKeyAndStringOrDefault(attrs, "", promTargetKeys[instanceLabel]...)
	if instanceKey == string(semconv.K8SPodNameKey) {
		if containerName, ok := attrs.Get(string(semconv.K8SContainerNameKey)); ok {
			instance += "/" + containerName.Str()
		}
	}
	return &monitoredrespb.MonitoredResource{
		Type: "prometheus_target",
		Labels: map[string]string{
			locationLabel:  getStringOrEmpty(attrs, promTargetKeys[locationLabel]...),
			clusterLabel:   getStringOrDefaultClusterName(attrs, promTargetKeys[clusterLabel]...),
			namespaceLabel: getStringOrEmpty(attrs, promTargetKeys[namespaceLabel]...),
			jobLabel:       job,
			instanceLabel:  instance,
		},
	}
}

// According to Cloud Monitoring docs, there are special values for
// cluser in some runtimes.
// See: https://cloud.google.com/stackdriver/docs/managed-prometheus/setup-opsagent
func getStringOrDefaultClusterName(attrs pcommon.Map, keys ...string) string {
	defaultClusterName := ""
	cloudPlatform := getStringOrEmpty(attrs, string(semconv.CloudPlatformKey))
	switch cloudPlatform {
	case string(semconv.CloudPlatformGCPComputeEngine.Value.AsString()):
		defaultClusterName = "__gce__"
	case string(semconv.CloudPlatformGCPCloudRun.Value.AsString()):
		defaultClusterName = "__run__"
	}
	return getStringOrDefault(attrs, defaultClusterName, promTargetKeys[clusterLabel]...)
}

// getStringOrDefault returns the value of the first key found, or the orElse string.
func getStringOrDefault(attributes pcommon.Map, orElse string, keys ...string) string {
	_, str := getKeyAndStringOrDefault(attributes, orElse, keys...)
	return str
}

// getStringOrEmpty returns the value of the first key found, or the orElse string.
func getKeyAndStringOrDefault(attributes pcommon.Map, orElse string, keys ...string) (string, string) {
	for _, k := range keys {
		// skip the attribute if it starts with unknown_service, since the SDK
		// sets this by default. It is used as a fallback below if no other
		// values are found.
		if val, ok := attributes.Get(k); ok && !strings.HasPrefix(val.Str(), unknownServicePrefix) {
			return k, val.Str()
		}
	}
	if contains(keys, string(semconv.ServiceNameKey)) {
		// the service name started with unknown_service, and was ignored above
		if val, ok := attributes.Get(string(semconv.ServiceNameKey)); ok {
			return string(semconv.ServiceNameKey), val.Str()
		}
	}
	return "", orElse
}

// getStringOrEmpty returns the value of the first key found, or the empty string.
func getStringOrEmpty(attributes pcommon.Map, keys ...string) string {
	return getStringOrDefault(attributes, "", keys...)
}

func contains(list []string, element string) bool {
	for _, item := range list {
		if item == element {
			return true
		}
	}
	return false
}
