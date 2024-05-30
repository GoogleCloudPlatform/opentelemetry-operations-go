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

package testcases

import (
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"google.golang.org/api/option"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric"
)

var MetricsTestCases = []TestCase{
	// Tests for the basic exporter
	{
		Name:                 "Sum becomes a GCM Cumulative",
		OTLPInputFixturePath: "testdata/fixtures/metrics/counter.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/counter_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.InstrumentationLibraryLabels = true
			// Disable service resource labels here and below to keep output examples simpler.
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		MetricSDKExporterOptions: []metric.Option{metric.WithFilteredResourceAttributes(metric.NoAttributes)},
	},
	{
		Name:                 "Delta Sum becomes a GCM cumulative",
		OTLPInputFixturePath: "testdata/fixtures/metrics/delta_counter.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/delta_counter_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		MetricSDKExporterOptions: []metric.Option{metric.WithFilteredResourceAttributes(metric.NoAttributes)},
	},
	{
		Name:                 "Non-monotonic Sum becomes a GCM Gauge",
		OTLPInputFixturePath: "testdata/fixtures/metrics/nonmonotonic_counter.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/nonmonotonic_counter_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		MetricSDKExporterOptions: []metric.Option{metric.WithFilteredResourceAttributes(metric.NoAttributes)},
	},
	{
		Name:                 "Summary becomes a GCM Cumulative for sum/count, Gauges for quantiles",
		OTLPInputFixturePath: "testdata/fixtures/metrics/summary.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/summary_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		// Summary metrics are not possible with the SDK.
		SkipForSDK: true,
	},
	{
		Name:                 "Gauge becomes a GCM Gauge",
		OTLPInputFixturePath: "testdata/fixtures/metrics/gauge.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/gauge_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.InstrumentationLibraryLabels = true
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		MetricSDKExporterOptions: []metric.Option{metric.WithFilteredResourceAttributes(metric.NoAttributes)},
	},
	{
		Name:                 "Boolean-valued Gauge metric becomes an Int Gauge",
		OTLPInputFixturePath: "testdata/fixtures/metrics/boolean_gauge.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/boolean_gauge_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		SkipForSDK: true, // Boolean valued metrics not implemented in SDK
	},
	{
		Name:                 "Gauge with Untyped label is a standard GCM Gauge without GMP",
		OTLPInputFixturePath: "testdata/fixtures/metrics/untyped_gauge.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/untyped_gauge_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		SkipForSDK: true,
	},
	{
		Name:                 "Histogram becomes a GCM Distribution",
		OTLPInputFixturePath: "testdata/fixtures/metrics/histogram.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/histogram_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.ServiceResourceLabels = false
			cfg.MetricConfig.InstrumentationLibraryLabels = true
			cfg.MetricConfig.EnableSumOfSquaredDeviation = true
		},
		MetricSDKExporterOptions: []metric.Option{
			metric.WithFilteredResourceAttributes(metric.NoAttributes),
			metric.WithSumOfSquaredDeviation(),
		},
	},
	{
		Name:                 "Exponential Histogram becomes a GCM Distribution with exponential bucketOptions",
		OTLPInputFixturePath: "testdata/fixtures/metrics/exponential_histogram.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/exponential_histogram_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		// Blocked on upstream support for exponential histograms:
		// https://github.com/open-telemetry/opentelemetry-go/issues/2966
		SkipForSDK: true,
	},
	{
		Name:                 "Metrics from the Prometheus receiver can be successfully delivered",
		OTLPInputFixturePath: "testdata/fixtures/metrics/prometheus.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/prometheus_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.EnableSumOfSquaredDeviation = true
		},
		MetricSDKExporterOptions: []metric.Option{
			metric.WithSumOfSquaredDeviation(),
		},
	},
	{
		Name:                 "Prometheus stale data point is dropped",
		OTLPInputFixturePath: "testdata/fixtures/metrics/prometheus_stale.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/prometheus_stale_expect.json",
		CompareFixturePath:   "testdata/fixtures/metrics/prometheus_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.EnableSumOfSquaredDeviation = true
		},
		SkipForSDK: true, // Stale point handling is not required for the SDK.
	},
	// Tests with special configuration options
	{
		Name:                 "Project not found return code",
		OTLPInputFixturePath: "testdata/fixtures/metrics/counter.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/counter_notfound_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.ProjectID = "notfoundproject"
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		MetricSDKExporterOptions: []metric.Option{metric.WithProjectID("notfoundproject"), metric.WithFilteredResourceAttributes(metric.NoAttributes)},
		ExpectErr:                true,
	},
	{
		Name:                 "Modified prefix unknown domain",
		OTLPInputFixturePath: "testdata/fixtures/metrics/counter.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/counter_unknown_domain_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.Prefix = "custom.googleapis.com/foobar.org"
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		MetricSDKExporterOptions: []metric.Option{
			metric.WithMetricDescriptorTypeFormatter(func(m metricdata.Metrics) string {
				return "custom.googleapis.com/foobar.org/" + m.Name
			}),
			metric.WithFilteredResourceAttributes(metric.NoAttributes),
		},
	},
	{
		Name:                 "Modified prefix workload.googleapis.com",
		OTLPInputFixturePath: "testdata/fixtures/metrics/counter.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/counter_workloadgoogleapis_prefix_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.Prefix = "workload.googleapis.com"
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		MetricSDKExporterOptions: []metric.Option{metric.WithFilteredResourceAttributes(metric.NoAttributes)},
	},
	{
		Name:                 "Batching only sends 200 timeseries per-batch",
		OTLPInputFixturePath: "testdata/fixtures/metrics/batching.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/batching_expect.json",
		// Summary metrics are not possible with the SDK.
		SkipForSDK: true,
	},
	{
		Name:                 "WithResourceFilter adds the appropriate resource attributes",
		OTLPInputFixturePath: "testdata/fixtures/metrics/with_resource_filter.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/with_resource_filter_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.ResourceFilters = []collector.ResourceFilter{
				{Prefix: "telemetry.sdk."},
			}
		},
		MetricSDKExporterOptions: []metric.Option{
			metric.WithFilteredResourceAttributes(func(kv attribute.KeyValue) bool {
				// Include the default set of promoted resource attributes
				if metric.DefaultResourceAttributesFilter(kv) {
					return true
				}
				return strings.HasPrefix(string(kv.Key), "telemetry.sdk.")
			}),
		},
	},
	{
		Name:                 "Multi-project metrics splits into multiple requests to different projects",
		OTLPInputFixturePath: "testdata/fixtures/metrics/multi_project.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/multi_project_expected.json",
		// Multi-project exporting is not supported in the SDK exporter
		SkipForSDK: true,
	},
	{
		// see https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/issues/525
		Name:                 "Metrics with only one +inf bucket can be sent",
		OTLPInputFixturePath: "testdata/fixtures/metrics/prometheus_empty_buckets.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/prometheus_empty_buckets_expected.json",
		SkipForSDK:           true,
	},
	{
		Name:                 "Gzip compression enabled",
		OTLPInputFixturePath: "testdata/fixtures/metrics/counter.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/counter_compressed_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.ClientConfig.Compression = "gzip"
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		SkipForSDK: true,
	},
	{
		Name:                 "CreateServiceTimeSeries option enabled makes CreateServiceTimeSeries calls",
		OTLPInputFixturePath: "testdata/fixtures/metrics/create_service_timeseries.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/create_service_timeseries_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.CreateServiceTimeSeries = true
		},
		// SDK exporter does not support CreateServiceTimeSeries
		SkipForSDK: true,
	},
	{
		Name:                 "Write ahead log enabled",
		OTLPInputFixturePath: "testdata/fixtures/metrics/counter.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/counter_wal_expect.json",
		CompareFixturePath:   "testdata/fixtures/metrics/counter_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			dir, _ := os.MkdirTemp("", "test-wal-")
			cfg.MetricConfig.WALConfig = &collector.WALConfig{
				Directory:  dir,
				MaxBackoff: time.Duration(1 * time.Second),
			}
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		SkipForSDK: true,
	},
	{
		Name:                 "Write ahead log enabled, basic prometheus metrics",
		OTLPInputFixturePath: "testdata/fixtures/metrics/prometheus.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/prometheus_wal_expect.json",
		CompareFixturePath:   "testdata/fixtures/metrics/prometheus_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			dir, _ := os.MkdirTemp("", "test-wal-")
			cfg.MetricConfig.WALConfig = &collector.WALConfig{
				Directory:  dir,
				MaxBackoff: time.Duration(1 * time.Second),
			}
			cfg.MetricConfig.EnableSumOfSquaredDeviation = true
		},
		SkipForSDK: true,
	},
	{
		Name:                 "Write ahead log enabled, basic Counter with unavailable return code",
		OTLPInputFixturePath: "testdata/fixtures/metrics/counter.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/counter_wal_unavailable_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.ProjectID = "unavailableproject"
			dir, _ := os.MkdirTemp("", "test-wal-")
			cfg.MetricConfig.WALConfig = &collector.WALConfig{
				Directory:  dir,
				MaxBackoff: time.Duration(2 * time.Second),
			}
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		SkipForSDK:    true,
		ExpectRetries: true,
	},
	{
		Name:                 "Write ahead log enabled, basic Counter with deadline_exceeded return code",
		OTLPInputFixturePath: "testdata/fixtures/metrics/counter.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/counter_wal_deadline_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.ProjectID = "deadline_exceededproject"
			dir, _ := os.MkdirTemp("", "test-wal-")
			cfg.MetricConfig.WALConfig = &collector.WALConfig{
				Directory:  dir,
				MaxBackoff: time.Duration(2 * time.Second),
			}
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		SkipForSDK:    true,
		ExpectRetries: true,
	},
	{
		Name:                 "Write ahead log enabled, CreateServiceTimeSeries",
		OTLPInputFixturePath: "testdata/fixtures/metrics/create_service_timeseries.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/create_service_timeseries_wal_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.CreateServiceTimeSeries = true
			dir, _ := os.MkdirTemp("", "test-wal-")
			cfg.MetricConfig.WALConfig = &collector.WALConfig{
				Directory:  dir,
				MaxBackoff: time.Duration(1 * time.Second),
			}
		},
		// SDK exporter does not support CreateServiceTimeSeries
		SkipForSDK: true,
	},
	{
		Name:                 "Custom User Agent",
		OTLPInputFixturePath: "testdata/fixtures/metrics/counter.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/counter_user_agent_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.UserAgent = "custom-user-agent"
		},
		MetricSDKExporterOptions: []metric.Option{
			metric.WithMonitoringClientOptions(option.WithUserAgent("custom-user-agent")),
		},
	},
	// Tests for the GMP exporter
	{
		Name:                 "[GMP] prometheus receiver metrics",
		OTLPInputFixturePath: "testdata/fixtures/metrics/google_managed_prometheus.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/google_managed_prometheus_expect.json",
		ConfigureCollector:   configureGMPCollector,
		// prometheus_target is not supported by the SDK
		SkipForSDK: true,
	},
	{
		Name:                 "[GMP] Gauge becomes a GCM Gauge with /gauge suffix",
		OTLPInputFixturePath: "testdata/fixtures/metrics/gauge.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/gauge_gmp_expect.json",
		ConfigureCollector:   configureGMPCollector,
		// prometheus_target is not supported by the SDK
		SkipForSDK: true,
	},
	{
		Name:                 "[GMP] Untyped Gauge becomes a GCM Gauge and a Cumulative with /unknown and /unknown:counter suffixes",
		OTLPInputFixturePath: "testdata/fixtures/metrics/untyped_gauge.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/untyped_gauge_gmp_expect.json",
		ConfigureCollector:   configureGMPCollector,
		// prometheus_target is not supported by the SDK
		SkipForSDK: true,
	},
	{
		Name:                 "[GMP] Sum becomes a GCM Cumulative with /counter suffix",
		OTLPInputFixturePath: "testdata/fixtures/metrics/counter.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/counter_gmp_expect.json",
		ConfigureCollector:   configureGMPCollector,
		// prometheus_target is not supported by the SDK
		SkipForSDK: true,
	},
	{
		Name:                 "[GMP] Delta Sum becomes a GCM Cumulative with a /counter suffix",
		OTLPInputFixturePath: "testdata/fixtures/metrics/delta_counter.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/delta_counter_gmp_expect.json",
		ConfigureCollector:   configureGMPCollector,
		// prometheus_target is not supported by the SDK
		SkipForSDK: true,
	},
	{
		Name:                 "[GMP] Non-Monotonic Sum becomes a GCM Gauge with a /gauge suffix",
		OTLPInputFixturePath: "testdata/fixtures/metrics/nonmonotonic_counter.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/nonmonotonic_counter_gmp_expect.json",
		ConfigureCollector:   configureGMPCollector,
		// prometheus_target is not supported by the SDK
		SkipForSDK: true,
	},
	{
		Name:                 "[GMP] Histogram becomes a GCM Histogram with a /histogram suffix",
		OTLPInputFixturePath: "testdata/fixtures/metrics/histogram.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/histogram_gmp_expect.json",
		ConfigureCollector:   configureGMPCollector,
		// prometheus_target is not supported by the SDK
		SkipForSDK: true,
	},
	{
		Name:                 "[GMP] Exponential Histogram becomes a GCM Distribution with exponential bucketOptions and a /histogram suffix",
		OTLPInputFixturePath: "testdata/fixtures/metrics/exponential_histogram.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/exponential_histogram_gmp_expect.json",
		ConfigureCollector:   configureGMPCollector,
		// prometheus_target is not supported by the SDK
		SkipForSDK: true,
	},
	{
		Name:                 "[GMP] Summary becomes a GCM Cumulative for sum/count, Gauges for quantiles",
		OTLPInputFixturePath: "testdata/fixtures/metrics/summary.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/summary_gmp_expect.json",
		ConfigureCollector:   configureGMPCollector,
		// prometheus_target is not supported by the SDK
		SkipForSDK: true,
	},
	// Tests for specific distributions of the collector
	{
		Name:                 "Ops Agent Self-Reported metrics",
		OTLPInputFixturePath: "testdata/fixtures/metrics/ops_agent_self_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/ops_agent_self_metrics_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			// Metric descriptors should not be created under agent.googleapis.com
			cfg.MetricConfig.SkipCreateMetricDescriptor = true
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		// We don't support disabling metric descriptor creation for the SDK exporter
		SkipForSDK: true,
	},
	{
		Name:                 "Ops Agent Host Metrics",
		OTLPInputFixturePath: "testdata/fixtures/metrics/ops_agent_host_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/ops_agent_host_metrics_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			// Metric descriptors should not be created under agent.googleapis.com
			cfg.MetricConfig.SkipCreateMetricDescriptor = true
		},
		// We don't support disabling metric descriptor creation for the SDK exporter
		SkipForSDK: true,
	},
	{
		Name:                 "GKE Workload Metrics",
		OTLPInputFixturePath: "testdata/fixtures/metrics/workload_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/workload_metrics_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.Prefix = "workload.googleapis.com/"
			cfg.MetricConfig.SkipCreateMetricDescriptor = true
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		// We don't support disabling metric descriptor creation for the SDK exporter
		SkipForSDK: true,
	},
	{
		Name:                 "GKE Metrics Agent",
		OTLPInputFixturePath: "testdata/fixtures/metrics/gke_metrics_agent.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/gke_metrics_agent_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.CreateServiceTimeSeries = true
		},
		// SDK exporter does not support CreateServiceTimeSeries
		SkipForSDK: true,
	},
	{
		Name:                 "GKE Control Plane Metrics Agent",
		OTLPInputFixturePath: "testdata/fixtures/metrics/gke_control_plane.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/gke_control_plane_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.CreateServiceTimeSeries = true
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		// SDK exporter does not support CreateServiceTimeSeries
		SkipForSDK: true,
	},
	{
		Name:                 "BMS Ops Agent Host Metrics",
		OTLPInputFixturePath: "testdata/fixtures/metrics/bms_ops_agent_host_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/bms_ops_agent_host_metrics_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			// Metric descriptors should not be created under agent.googleapis.com
			cfg.MetricConfig.SkipCreateMetricDescriptor = true
		},
		// We don't support disabling metric descriptor creation for the SDK exporter
		SkipForSDK: true,
	},
	// TODO: Add integration tests for workload.googleapis.com metrics from the ops agent
}

func configureGMPCollector(cfg *collector.Config) {
	//nolint:errcheck
	featuregate.GlobalRegistry().Set("pkg.translator.prometheus.NormalizeName", true)
	cfg.MetricConfig.Prefix = "prometheus.googleapis.com/"
	cfg.MetricConfig.SkipCreateMetricDescriptor = true
	gmpConfig := googlemanagedprometheus.DefaultConfig()
	cfg.MetricConfig.GetMetricName = gmpConfig.GetMetricName
	cfg.MetricConfig.MapMonitoredResource = gmpConfig.MapToPrometheusTarget
	cfg.MetricConfig.ExtraMetrics = gmpConfig.ExtraMetrics
	cfg.MetricConfig.InstrumentationLibraryLabels = false
	cfg.MetricConfig.ServiceResourceLabels = false
	cfg.MetricConfig.EnableSumOfSquaredDeviation = true
}
