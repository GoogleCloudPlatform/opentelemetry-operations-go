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

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric"
)

var MetricsTestCases = []TestCase{
	{
		Name:                 "Basic Counter",
		OTLPInputFixturePath: "testdata/fixtures/metrics/basic_counter_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/basic_counter_metrics_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.InstrumentationLibraryLabels = true
		},
	},
	{
		Name:                 "Basic Counter with not found return code",
		OTLPInputFixturePath: "testdata/fixtures/metrics/basic_counter_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/basic_counter_metrics_notfound_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.ProjectID = "notfoundproject"
		},
		MetricSDKExporterOptions: []metric.Option{metric.WithProjectID("notfoundproject")},
		ExpectErr:                true,
	},
	{
		Name:                 "Basic Prometheus metrics",
		OTLPInputFixturePath: "testdata/fixtures/metrics/basic_prometheus_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/basic_prometheus_metrics_expect.json",
	},
	{
		Name:                 "Basic Prometheus metrics with stale data point",
		OTLPInputFixturePath: "testdata/fixtures/metrics/basic_prometheus_metrics_stale.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/basic_prometheus_metrics_stale_expect.json",
		CompareFixturePath:   "testdata/fixtures/metrics/basic_prometheus_metrics_expect.json",
		SkipForSDK:           true,
	},
	{
		Name:                 "Modified prefix unknown domain",
		OTLPInputFixturePath: "testdata/fixtures/metrics/basic_counter_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/unknown_domain_metrics_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.Prefix = "custom.googleapis.com/foobar.org"
		},
		MetricSDKExporterOptions: []metric.Option{metric.WithMetricDescriptorTypeFormatter(func(m metricdata.Metrics) string {
			return "custom.googleapis.com/foobar.org/" + m.Name
		})},
	},
	{
		Name:                 "Modified prefix workload.googleapis.com",
		OTLPInputFixturePath: "testdata/fixtures/metrics/basic_counter_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/workloadgoogleapis_prefix_metrics_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.Prefix = "workload.googleapis.com"
		},
	},
	{
		Name:                 "Delta Counter",
		OTLPInputFixturePath: "testdata/fixtures/metrics/delta_counter_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/delta_counter_metrics_expect.json",
	},
	{
		Name:                 "Non-monotonic Counter",
		OTLPInputFixturePath: "testdata/fixtures/metrics/nonmonotonic_counter_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/nonmonotonic_counter_metrics_expect.json",
	},
	{
		Name:                 "Summary",
		OTLPInputFixturePath: "testdata/fixtures/metrics/summary_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/summary_metrics_expect.json",
		// Summary metrics are not possible with the SDK.
		SkipForSDK: true,
	},
	{
		Name:                 "Batching",
		OTLPInputFixturePath: "testdata/fixtures/metrics/batching_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/batching_metrics_expect.json",
		// Summary metrics are not possible with the SDK.
		SkipForSDK: true,
	},
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
		Name:                 "Google Managed Prometheus",
		OTLPInputFixturePath: "testdata/fixtures/metrics/google_managed_prometheus.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/google_managed_prometheus_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.Prefix = "prometheus.googleapis.com/"
			cfg.MetricConfig.SkipCreateMetricDescriptor = true
			cfg.MetricConfig.GetMetricName = googlemanagedprometheus.GetMetricName
			cfg.MetricConfig.MapMonitoredResource = googlemanagedprometheus.MapToPrometheusTarget
			cfg.MetricConfig.ExtraMetrics = func(m pmetric.Metrics) pmetric.ResourceMetricsSlice {
				googlemanagedprometheus.AddScopeInfoMetric(m)
				googlemanagedprometheus.AddTargetInfoMetric(m)
				return m.ResourceMetrics()
			}
			cfg.MetricConfig.InstrumentationLibraryLabels = false
			cfg.MetricConfig.ServiceResourceLabels = false
			cfg.MetricConfig.EnableSumOfSquaredDeviation = true
		},
		// prometheus_target is not supported by the SDK
		SkipForSDK: true,
	},
	{
		Name:                 "GKE Metrics Agent",
		OTLPInputFixturePath: "testdata/fixtures/metrics/gke_metrics_agent_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/gke_metrics_agent_metrics_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.CreateServiceTimeSeries = true
		},
		// SDK exporter does not support CreateServiceTimeSeries
		SkipForSDK: true,
	},
	{
		Name:                 "GKE Control Plane Metrics Agent",
		OTLPInputFixturePath: "testdata/fixtures/metrics/gke_control_plane_metrics_agent_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/gke_control_plane_metrics_agent_metrics_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.CreateServiceTimeSeries = true
			cfg.MetricConfig.ServiceResourceLabels = false
		},
		// SDK exporter does not support CreateServiceTimeSeries
		SkipForSDK: true,
	},
	{
		Name:                 "Exponential Histogram",
		OTLPInputFixturePath: "testdata/fixtures/metrics/exponential_histogram_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/exponential_histogram_metrics_expect.json",
		// Blocked on upstream support for exponential histograms:
		// https://github.com/open-telemetry/opentelemetry-go/issues/2966
		SkipForSDK: true,
	},
	{
		Name:                 "CreateServiceTimeSeries",
		OTLPInputFixturePath: "testdata/fixtures/metrics/create_service_timeseries_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/create_service_timeseries_metrics_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.CreateServiceTimeSeries = true
		},
		// SDK exporter does not support CreateServiceTimeSeries
		SkipForSDK: true,
	},
	{
		Name:                 "WithResourceFilter",
		OTLPInputFixturePath: "testdata/fixtures/metrics/with_resource_filter_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/with_resource_filter_metrics_expect.json",
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
		Name:                 "Multi-project metrics",
		OTLPInputFixturePath: "testdata/fixtures/metrics/metrics_multi_project.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/metrics_multi_project_expected.json",
		// Multi-project exporting is not supported in the SDK exporter
		SkipForSDK: true,
	},
	{
		// see https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/issues/525
		Name:                 "Metrics with only one +inf bucket",
		OTLPInputFixturePath: "testdata/fixtures/metrics/prometheus_empty_buckets.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/prometheus_empty_buckets_expected.json",
		SkipForSDK:           true,
	},
	{
		Name:                 "Basic Counter with gzip compression",
		OTLPInputFixturePath: "testdata/fixtures/metrics/basic_counter_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/basic_counter_metrics_compressed_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.ClientConfig.Compression = "gzip"
		},
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
	{
		Name:                 "Write ahead log enabled",
		OTLPInputFixturePath: "testdata/fixtures/metrics/basic_counter_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/basic_counter_metrics_wal_expect.json",
		CompareFixturePath:   "testdata/fixtures/metrics/basic_counter_metrics_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			dir, _ := os.MkdirTemp("", "test-wal-")
			cfg.MetricConfig.WALConfig = &collector.WALConfig{
				Directory:  dir,
				MaxBackoff: time.Duration(1 * time.Second),
			}
		},
		SkipForSDK: true,
	},
	{
		Name:                 "Write ahead log enabled, basic prometheus metrics",
		OTLPInputFixturePath: "testdata/fixtures/metrics/basic_prometheus_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/basic_prometheus_metrics_wal_expect.json",
		CompareFixturePath:   "testdata/fixtures/metrics/basic_prometheus_metrics_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			dir, _ := os.MkdirTemp("", "test-wal-")
			cfg.MetricConfig.WALConfig = &collector.WALConfig{
				Directory:  dir,
				MaxBackoff: time.Duration(1 * time.Second),
			}
		},
		SkipForSDK: true,
	},
	{
		Name:                 "Write ahead log enabled, basic Counter with unavailable return code",
		OTLPInputFixturePath: "testdata/fixtures/metrics/basic_counter_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/basic_counter_metrics_wal_unavailable_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.ProjectID = "unavailableproject"
			dir, _ := os.MkdirTemp("", "test-wal-")
			cfg.MetricConfig.WALConfig = &collector.WALConfig{
				Directory:  dir,
				MaxBackoff: time.Duration(2 * time.Second),
			}
		},
		SkipForSDK:    true,
		ExpectRetries: true,
	},
	{
		Name:                 "Write ahead log enabled, basic Counter with deadline_exceeded return code",
		OTLPInputFixturePath: "testdata/fixtures/metrics/basic_counter_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/basic_counter_metrics_wal_deadline_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.ProjectID = "deadline_exceededproject"
			dir, _ := os.MkdirTemp("", "test-wal-")
			cfg.MetricConfig.WALConfig = &collector.WALConfig{
				Directory:  dir,
				MaxBackoff: time.Duration(2 * time.Second),
			}
		},
		SkipForSDK:    true,
		ExpectRetries: true,
	},
	{
		Name:                 "Write ahead log enabled, CreateServiceTimeSeries",
		OTLPInputFixturePath: "testdata/fixtures/metrics/create_service_timeseries_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/create_service_timeseries_metrics_wal_expect.json",
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
	// TODO: Add integration tests for workload.googleapis.com metrics from the ops agent
}
