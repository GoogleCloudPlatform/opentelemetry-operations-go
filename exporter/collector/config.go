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
	"fmt"
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

	TraceConfig TraceConfig `mapstructure:"trace"`

	MetricConfig MetricConfig `mapstructure:"metric"`

	LogConfig LogConfig `mapstructure:"log"`
}

type ClientConfig struct {
	Endpoint string `mapstructure:"endpoint"`
	// Only has effect if Endpoint is not ""
	UseInsecure bool `mapstructure:"use_insecure"`

	// GetClientOptions returns additional options to be passed
	// to the underlying Google Cloud API client.
	// Must be set programmatically (no support via declarative config).
	// Optional.
	GetClientOptions func() []option.ClientOption
}

type TraceConfig struct {
	ClientConfig ClientConfig `mapstructure:",squash"`
	// AttributeMappings determines how to map from OpenTelemetry attribute
	// keys to Google Cloud Trace keys.  By default, it changes http and
	// service keys so that they appear more prominently in the UI.
	AttributeMappings []AttributeMapping `mapstructure:"attribute_mappings"`
}

// AttributeMapping maps from an OpenTelemetry key to a Google Cloud Trace key.
type AttributeMapping struct {
	// Key is the OpenTelemetry attribute key
	Key string `mapstructure:"key"`
	// Replacement is the attribute sent to Google Cloud Trace
	Replacement string `mapstructure:"replacement"`
}

type MetricConfig struct {
	ClientConfig               ClientConfig `mapstructure:",squash"`
	Prefix                     string       `mapstructure:"prefix"`
	SkipCreateMetricDescriptor bool         `mapstructure:"skip_create_descriptor"`
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
	// resource. Disabling this option does not prevent resource_filters from adding those
	// labels. Default is true.
	ServiceResourceLabels bool `mapstructure:"service_resource_labels"`

	// If provided, resource attributes matching any filter will be included in metric labels.
	// Defaults to empty, which won't include any additional resource labels. Note that the
	// service_resource_labels option operates independently from resource_filters.
	ResourceFilters []ResourceFilter `mapstructure:"resource_filters"`

	// CumulativeNormalization normalizes cumulative metrics without start times or with
	// explicit reset points by subtracting subsequent points from the initial point.
	// It is enabled by default. Since it caches starting points, it may result in
	// increased memory usage.
	CumulativeNormalization bool `mapstructure:"cumulative_normalization"`

	// This enables calculation of an estimated sum of squared deviation.  It isn't correct,
	// so we don't send it by default, and don't expose it to users. For some uses, it is
	// expected, however.
	EnableSumOfSquaredDeviation bool `mapstructure:"sum_of_squared_deviation"`
}

type ResourceFilter struct {
	// Match resource keys by prefix
	Prefix string `mapstructure:"prefix"`
}

type LogConfig struct {
	ClientConfig ClientConfig `mapstructure:",squash"`

	// DefaultLogLanme sets the fallback log name to use when one isn't explicitly set
	// for a log entry. If unset, logs without a log name will raise an error.
	DefaultLogName string `mapstructure:"default_log_name"`
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
			CumulativeNormalization:          true,
		},
	}
}

// ValidateConfig returns an error if the provided configuration is invalid
func ValidateConfig(cfg Config) error {
	seenKeys := make(map[string]struct{}, len(cfg.TraceConfig.AttributeMappings))
	seenReplacements := make(map[string]struct{}, len(cfg.TraceConfig.AttributeMappings))
	for _, mapping := range cfg.TraceConfig.AttributeMappings {
		if _, ok := seenKeys[mapping.Key]; ok {
			return fmt.Errorf("duplicate key in traces.attribute_mappings: %q", mapping.Key)
		}
		seenKeys[mapping.Key] = struct{}{}
		if _, ok := seenReplacements[mapping.Replacement]; ok {
			return fmt.Errorf("duplicate replacement in traces.attribute_mappings: %q", mapping.Replacement)
		}
		seenReplacements[mapping.Replacement] = struct{}{}
	}
	return nil
}
