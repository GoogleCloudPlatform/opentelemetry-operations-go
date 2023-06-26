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
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/option"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding/gzip"
)

const (
	DefaultTimeout = 12 * time.Second // Consistent with Cloud Monitoring's timeout
)

// Config defines configuration for Google Cloud exporter.
type Config struct {
	// ProjectID is the project telemetry is sent to if the gcp.project.id
	// resource attribute is not set. If unspecified, this is determined using
	// application default credentials.
	ProjectID               string            `mapstructure:"project"`
	UserAgent               string            `mapstructure:"user_agent"`
	ImpersonateConfig       ImpersonateConfig `mapstructure:"impersonate"`
	TraceConfig             TraceConfig       `mapstructure:"trace"`
	LogConfig               LogConfig         `mapstructure:"log"`
	MetricConfig            MetricConfig      `mapstructure:"metric"`
	DestinationProjectQuota bool              `mapstructure:"destination_project_quota"`
}

type ClientConfig struct {
	// GetClientOptions returns additional options to be passed
	// to the underlying Google Cloud API client.
	// Must be set programmatically (no support via declarative config).
	// If GetClientOptions returns any options, the exporter will not add the
	// default credentials, as those could conflict with options provided via
	// GetClientOptions.
	// Optional.
	GetClientOptions func() []option.ClientOption

	Endpoint string `mapstructure:"endpoint"`
	// Compression specifies the compression format for Metrics and Logging gRPC requests.
	// Supported values: gzip.
	Compression string `mapstructure:"compression"`
	// Only has effect if Endpoint is not ""
	UseInsecure bool `mapstructure:"use_insecure"`
	// GRPCPoolSize sets the size of the connection pool in the GCP client
	GRPCPoolSize int `mapstructure:"grpc_pool_size"`
}

type TraceConfig struct {
	// AttributeMappings determines how to map from OpenTelemetry attribute
	// keys to Google Cloud Trace keys.  By default, it changes http and
	// service keys so that they appear more prominently in the UI.
	AttributeMappings []AttributeMapping `mapstructure:"attribute_mappings"`

	ClientConfig ClientConfig `mapstructure:",squash"`
}

// AttributeMapping maps from an OpenTelemetry key to a Google Cloud Trace key.
type AttributeMapping struct {
	// Key is the OpenTelemetry attribute key
	Key string `mapstructure:"key"`
	// Replacement is the attribute sent to Google Cloud Trace
	Replacement string `mapstructure:"replacement"`
}

