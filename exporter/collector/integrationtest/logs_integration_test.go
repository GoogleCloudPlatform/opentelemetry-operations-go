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

//go:build integrationtest
// +build integrationtest

package integrationtest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/integrationtest/testcases"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
)

func createLogsExporter(
	ctx context.Context,
	t *testing.T,
	test *testcases.TestCase,
) *collector.LogsExporter {
	logger, _ := zap.NewProduction()
	cfg := test.CreateLogConfig()
	// For sending to a real project, set the project ID from an env var.
	cfg.ProjectID = os.Getenv("PROJECT_ID")

	var duration time.Duration
	exporter, err := collector.NewGoogleCloudLogsExporter(
		ctx,
		cfg,
		logger,
		otel.GetMeterProvider(),
		integrationTestBuildInfo,
		duration,
	)
	require.NoError(t, err)
	err = exporter.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	exporter.ConfigureExporter(test.ConfigureLogsExporter)
	t.Log("Collector logs exporter started")
	return exporter
}

func TestIntegrationLogs(t *testing.T) {
	ctx := context.Background()
	endTime := time.Now()
	startTime := endTime.Add(-time.Second)

	for _, test := range testcases.LogsTestCases {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			logs := test.LoadOTLPLogsInput(t, startTime)
			setSecondProjectInLogs(t, logs)
			exporter := createLogsExporter(ctx, t, &test)
			defer func() { require.NoError(t, exporter.Shutdown(ctx)) }()

			require.NoError(
				t,
				exporter.PushLogs(ctx, logs),
				"Failed to export logs",
			)
		})
	}
}

// setSecondProjectInLogs overwrites the gcp.project.id resource attribute
// with the contents of the SECOND_PROJECT_ID environment variable. This makes
// the integration test send those logs with a gcp.project.id attribute to the
// specified second project.
func setSecondProjectInLogs(t *testing.T, logs plog.Logs) {
	secondProject := os.Getenv(testcases.SecondProjectEnv)
	require.NotEmpty(t, secondProject, "set the SECOND_PROJECT_ID environment to run this test")
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		if project, found := logs.ResourceLogs().At(i).Resource().Attributes().Get(resourcemapping.ProjectIDAttributeKey); found {
			project.SetStr(secondProject)
		}
	}
}
