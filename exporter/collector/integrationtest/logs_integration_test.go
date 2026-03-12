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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/integrationtest/testcases"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	googleclientauthextensionlib "github.com/GoogleCloudPlatform/opentelemetry-operations-go/extension/googleclientauthextension"
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

	set := testcases.NewTestExporterSettings(logger, noop.NewMeterProvider())
	testcases.SetTestUserAgent(&cfg, set.BuildInfo)
	var duration time.Duration
	exporter, err := collector.NewGoogleCloudLogsExporter(
		ctx,
		cfg,
		set,
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

func setProjectInLogs(t *testing.T, logs plog.Logs) {
	project := os.Getenv(testcases.ProejctEnv)
	require.NotEmpty(t, project, "set the PROJECT_ID environment to run this test")
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		// Don't override the "second project ID" set above.
		if _, found := logs.ResourceLogs().At(i).Resource().Attributes().Get(resourcemapping.ProjectIDAttributeKey); !found {
			logs.ResourceLogs().At(i).Resource().Attributes().PutStr(resourcemapping.ProjectIDAttributeKey, project)
		}
	}
}

func TestIntegrationLogsOTLPGRPC(t *testing.T) {
	ctx := context.Background()
	endTime := time.Now()
	startTime := endTime.Add(-time.Second)

	for _, test := range testcases.OTLPLogsTestCases {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			logs := test.LoadOTLPLogsInput(t, startTime)
			setSecondProjectInLogs(t, logs)
			setProjectInLogs(t, logs)
			exporter := createOTLPGRPCLogsExporter(ctx, t, &test)
			defer func() { require.NoError(t, exporter.Shutdown(ctx)) }()

			require.NoError(
				t,
				exporter.ConsumeLogs(ctx, logs),
				"Failed to export logs",
			)
		})
	}
}

func createOTLPGRPCLogsExporter(
	ctx context.Context,
	t *testing.T,
	test *testcases.TestCase,
) exporter.Logs {
	// Make sure we don't have any partial errors.
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(
		zap.Hooks(func(e zapcore.Entry) error {
			if e.Level == zap.ErrorLevel || e.Level == zap.WarnLevel {
				t.Fatalf("Error or warning log entry observed")
			}
			return nil
		}),
	))
	set := testcases.NewTestExporterSettings(logger, noop.NewMeterProvider())
	set.ID = component.MustNewID("otlp")
	factory := otlpexporter.NewFactory()
	cfg := factory.CreateDefaultConfig().(*otlpexporter.Config)
	cfg.ClientConfig.Endpoint = "https://telemetry.googleapis.com:443"
	cfg.ClientConfig.Auth = configoptional.Some(configauth.Config{AuthenticatorID: googleclientauthExtensionID})
	// Make export synchronous so that the test fails when requests fail.
	cfg.QueueConfig = exporterhelper.QueueBatchConfig{WaitForResult: true}
	exporter, err := factory.CreateLogs(ctx, set, cfg)
	require.NoError(t, err)
	err = exporter.Start(ctx, newHost(ctx, t))
	require.NoError(t, err)
	t.Log("Collector OTLP http logs exporter started")
	return exporter
}

func TestIntegrationLogsOTLPHTTP(t *testing.T) {
	ctx := context.Background()
	endTime := time.Now()
	startTime := endTime.Add(-time.Second)

	for _, test := range testcases.OTLPLogsTestCases {
		test := test

		for _, encoding := range []otlphttpexporter.EncodingType{
			otlphttpexporter.EncodingProto,
			otlphttpexporter.EncodingJSON,
		} {
			t.Run(fmt.Sprintf("%s/%v", test.Name, encoding), func(t *testing.T) {
				logs := test.LoadOTLPLogsInput(t, startTime)
				setSecondProjectInLogs(t, logs)
				setProjectInLogs(t, logs)
				exporter := createOTLPHTTPLogsExporter(ctx, t, encoding, &test)
				defer func() { require.NoError(t, exporter.Shutdown(ctx)) }()

				require.NoError(
					t,
					exporter.ConsumeLogs(ctx, logs),
					"Failed to export logs",
				)
			})
		}
	}
}

func createOTLPHTTPLogsExporter(
	ctx context.Context,
	t *testing.T,
	encoding otlphttpexporter.EncodingType,
	test *testcases.TestCase,
) exporter.Logs {
	// Make sure we don't have any partial errors.
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(
		zap.Hooks(func(e zapcore.Entry) error {
			if e.Level == zap.ErrorLevel || e.Level == zap.WarnLevel {
				t.Fatalf("Error or warning log entry observed")
			}
			return nil
		}),
	))
	set := testcases.NewTestExporterSettings(logger, noop.NewMeterProvider())
	set.ID = component.MustNewID("otlphttp")
	factory := otlphttpexporter.NewFactory()
	cfg := factory.CreateDefaultConfig().(*otlphttpexporter.Config)
	cfg.Encoding = encoding
	cfg.ClientConfig.Endpoint = "https://telemetry.googleapis.com"
	cfg.ClientConfig.Auth = configoptional.Some(configauth.Config{AuthenticatorID: googleclientauthExtensionID})
	// Make export synchronous so that the test fails when requests fail.
	cfg.QueueConfig = exporterhelper.QueueBatchConfig{WaitForResult: true}
	exporter, err := factory.CreateLogs(ctx, set, cfg)
	require.NoError(t, err)
	err = exporter.Start(ctx, newHost(ctx, t))
	require.NoError(t, err)
	t.Log("Collector OTLP http logs exporter started")
	return exporter
}

var googleclientauthExtensionID = component.MustNewID("googleclientauth")

func newHost(ctx context.Context, t *testing.T) component.Host {
	logger := zaptest.NewLogger(t)
	factory := googleclientauthextension.NewFactory()
	set := testcases.NewTestExtensionSettings(logger, noop.NewMeterProvider())
	set.ID = googleclientauthExtensionID
	cfg := factory.CreateDefaultConfig().(*googleclientauthextension.Config)
	cfg.Config = googleclientauthextensionlib.Config{
		// For sending to a real project, set the project ID from an env var.
		Project: os.Getenv("PROJECT_ID"),
	}
	ext, err := factory.Create(ctx, set, cfg)
	require.NoError(t, err)
	err = ext.Start(ctx, componenttest.NewNopHost())
	return &mockHost{ext: map[component.ID]component.Component{
		googleclientauthExtensionID: ext,
	}}
}

type mockHost struct {
	ext map[component.ID]component.Component
}

func (nh *mockHost) GetExtensions() map[component.ID]component.Component {
	return nh.ext
}
