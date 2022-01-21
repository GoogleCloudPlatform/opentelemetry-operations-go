// Copyright 2021 OpenTelemetry Authors
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

package collector

import (
	"time"

	"google.golang.org/api/option"
)

const (
	DefaultTimeout = 12 * time.Second // Consistent with Cloud Monitoring's timeout
)

// Config defines configuration for Google Cloud exporter.
type Config struct {
	ProjectID string `mapstructure:"project"`
	UserAgent string `mapstructure:"user_agent"`
	Endpoint  string `mapstructure:"endpoint"`
	// Only has effect if Endpoint is not ""
	UseInsecure bool `mapstructure:"use_insecure"`

	ResourceMappings []ResourceMapping `mapstructure:"resource_mappings"`
	// GetClientOptions returns additional options to be passed
	// to the underlying Google Cloud API client.
	// Must be set programmatically (no support via declarative config).
	// Optional.
	GetClientOptions func() []option.ClientOption

	MetricConfig MetricConfig `mapstructure:"metric"`
}

type MetricConfig struct {
	Prefix                     string `mapstructure:"prefix"`
	SkipCreateMetricDescriptor bool   `mapstructure:"skip_create_descriptor"`
	// If a metric belongs to one of these domains it does not get a prefix.
	KnownDomains []string `mapstructure:"known_domains"`

	// If true, set the instrumentation_source and instrumentation_version
	// labels. Defaults to true.
	InstrumentationLibraryLabels bool `mapstructure:"instrumentation_library_labels"`

	// If true, this will send all timeseries using `CreateServiceTimeSeries`.
	// Implicitly, this sets `SkipMetricDescriptor` to true.
	CreateServiceTimeSeries bool `mapstructure:"create_service_timeseries"`

	// Buffer size for the channel which asynchronously calls CreateMetricDescriptor. Default
	// is 10.
	CreateMetricDescriptorBufferSize int `mapstructure:"create_metric_descriptor_buffer_size"`

	// If true, the exporter will copy OTel's service.name, service.namespace, and
	// service.instance.id resource attributes into the GCM timeseries metric labels. This
	// option is recommended to avoid writing duplicate timeseries against the same monitored
	// resource. Default is true.
	ServiceResourceLabels bool `mapstructure:"service_resource_labels"`

	// If provided, resource attributes matching any filter will be included in metric labels.
	// Defaults to empty, which won't include any additional resource labels. Note that the
	// service_resource_labels option operates independently from resource_filters.
	ResourceFilters []ResourceFilter `mapstructure:"resource_filters"`
}

type ResourceFilter struct {
	// Match resource keys by prefix
	Prefix string `mapstructure:"prefix"`
}

// ResourceMapping defines mapping of resources from source (OpenCensus) to target (Google Cloud).
type ResourceMapping struct {
	SourceType string `mapstructure:"source_type"`
	TargetType string `mapstructure:"target_type"`

	LabelMappings []LabelMapping `mapstructure:"label_mappings"`
}

type LabelMapping struct {
	SourceKey string `mapstructure:"source_key"`
	TargetKey string `mapstructure:"target_key"`
	// Optional flag signals whether we can proceed with transformation if a label is missing in the resource.
	// When required label is missing, we fallback to default resource mapping.
	Optional bool `mapstructure:"optional"`
}

// Known metric domains. Note: This is now configurable for advanced usages.
var domains = []string{"googleapis.com", "kubernetes.io", "istio.io", "knative.dev"}

// DefaultConfig creates the default configuration for exporter.
func DefaultConfig() Config {
	return Config{
		UserAgent: "opentelemetry-collector-contrib {{version}}",
		MetricConfig: MetricConfig{
			KnownDomains:                     domains,
			Prefix:                           "workload.googleapis.com",
			CreateMetricDescriptorBufferSize: 10,
			InstrumentationLibraryLabels:     true,
			ServiceResourceLabels:            true,
		},
	}
}
