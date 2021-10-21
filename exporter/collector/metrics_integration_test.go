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

package collector_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/integrationtest"
)

var (
	testCases = []integrationtest.MetricsTestCase{
		{
			Name:                                     "Basic Counter",
			OTLPInputFixturePath:                     "testdata/fixtures/basic_counter_metrics.json",
			CreateMetricDescriptorRequestFixturePath: "testdata/fixtures/basic_counter_metricdescriptor_expect.json",
			CreateTimeSeriesRequestFixturePath:       "testdata/fixtures/basic_counter_metrics_expect.json",
		},
	}
)

func createConfig(factory component.ExporterFactory) *collector.Config {
	cfg := factory.CreateDefaultConfig().(*collector.Config)
	// If not set it will use ADC
	cfg.ProjectID = os.Getenv("PROJECT_ID")
	// Disable queued retries as there is no way to flush them
	cfg.RetrySettings.Enabled = false
	cfg.QueueSettings.Enabled = false
	return cfg
}

func createMetricsExporter(ctx context.Context, t *testing.T) component.MetricsExporter {
	factory := collector.NewFactory()

	exporter, err := factory.CreateMetricsExporter(
		ctx,
		componenttest.NewNopExporterCreateSettings(),
		createConfig(factory),
	)
	require.NoError(t, err)
	require.NoError(t, exporter.Start(ctx, componenttest.NewNopHost()))
	t.Log("Collector metrics exporter started")
	return exporter
}

func createMetricsTestServerExporter(
	ctx context.Context,
	t *testing.T,
	testServer *integrationtest.MetricsTestServer,
) component.MetricsExporter {
	factory := collector.NewFactory()
	cfg := createConfig(factory)
	cfg.Endpoint = testServer.Endpoint
	cfg.UseInsecure = true
	cfg.ProjectID = "fakeprojectid"

	exporter, err := factory.CreateMetricsExporter(
		ctx,
		componenttest.NewNopExporterCreateSettings(),
		cfg,
	)
	require.NoError(t, err)
	require.NoError(t, exporter.Start(ctx, componenttest.NewNopHost()))
	t.Logf("Collector MetricsTestServer exporter started, pointing at %v", cfg.Endpoint)
	return exporter
}

func TestIntegrationMetrics(t *testing.T) {
	ctx := context.Background()
	endTime := time.Now()
	startTime := endTime.Add(-time.Second)

	for _, test := range testCases {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			metrics := test.LoadOTLPMetricsInput(t, startTime, endTime)

			mockExportPassed := t.Run("Mock Export", func(t *testing.T) {
				testServer, err := integrationtest.NewMetricTestServer()
				require.NoError(t, err)
				go testServer.Serve()
				testServerExporter := createMetricsTestServerExporter(ctx, t, testServer)

				defer func() {
					testServer.Shutdown()
					require.NoError(t, testServerExporter.Shutdown(ctx))
				}()

				// First try exporting to local GCM test server and compare the requests
				require.NoError(
					t,
					testServerExporter.ConsumeMetrics(ctx, metrics),
					"Failed to export metrics to local test server",
				)
				actualCreateMetricDescriptorReq := <-testServer.CreateMetricDescriptorChan
				actualCreateTimeSeriesReq := <-testServer.CreateTimeSeriesChan

				expectedCreateMetricDescriptorReq := test.LoadCreateMetricDescriptorFixture(
					t,
					startTime,
					endTime,
				)
				diff := integrationtest.DiffProtos(
					actualCreateMetricDescriptorReq,
					expectedCreateMetricDescriptorReq,
				)
				require.Emptyf(
					t,
					diff,
					"Expected CreateMetricDescriptor request and actual GCM request differ:\n%v",
					diff,
				)
				expectedCreateTimeSeriesReq := test.LoadCreateTimeSeriesFixture(t, startTime, endTime)
				diff = integrationtest.DiffProtos(
					actualCreateTimeSeriesReq,
					expectedCreateTimeSeriesReq,
				)
				require.Emptyf(
					t,
					diff,
					"Expected CreateTimeSeries request and actual GCM request differ:\n%v",
					diff,
				)
			})

			t.Run("Real Export", func(t *testing.T) {
				integrationtest.SkipIfNotExportToGcp(t)
				if !mockExportPassed {
					t.Skip("Skipping export to real GCP APIs because mock export failed")
				}

				exporter := createMetricsExporter(ctx, t)
				defer func() { require.NoError(t, exporter.Shutdown(ctx)) }()

				// Try exporting with the real GCM exporter
				require.NoError(
					t,
					exporter.ConsumeMetrics(ctx, metrics),
					"Failed to export metrics",
				)
			})
		})
	}
}
