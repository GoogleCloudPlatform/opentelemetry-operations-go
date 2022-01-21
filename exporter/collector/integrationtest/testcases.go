// Copyright 2021 Google LLC
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

package integrationtest

import "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"

var (
	TestCases = []MetricsTestCase{
		{
			Name:                 "Basic Counter",
			OTLPInputFixturePath: "testdata/fixtures/basic_counter_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/basic_counter_metrics_expect.json",
		},
		{
			Name:                 "Modified prefix unknown domain",
			OTLPInputFixturePath: "testdata/fixtures/basic_counter_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/unknown_domain_metrics_expect.json",
			Configure: func(cfg *collector.Config) {
				cfg.MetricConfig.Prefix = "custom.googleapis.com/foobar.org"
			},
		},
		{
			Name:                 "Modified prefix workload.googleapis.com",
			OTLPInputFixturePath: "testdata/fixtures/basic_counter_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/workloadgoogleapis_prefix_metrics_expect.json",
			Configure: func(cfg *collector.Config) {
				cfg.MetricConfig.Prefix = "workload.googleapis.com"
			},
		},
		{
			Name:                 "Delta Counter",
			OTLPInputFixturePath: "testdata/fixtures/delta_counter_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/delta_counter_metrics_expect.json",
		},
		{
			Name:                 "Non-monotonic Counter",
			OTLPInputFixturePath: "testdata/fixtures/nonmonotonic_counter_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/nonmonotonic_counter_metrics_expect.json",
		},
		{
			Name:                 "Summary",
			OTLPInputFixturePath: "testdata/fixtures/summary_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/summary_metrics_expect.json",
		},
		{
			Name:                 "Ops Agent Self-Reported metrics",
			OTLPInputFixturePath: "testdata/fixtures/ops_agent_self_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/ops_agent_self_metrics_expect.json",
			Configure: func(cfg *collector.Config) {
				// Previous exporter did NOT export metric descriptors.
				// TODO: Add a new test that also checks metric descriptors.
				cfg.MetricConfig.SkipCreateMetricDescriptor = true
				cfg.MetricConfig.IncludeServiceResourceAttributes = false
			},
		},
		{
			Name:                 "Ops Agent Host Metrics",
			OTLPInputFixturePath: "testdata/fixtures/ops_agent_host_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/ops_agent_host_metrics_expect.json",
			Configure: func(cfg *collector.Config) {
				// Previous exporter did NOT export metric descriptors.
				// TODO: Add a new test that also checks metric descriptors.
				cfg.MetricConfig.SkipCreateMetricDescriptor = true
			},
		},
		{
			Name:                 "GKE Workload Metrics",
			OTLPInputFixturePath: "testdata/fixtures/workload_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/workload_metrics_expect.json",
			Skip:                 true,
			Configure: func(cfg *collector.Config) {
				cfg.MetricConfig.Prefix = "workload.googleapis.com/"
			},
		},
		{
			Name:                 "GKE Metrics Agent",
			OTLPInputFixturePath: "testdata/fixtures/gke_metrics_agent_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/gke_metrics_agent_metrics_expect.json",
			Configure: func(cfg *collector.Config) {
				cfg.MetricConfig.CreateServiceTimeSeries = true
			},
		},
		{
			Name:                 "GKE Control Plane Metrics Agent",
			OTLPInputFixturePath: "testdata/fixtures/gke_control_plane_metrics_agent_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/gke_control_plane_metrics_agent_metrics_expect.json",
			Configure: func(cfg *collector.Config) {
				cfg.MetricConfig.CreateServiceTimeSeries = true
				cfg.MetricConfig.IncludeServiceResourceAttributes = false
			},
		},
		{
			Name:                 "Exponential Histogram",
			OTLPInputFixturePath: "testdata/fixtures/exponential_histogram_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/exponential_histogram_metrics_expect.json",
		},
		{
			Name:                 "CreateServiceTimeSeries",
			OTLPInputFixturePath: "testdata/fixtures/create_service_timeseries_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/create_service_timeseries_metrics_expect.json",
			Configure: func(cfg *collector.Config) {
				cfg.MetricConfig.CreateServiceTimeSeries = true
			},
		},
		{
			Name:                 "WithResourceFilter",
			OTLPInputFixturePath: "testdata/fixtures/with_resource_filter_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/with_resource_filter_metrics_expect.json",
			Configure: func(cfg *collector.Config) {
				cfg.MetricConfig.ResourceFilter = `telemetry\.sdk\..*`
			},
		},
	}
)
