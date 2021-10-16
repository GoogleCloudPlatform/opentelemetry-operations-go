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

//go:build integrationtest
// +build integrationtest

package collector_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/model/pdata"
)

const (
	testTimeout = time.Second * 30
)

func createMetricsExporter(ctx context.Context, t *testing.T) component.MetricsExporter {
	factory := collector.NewFactory()
	cfg := factory.CreateDefaultConfig()
	eCfg := cfg.(*collector.Config)
	// If not set it will use ADC
	eCfg.ProjectID = os.Getenv("PROJECT_ID")
	// Disable queued retries as there is no way to flush them
	eCfg.RetrySettings.Enabled = false
	eCfg.QueueSettings.Enabled = false

	exporter, err := factory.CreateMetricsExporter(
		ctx,
		componenttest.NewNopExporterCreateSettings(),
		eCfg,
	)
	require.NoError(t, err)
	require.NoError(t, exporter.Start(ctx, componenttest.NewNopHost()))
	t.Log("Collector metrics exporter started")
	return exporter
}

func TestIntegrationMetrics(t *testing.T) {
	ctx := context.Background()

	exporter := createMetricsExporter(ctx, t)
	defer func() {
		if err := exporter.Shutdown(ctx); err != nil {
			t.Errorf("Exporter shutdown failed: %v", err)
		}
		t.Log("Exporter shutdown successfully")
	}()

	// Temporary just for testing CI setup. This will be replaced with OTLP fixtures.
	metrics := pdata.NewMetrics()
	metric := metrics.ResourceMetrics().
		AppendEmpty().
		InstrumentationLibraryMetrics().
		AppendEmpty().
		Metrics().
		AppendEmpty()
	metric.SetDataType(pdata.MetricDataTypeSum)
	metric.SetName("testcounter")
	metric.SetDescription("This is a test counter")
	metric.SetUnit("1")
	sum := metric.Sum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
	point := sum.DataPoints().AppendEmpty()
	now := time.Now()
	point.SetStartTimestamp(pdata.NewTimestampFromTime(now.Add(-5 * time.Second)))
	point.SetTimestamp(pdata.NewTimestampFromTime(now))
	point.SetIntVal(253)
	point.Attributes().Insert("foo", pdata.NewAttributeValueString("bar"))

	require.NoError(t, exporter.ConsumeMetrics(ctx, metrics), "Failed to export metrics")
}
