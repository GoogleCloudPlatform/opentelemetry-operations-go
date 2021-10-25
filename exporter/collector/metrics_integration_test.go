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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/integrationtest"
)

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

func TestIntegrationMetrics(t *testing.T) {
	ctx := context.Background()
	endTime := time.Now()
	startTime := endTime.Add(-time.Second)

	for _, test := range integrationtest.TestCases {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			metrics := test.LoadOTLPMetricsInput(t, startTime, endTime)
			exporter := createMetricsExporter(ctx, t)
			defer require.NoError(t, exporter.Shutdown(ctx))

			require.NoError(
				t,
				exporter.ConsumeMetrics(ctx, metrics),
				"Failed to export metrics",
			)
		})
	}
}
