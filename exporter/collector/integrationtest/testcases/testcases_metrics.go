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
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus"
)

var MetricsTestCases = []TestCase{
	{
		Name:                 "Basic Counter",
		OTLPInputFixturePath: "testdata/fixtures/metrics/basic_counter_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/basic_counter_metrics_expect.json",
		SkipForSDK:           true,
	},
	{
		Name:                 "Basic Counter with not found return code",
		OTLPInputFixturePath: "testdata/fixtures/metrics/basic_counter_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/basic_counter_metrics_notfound_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.ProjectID = "notfoundproject"
		},
		ExpectErr:  true,
		SkipForSDK: true,
	},
	{
		Name:                 "Basic Prometheus metrics",
		OTLPInputFixturePath: "testdata/fixtures/metrics/basic_prometheus_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/basic_prometheus_metrics_expect.json",
		SkipForSDK:           true,
	},
	{
		Name:                 "Modified prefix unknown domain",
		OTLPInputFixturePath: "testdata/fixtures/metrics/basic_counter_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/unknown_domain_metrics_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.Prefix = "custom.googleapis.com/foobar.org"
		},
		SkipForSDK: true,
	},
	{
		Name:                 "Modified prefix workload.googleapis.com",
		OTLPInputFixturePath: "testdata/fixtures/metrics/basic_counter_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/workloadgoogleapis_prefix_metrics_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.Prefix = "workload.googleapis.com"
		},
		SkipForSDK: true,
	},
	{
		Name:                 "Delta Counter",
		OTLPInputFixturePath: "testdata/fixtures/metrics/delta_counter_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/delta_counter_metrics_expect.json",
		SkipForSDK:           true,
	},
	{
		Name:                 "Non-monotonic Counter",
		OTLPInputFixturePath: "testdata/fixtures/metrics/nonmonotonic_counter_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/nonmonotonic_counter_metrics_expect.json",
		SkipForSDK:           true,
	},
	{
		Name:                 "Summary",
		OTLPInputFixturePath: "testdata/fixtures/metrics/summary_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/summary_metrics_expect.json",
		SkipForSDK:           true,
	},
	{
		Name:                 "Batching",
		OTLPInputFixturePath: "testdata/fixtures/metrics/batching_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/batching_metrics_expect.json",
		SkipForSDK:           true,
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
			cfg.MetricConfig.InstrumentationLibraryLabels = false
			cfg.MetricConfig.ServiceResourceLabels = false
			cfg.MetricConfig.EnableSumOfSquaredDeviation = true
		},
		SkipForSDK: true,
	},
	{
		Name:                 "GKE Metrics Agent",
		OTLPInputFixturePath: "testdata/fixtures/metrics/gke_metrics_agent_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/gke_metrics_agent_metrics_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.CreateServiceTimeSeries = true
		},
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
		SkipForSDK: true,
	},
	{
		Name:                 "Exponential Histogram",
		OTLPInputFixturePath: "testdata/fixtures/metrics/exponential_histogram_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/exponential_histogram_metrics_expect.json",
		SkipForSDK:           true,
	},
	{
		Name:                 "CreateServiceTimeSeries",
		OTLPInputFixturePath: "testdata/fixtures/metrics/create_service_timeseries_metrics.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/create_service_timeseries_metrics_expect.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.MetricConfig.CreateServiceTimeSeries = true
		},
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
		SkipForSDK: true,
	},
	{
		Name:                 "Multi-project metrics",
		OTLPInputFixturePath: "testdata/fixtures/metrics/metrics_multi_project.json",
		ExpectFixturePath:    "testdata/fixtures/metrics/metrics_multi_project_expected.json",
		SkipForSDK:           true,
	},
	// TODO: Add integration tests for workload.googleapis.com metrics from the ops agent
}
