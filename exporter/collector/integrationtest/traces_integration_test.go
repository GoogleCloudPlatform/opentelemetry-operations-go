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

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/integrationtest/testcases"
	"github.com/stretchr/testify/require"
)

func createTracesExporter(
	ctx context.Context,
	t *testing.T,
	test *testcases.TestCase,
) *collector.TraceExporter {
	cfg := test.CreateTraceConfig()
	cfg.ProjectID = os.Getenv("PROJECT_ID")
	exporter, err := collector.NewGoogleCloudTracesExporter(
		ctx,
		cfg,
		"latest",
		collector.DefaultTimeout,
	)
	require.NoError(t, err)
	t.Log("Collector traces exporter started")
	return exporter
}

func TestIntegrationTraces(t *testing.T) {
	ctx := context.Background()
	endTime := time.Now()
	startTime := endTime.Add(-time.Second)

	for _, test := range testcases.TracesTestCases {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			test.SkipIfNeeded(t)
			traces := test.LoadOTLPTracesInput(t, startTime, endTime)
			exporter := createTracesExporter(ctx, t, &test)
			defer func() { require.NoError(t, exporter.Shutdown(ctx)) }()

			require.NoError(
				t,
				exporter.PushTraces(ctx, traces),
				"Failed to export metrics",
			)
		})
	}
}
