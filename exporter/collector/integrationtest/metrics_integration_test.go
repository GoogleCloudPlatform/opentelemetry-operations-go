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
	"os"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/integrationtest/testcases"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
)

func createMetricsExporter(
	ctx context.Context,
	t *testing.T,
	test *testcases.TestCase,
) *collector.MetricsExporter {
	logger, _ := zap.NewProduction()
	cfg := test.CreateMetricConfig()
	// For sending to a real project, set the project ID from an env var.
	cfg.ProjectID = os.Getenv("PROJECT_ID")
	exporter, err := collector.NewGoogleCloudMetricsExporter(
		ctx,
		cfg,
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

	for _, test := range testcases.MetricsTestCases {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			test.SkipIfNeeded(t)
			metrics := test.LoadOTLPMetricsInput(t, startTime, endTime)
			setSecondProjectInMetrics(t, metrics)
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

// setSecondProjectInMetrics overwrites the gcp.project.id resource attribute
// with the contents of the SECOND_PROJECT_ID environment variable. This makes
// the integration test send those metrics with a gcp.project.id attribute to
// the specified second project.
func setSecondProjectInMetrics(t *testing.T, metrics pmetric.Metrics) {
	secondProject := os.Getenv(testcases.secondProjectEnv)
	require.NotEmpty(t, secondProject, "set the SECOND_PROJECT_ID environment to run this test")
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		metrics.ResourceMetrics().At(i).Resource().Attributes().Update(
			resourcemapping.ProjectIDAttributeKey,
			pcommon.NewValueString(secondProject),
		)
	}
}
