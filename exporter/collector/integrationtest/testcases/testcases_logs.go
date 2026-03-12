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
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/logsutil"
)

var LogsTestCases = []TestCase{
	{
		Name:                 "Apache access log with HTTPRequest",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_apache_access.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_apache_access_expected.json",
	},
	{
		Name:                 "Apache error log with severity",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_apache_error.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_apache_error_expected.json",
	},
	{
		Name:                 "Apache error log (text payload) with severity converted to Error Reporting type",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_apache_text_error.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_apache_text_error_reporting_expected.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.LogConfig.ErrorReportingType = true
		},
	},
	{
		Name:                 "Apache error log (json payload) with severity converted to Error Reporting type",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_apache_error.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_apache_json_error_reporting_expected.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.LogConfig.ErrorReportingType = true
		},
	},
	{
		Name:                 "Multi-project logs",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_multi_project.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_multi_project_expected.json",
	},
	{
		Name:                 "Multi-project logs with destination_project_quota enabled",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_multi_project.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_multi_project_destination_quota_expected.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.DestinationProjectQuota = true
		},
	},
	{
		Name:                 "Logs with scope information",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_apache_error_scope.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_apache_error_scope_expected.json",
	},
	{
		Name:                 "Logs with trace/span info",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_span_trace_id.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_span_trace_id_expected.json",
	},
	{
		Name:                 "Logs with additional resource attributes",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_apache_access_resource_attributes.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_apache_access_resource_attributes_expected.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.LogConfig.ResourceFilters = []collector.ResourceFilter{
				{Prefix: "custom."},
			}
		},
	},
	{
		Name:                 "Logs with multiple batches",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_apache_access.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_apache_access_batches_expected.json",
		ConfigureLogsExporter: &logsutil.ExporterConfig{
			MaxEntrySize:   50,
			MaxRequestSize: 550,
		},
	},
	{
		Name:                 "Logs custom user-agent",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_span_trace_id.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_user_agent_expected.json",
		ConfigureCollector: func(cfg *collector.Config) {
			cfg.UserAgent = "custom-user-agent {{version}}"
		},
	},
	{
		Name:                 "k8s_container monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_k8s_container.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_k8s_container_expected.json",
	},
	{
		Name:                 "k8s_pod monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_k8s_pod.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_k8s_pod_expected.json",
	},
	{
		Name:                 "k8s_node monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_k8s_node.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_k8s_node_expected.json",
	},
	{
		Name:                 "k8s_cluster monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_k8s_cluster.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_k8s_cluster_expected.json",
	},
	{
		Name:                 "gce_instance monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_gce_instance.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_gce_instance_expected.json",
	},
	{
		Name:                 "gae_instance monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_gae_instance.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_gae_instance_expected.json",
	},
	{
		Name:                 "aws_ec2_instance monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_aws_ec2_instance.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_aws_ec2_instance_expected.json",
	},
	{
		Name:                 "baremetalsolution.googleapis.com/Instance monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_bms.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_bms_expected.json",
	},
	{
		Name:                 "generic_task cloud run monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_generic_task_cloud_run.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_generic_task_cloud_run_expected.json",
	},
	{
		Name:                 "generic_task cloud functions monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_generic_task_cloud_functions.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_generic_task_cloud_functions_expected.json",
	},
	{
		Name:                 "generic_task service monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_generic_task_service.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_generic_task_service_expected.json",
	},
	{
		Name:                 "generic_node monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_generic_node.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_generic_node_expected.json",
	},
	{
		Name:                 "source_location",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_source_location.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_source_location_expected.json",
	},
}

var OTLPLogsTestCases = []TestCase{
	{
		Name:                 "Apache access log with HTTPRequest",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_apache_access.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_apache_access_expected.json",
	},
	{
		Name:                 "Apache error log with severity",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_apache_error.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_apache_error_expected.json",
	},
	{
		Name:                 "Multi-project logs",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_multi_project.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_multi_project_expected.json",
	},
	{
		Name:                 "Logs with scope information",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_apache_error_scope.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_apache_error_scope_expected.json",
	},
	{
		Name:                 "Logs with trace/span info",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_span_trace_id.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_span_trace_id_expected.json",
	},
	{
		Name:                 "k8s_container monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_k8s_container.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_k8s_container_expected.json",
	},
	{
		Name:                 "k8s_pod monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_k8s_pod.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_k8s_pod_expected.json",
	},
	{
		Name:                 "k8s_node monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_k8s_node.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_k8s_node_expected.json",
	},
	{
		Name:                 "k8s_cluster monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_k8s_cluster.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_k8s_cluster_expected.json",
	},
	{
		Name:                 "gce_instance monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_gce_instance.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_gce_instance_expected.json",
	},
	{
		Name:                 "gae_instance monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_gae_instance.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_gae_instance_expected.json",
	},
	{
		Name:                 "aws_ec2_instance monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_aws_ec2_instance.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_aws_ec2_instance_expected.json",
	},
	{
		Name:                 "baremetalsolution.googleapis.com/Instance monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_bms.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_bms_expected.json",
	},
	{
		Name:                 "generic_task cloud run monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_generic_task_cloud_run.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_generic_task_cloud_run_expected.json",
	},
	{
		Name:                 "generic_task cloud functions monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_generic_task_cloud_functions.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_generic_task_cloud_functions_expected.json",
	},
	{
		Name:                 "generic_task service monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_generic_task_service.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_generic_task_service_expected.json",
	},
	{
		Name:                 "generic_node monitored resource",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_generic_node.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_generic_node_expected.json",
	},
	{
		Name:                 "source_location",
		OTLPInputFixturePath: "testdata/fixtures/logs/logs_source_location.json",
		ExpectFixturePath:    "testdata/fixtures/logs/logs_source_location_expected.json",
	},
}
