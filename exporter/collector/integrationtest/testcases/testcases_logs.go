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
			MaxRequestSize: 500,
		},
	},
}
