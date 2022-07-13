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

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"google.golang.org/api/option"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

const (
	DefaultTimeout = 12 * time.Second // Consistent with Cloud Monitoring's timeout
)

// Config defines configuration for Google Cloud exporter.
type Config struct {
	ImpersonateConfig ImpersonateConfig `mapstructure:"impersonate"`
	// ProjectID is the project telemetry is sent to if the gcp.project.id
	// resource attribute is not set. If unspecified, this is determined using
	// application default credentials.
	ProjectID    string       `mapstructure:"project"`
	UserAgent    string       `mapstructure:"user_agent"`
	TraceConfig  TraceConfig  `mapstructure:"trace"`
	LogConfig    LogConfig    `mapstructure:"log"`
	MetricConfig MetricConfig `mapstructure:"metric"`
}

type ClientConfig struct {
	// GetClientOptions returns additional options to be passed
	// to the underlying Google Cloud API client.
	// Must be set programmatically (no support via declarative config).
	// Optional.
	GetClientOptions func() []option.ClientOption

	Endpoint string `mapstructure:"endpoint"`
	// Only has effect if Endpoint is not ""
	UseInsecure bool `mapstructure:"use_insecure"`
	// GRPCPoolSize sets the size of the connection pool in the GCP client
	GRPCPoolSize int `mapstructure:"grpcPoolSize"`
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
	// GetMetricName is not settable in config files, but can be used by other
	// exporters which extend the functionality of this exporter. It allows
	// customizing the naming of metrics. baseName already includes type
	// suffixes for summary metrics, but does not (yet) include the domain prefix
	GetMetricName func(baseName string, metric pmetric.Metric) (string, error)
	// MapMonitoredResource is not exposed as an option in the configuration, but
	// can be used by other exporters to extend the functionality of this
	// exporter. It allows overriding the function used to map otel resource to
	// monitored resource.
	MapMonitoredResource func(pcommon.Resource) *monitoredrespb.MonitoredResource
	Prefix               string       `mapstructure:"prefix"`
	ClientConfig         ClientConfig `mapstructure:",squash"`
	// KnownDomains contains a list of prefixes. If a metric already has one
	// of these prefixes, the prefix is not added.
	KnownDomains []string `mapstructure:"known_domains"`
	// ResourceFilters, if provided, provides a list of resource filters.
	// Resource attributes matching any filter will be included in metric labels.
	// Defaults to empty, which won't include any additional resource labels. Note that the
	// service_resource_labels option operates independently from resource_filters.
	ResourceFilters []ResourceFilter `mapstructure:"resource_filters"`
	// CreateMetricDescriptorBufferSize is the buffer size for the channel
	// which asynchronously calls CreateMetricDescriptor. Default is 10.
	CreateMetricDescriptorBufferSize int  `mapstructure:"create_metric_descriptor_buffer_size"`
	SkipCreateMetricDescriptor       bool `mapstructure:"skip_create_descriptor"`
	// CreateServiceTimeSeries, if true, this will send all timeseries using `CreateServiceTimeSeries`.
	// Implicitly, this sets `SkipMetricDescriptor` to true.
	CreateServiceTimeSeries bool `mapstructure:"create_service_timeseries"`
	// InstrumentationLibraryLabels, if true, set the instrumentation_source
	// and instrumentation_version labels. Defaults to true.
	InstrumentationLibraryLabels bool `mapstructure:"instrumentation_library_labels"`
	// ServiceResourceLabels, if true, causes the exporter to copy OTel's
	// service.name, service.namespace, and service.instance.id resource attributes into the GCM timeseries metric labels. This
	// option is recommended to avoid writing duplicate timeseries against the same monitored
	// resource. Disabling this option does not prevent resource_filters from adding those
	// labels. Default is true.
	ServiceResourceLabels bool `mapstructure:"service_resource_labels"`
	// CumulativeNormalization normalizes cumulative metrics without start times or with
	// explicit reset points by subtracting subsequent points from the initial point.
	// It is enabled by default. Since it caches starting points, it may result in
	// increased memory usage.
	CumulativeNormalization bool `mapstructure:"cumulative_normalization"`
	// EnableSumOfSquaredDeviation enables calculation of an estimated sum of squared
	// deviation.  It isn't correct, so we don't send it by default, and don't expose
	// it to users. For some uses, it is expected, however.
	EnableSumOfSquaredDeviation bool `mapstructure:"sum_of_squared_deviation"`
}

// ImpersonateConfig defines configuration for service account impersonation
type ImpersonateConfig struct {
	TargetPrincipal string   `mapstructure:"target_principal"`
	Subject         string   `mapstructure:"subject"`
	Delegates       []string `mapstructure:"delegates"`
}

type ResourceFilter struct {
	// Match resource keys by prefix
	Prefix string `mapstructure:"prefix"`
}

type LogConfig struct {
	// DefaultLogName sets the fallback log name to use when one isn't explicitly set
	// for a log entry. If unset, logs without a log name will raise an error.
	DefaultLogName string       `mapstructure:"default_log_name"`
	ClientConfig   ClientConfig `mapstructure:",squash"`
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
			GetMetricName:                    defaultGetMetricName,
			MapMonitoredResource:             defaultResourceToMonitoredResource,
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
