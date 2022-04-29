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

package integrationtest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
)

func createMetricsExporter(
	ctx context.Context,
	t *testing.T,
	test *TestCase,
) *collector.MetricsExporter {
	logger, _ := zap.NewProduction()
	exporter, err := collector.NewGoogleCloudMetricsExporter(
		ctx,
		test.CreateMetricConfig(),
		logger,
		"latest",
		collector.DefaultTimeout,
	)
	require.NoError(t, err)
	t.Log("Collector metrics exporter started")
	return exporter
}

func TestIntegrationMetrics(t *testing.T) {
	ctx := context.Background()
	endTime := time.Now()
	startTime := endTime.Add(-time.Second)

	for _, test := range MetricsTestCases {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			test.SkipIfNeeded(t)
			metrics := test.LoadOTLPMetricsInput(t, startTime, endTime)
			exporter := createMetricsExporter(ctx, t, &test)
			defer func() { require.NoError(t, exporter.Shutdown(ctx)) }()

			require.NoError(
				t,
				exporter.PushMetrics(ctx, metrics),
				"Failed to export metrics",
			)
		})
	}
}
