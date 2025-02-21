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

package integrationtest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/integrationtest/protos"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/integrationtest/testcases"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/logsutil"
	gcpmetric "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock"
)

// OTel metrics exporter used to capture self observability metrics.
type InMemoryOTelExporter struct {
	testServer    *cloudmock.MetricsTestServer
	MeterProvider *sdkmetric.MeterProvider
}

func (i *InMemoryOTelExporter) Proto(ctx context.Context) (*protos.SelfObservabilityMetric, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Millisecond*500)
	defer cancel()
	err := i.MeterProvider.ForceFlush(ctx)
	if err != nil {
		return nil, err
	}

	return &protos.SelfObservabilityMetric{
			CreateTimeSeriesRequests:       i.testServer.CreateTimeSeriesRequests(),
			CreateMetricDescriptorRequests: i.testServer.CreateMetricDescriptorRequests(),
		},
		nil
}

// Shutdown unregisters the global OpenCensus views to reset state for the next test.
func (i *InMemoryOTelExporter) Shutdown(ctx context.Context) error {
	err := i.MeterProvider.Shutdown(ctx)
	i.testServer.Shutdown()
	return err
}

// NewInMemoryOTelExporter creates a new in memory OTel exporter for testing. Be sure to defer
// a call to Shutdown().
func NewInMemoryOTelExporter() (*InMemoryOTelExporter, error) {
	testServer, err := cloudmock.NewMetricTestServer()
	if err != nil {
		return nil, err
	}
	//nolint:errcheck
	go testServer.Serve()
	conn, err := grpc.NewClient(testServer.Endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	exporter, err := gcpmetric.New(
		gcpmetric.WithProjectID("myproject"),
		gcpmetric.WithMonitoringClientOptions(option.WithGRPCConn(conn)),
		gcpmetric.WithFilteredResourceAttributes(gcpmetric.NoAttributes),
	)
	if err != nil {
		return nil, err
	}

	return &InMemoryOTelExporter{
			testServer: testServer,
			MeterProvider: sdkmetric.NewMeterProvider(
				sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
			),
		},
		nil
}

func NewTraceTestExporter(
	ctx context.Context,
	t testing.TB,
	s *cloudmock.TracesTestServer,
	cfg collector.Config,
	meterProvider metric.MeterProvider,
) *collector.TraceExporter {
	cfg.TraceConfig.ClientConfig.Endpoint = s.Endpoint
	cfg.TraceConfig.ClientConfig.UseInsecure = true
	cfg.ProjectID = "fakeprojectid"

	set := testcases.NewTestExporterSettings(zap.NewNop(), meterProvider)
	testcases.SetTestUserAgent(&cfg, set.BuildInfo)
	exporter, err := collector.NewGoogleCloudTracesExporter(
		ctx,
		cfg,
		set,
		collector.DefaultTimeout,
	)
	require.NoError(t, err)
	err = exporter.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	t.Logf("Collector TracesTestServer exporter started, pointing at %v", cfg.TraceConfig.ClientConfig.Endpoint)
	return exporter
}

// NewMetricsTestExporter creates and starts a googlecloud exporter by updating the
// given cfg copy to point to the test server.
func NewMetricTestExporter(
	ctx context.Context,
	t testing.TB,
	m *cloudmock.MetricsTestServer,
	cfg collector.Config,
	meterProvider metric.MeterProvider,
) *collector.MetricsExporter {
	cfg.MetricConfig.ClientConfig.Endpoint = m.Endpoint
	cfg.MetricConfig.ClientConfig.UseInsecure = true

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	set := testcases.NewTestExporterSettings(logger, meterProvider)
	testcases.SetTestUserAgent(&cfg, set.BuildInfo)
	exporter, err := collector.NewGoogleCloudMetricsExporter(
		ctx,
		cfg,
		set,
		collector.DefaultTimeout,
	)
	require.NoError(t, err)
	err = exporter.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	t.Logf("Collector MetricsTestServer exporter started, pointing at %v", cfg.MetricConfig.ClientConfig.Endpoint)
	return exporter
}

func NewLogTestExporter(
	ctx context.Context,
	t testing.TB,
	l *cloudmock.LogsTestServer,
	cfg collector.Config,
	extraConfig *logsutil.ExporterConfig,
	meterProvider metric.MeterProvider,
) *collector.LogsExporter {
	cfg.LogConfig.ClientConfig.Endpoint = l.Endpoint
	cfg.LogConfig.ClientConfig.UseInsecure = true
	cfg.ProjectID = "fakeprojectid"

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	set := testcases.NewTestExporterSettings(logger, meterProvider)
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

	exporter.ConfigureExporter(extraConfig)
	t.Logf("Collector LogsTestServer exporter started, pointing at %v", cfg.LogConfig.ClientConfig.Endpoint)
	return exporter
}