type MetricConfig struct {
	// MapMonitoredResource is not exposed as an option in the configuration, but
	// can be used by other exporters to extend the functionality of this
	// exporter. It allows overriding the function used to map otel resource to
	// monitored resource.
	MapMonitoredResource func(pcommon.Resource) *monitoredrespb.MonitoredResource
	// ExtraMetrics is an extension point for exporters to add to the set
	// of ResourceMetrics during a call to PushMetrics.
	ExtraMetrics func(pmetric.Metrics) pmetric.ResourceMetricsSlice
	// GetMetricName is not settable in config files, but can be used by other
	// exporters which extend the functionality of this exporter. It allows
	// customizing the naming of metrics. baseName already includes type
	// suffixes for summary metrics, but does not (yet) include the domain prefix
	GetMetricName func(baseName string, metric pmetric.Metric) (string, error)
	// WALConfig holds configuration settings for the write ahead log.
	WALConfig *WALConfig `mapstructure:"experimental_wal_config"`
	Prefix    string     `mapstructure:"prefix"`
	// KnownDomains contains a list of prefixes. If a metric already has one
	// of these prefixes, the prefix is not added.
	KnownDomains []string `mapstructure:"known_domains"`
	// ResourceFilters, if provided, provides a list of resource filters.
	// Resource attributes matching any filter will be included in metric labels.
	// Defaults to empty, which won't include any additional resource labels. Note that the
	// service_resource_labels option operates independently from resource_filters.
	ResourceFilters []ResourceFilter `mapstructure:"resource_filters"`
	ClientConfig    ClientConfig     `mapstructure:",squash"`
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

// WALConfig defines settings for the write ahead log. WAL buffering writes data
// points in-order to disk before reading and exporting them. This allows for
// better retry logic when exporting fails (such as a network outage), because
// it preserves both the data on disk and the order of the data points.
type WALConfig struct {
	// Directory is the location to store WAL files.
	Directory string `mapstructure:"directory"`
	// MaxBackoff sets the length of time to exponentially re-try failed exports.
	MaxBackoff time.Duration `mapstructure:"max_backoff"`
}

// ImpersonateConfig defines configuration for service account impersonation.
type ImpersonateConfig struct {
	TargetPrincipal string   `mapstructure:"target_principal"`
	Subject         string   `mapstructure:"subject"`
	Delegates       []string `mapstructure:"delegates"`
}

type ResourceFilter struct {
	// Match resource keys by prefix
	Prefix string `mapstructure:"prefix"`
	// Match resource keys by regex
	Regex string `mapstructure:"regex"`
}

type LogConfig struct {
	// DefaultLogName sets the fallback log name to use when one isn't explicitly set
	// for a log entry. If unset, logs without a log name will raise an error.
	DefaultLogName string `mapstructure:"default_log_name"`
	// ResourceFilters, if provided, provides a list of resource filters.
	// Resource attributes matching any filter will be included in LogEntry labels.
	// Defaults to empty, which won't include any additional resource labels.
	ResourceFilters []ResourceFilter `mapstructure:"resource_filters"`
	ClientConfig    ClientConfig     `mapstructure:",squash"`
	// ServiceResourceLabels, if true, causes the exporter to copy OTel's
	// service.name, service.namespace, and service.instance.id resource attributes into the Cloud Logging LogEntry labels.
	// Disabling this option does not prevent resource_filters from adding those labels. Default is true.
	ServiceResourceLabels bool `mapstructure:"service_resource_labels"`
}

// Known metric domains. Note: This is now configurable for advanced usages.
var domains = []string{"googleapis.com", "kubernetes.io", "istio.io", "knative.dev"}

// DefaultConfig creates the default configuration for exporter.
func DefaultConfig() Config {
	return Config{
		UserAgent: "opentelemetry-collector-contrib {{version}}",
		LogConfig: LogConfig{
			ServiceResourceLabels: true,
		},
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

// ValidateConfig returns an error if the provided configuration is invalid.
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

	for _, resourceFilter := range cfg.MetricConfig.ResourceFilters {
		if len(resourceFilter.Regex) == 0 {
			continue
		}
		if _, err := regexp.Compile(resourceFilter.Regex); err != nil {
			return fmt.Errorf("unable to parse resource filter regex: %s", err.Error())
		}
	}

	if len(cfg.LogConfig.ClientConfig.Compression) > 0 && cfg.LogConfig.ClientConfig.Compression != gzip.Name {
		return fmt.Errorf("unknown compression option '%s', allowed values: '', 'gzip'", cfg.LogConfig.ClientConfig.Compression)
	}
	if len(cfg.MetricConfig.ClientConfig.Compression) > 0 && cfg.MetricConfig.ClientConfig.Compression != gzip.Name {
		return fmt.Errorf("unknown compression option '%s', allowed values: '', 'gzip'", cfg.MetricConfig.ClientConfig.Compression)
	}
	if len(cfg.TraceConfig.ClientConfig.Compression) > 0 {
		return fmt.Errorf("traces.compression invalid: compression is only available for logs and metrics")
	}

	return nil
}

func setVersionInUserAgent(cfg *Config, version string) {
	cfg.UserAgent = strings.ReplaceAll(cfg.UserAgent, "{{version}}", version)
}

func generateClientOptions(ctx context.Context, clientCfg *ClientConfig, cfg *Config, scopes []string) ([]option.ClientOption, error) {
	var copts []option.ClientOption
	// option.WithUserAgent is used by the Trace exporter, but not the Metric exporter (see comment below)
	if cfg.UserAgent != "" {
		copts = append(copts, option.WithUserAgent(cfg.UserAgent))
	}
	if clientCfg.Endpoint != "" {
		if clientCfg.UseInsecure {
			// option.WithGRPCConn option takes precedent over all other supplied options so the
			// following user agent will be used by both exporters if we reach this branch
			dialOpts := []grpc.DialOption{
				grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			}
			if cfg.UserAgent != "" {
				dialOpts = append(dialOpts, grpc.WithUserAgent(cfg.UserAgent))
			}
			conn, err := grpc.Dial(clientCfg.Endpoint, dialOpts...)
			if err != nil {
				return nil, fmt.Errorf("cannot configure grpc conn: %w", err)
			}
			copts = append(copts, option.WithGRPCConn(conn))
		} else {
			copts = append(copts, option.WithEndpoint(clientCfg.Endpoint))
		}
	}
	if cfg.ImpersonateConfig.TargetPrincipal != "" {
		if cfg.ProjectID == "" {
			creds, err := google.FindDefaultCredentials(ctx, scopes...)
			if err != nil {
				return nil, fmt.Errorf("error finding default application credentials: %v", err)
			}
			cfg.ProjectID = creds.ProjectID
		}
		tokenSource, err := impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
			TargetPrincipal: cfg.ImpersonateConfig.TargetPrincipal,
			Delegates:       cfg.ImpersonateConfig.Delegates,
			Subject:         cfg.ImpersonateConfig.Subject,
			Scopes:          scopes,
		})
		if err != nil {
			return nil, err
		}
		copts = append(copts, option.WithTokenSource(tokenSource))
	} else if !clientCfg.UseInsecure && (clientCfg.GetClientOptions == nil || len(clientCfg.GetClientOptions()) == 0) {
		// Only add default credentials if GetClientOptions does not
		// provide additional options since GetClientOptions could pass
		// credentials which conflict with the default creds.
		creds, err := google.FindDefaultCredentials(ctx, scopes...)
		if err != nil {
			return nil, fmt.Errorf("error finding default application credentials: %v", err)
		}
		copts = append(copts, option.WithCredentials(creds))
		if cfg.ProjectID == "" {
			cfg.ProjectID = creds.ProjectID
		}
	}
	if clientCfg.GRPCPoolSize > 0 {
		copts = append(copts, option.WithGRPCConnectionPool(clientCfg.GRPCPoolSize))
	}
	if clientCfg.GetClientOptions != nil {
		copts = append(copts, clientCfg.GetClientOptions()...)
	}
	if cfg.ProjectID == "" {
		return nil, errors.New("no project set in config, or found with application default credentials")
	}
	return copts, nil
}
