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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
)

func createLogsExporter(
	ctx context.Context,
	t *testing.T,
	test *LogsTestCase,
) *collector.LogsExporter {
	logger, _ := zap.NewProduction()
	exporter, err := collector.NewGoogleCloudLogsExporter(
		ctx,
		test.CreateConfig(),
		logger,
	)
	require.NoError(t, err)
	t.Log("Collector logs exporter started")
	return exporter
}

func TestIntegrationLogs(t *testing.T) {
	ctx := context.Background()
	endTime := time.Now()
	startTime := endTime.Add(-time.Second)

	for _, test := range LogsTestCases {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			logs := test.LoadOTLPLogsInput(t, startTime)
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
