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

var (
	TestCases = []MetricsTestCase{
		{
			Name:                 "Basic Counter",
			OTLPInputFixturePath: "testdata/fixtures/basic_counter_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/basic_counter_metrics_expect.json",
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
		},
		{
			Name:                 "Ops Agent Host Metrics",
			OTLPInputFixturePath: "testdata/fixtures/ops_agent_host_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/ops_agent_host_metrics_expect.json",
		},
		{
			Name:                 "GKE Workload Metrics",
			OTLPInputFixturePath: "testdata/fixtures/workload_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/workload_metrics_expect.json",
		},
		{
			Name:                 "GKE Metrics Agent",
			OTLPInputFixturePath: "testdata/fixtures/gke_metrics_agent_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/gke_metrics_agent_metrics_expect.json",
		},
		{
			Name:                 "GKE Control Plane Metrics Agent",
			OTLPInputFixturePath: "testdata/fixtures/gke_control_plane_metrics_agent_metrics.json",
			ExpectFixturePath:    "testdata/fixtures/gke_control_plane_metrics_agent_metrics_expect.json",
		},
	}
)
