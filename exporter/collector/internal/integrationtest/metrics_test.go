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

package integrationtest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMetrics(t *testing.T) {
	ctx := context.Background()
	endTime := time.Now()
	startTime := endTime.Add(-time.Second)

	for _, test := range TestCases {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			test.SkipIfNeeded(t)
			metrics := test.LoadOTLPMetricsInput(t, startTime, endTime)

			testServer, err := NewMetricTestServer()
			require.NoError(t, err)
			go testServer.Serve()
			defer testServer.Shutdown()
			testServerExporter := testServer.NewExporter(ctx, t, test.CreateConfig())
			// For collecting self observability metrics
			inMemoryOCExporter, err := NewInMemoryOCViewExporter()
			require.NoError(t, err)
			defer inMemoryOCExporter.Shutdown(ctx)

			require.NoError(
				t,
				testServerExporter.PushMetrics(ctx, metrics),
				"Failed to export metrics to local test server",
			)
			require.NoError(t, testServerExporter.Shutdown(ctx))

			expectFixture := test.LoadExpectFixture(
				t,
				startTime,
				endTime,
			)
			diff := DiffProtos(
				&MetricExpectFixture{
					CreateTimeSeriesRequests:        testServer.CreateTimeSeriesRequests(),
					CreateMetricDescriptorRequests:  testServer.CreateMetricDescriptorRequests(),
					CreateServiceTimeSeriesRequests: testServer.CreateServiceTimeSeriesRequests(),
					SelfObservabilityMetrics:        inMemoryOCExporter.Proto(),
				},
				expectFixture,
			)
			if diff != "" {
				require.Fail(
					t,
					"Expected requests fixture and actual GCM requests differ",
					diff,
				)
			}
		})
	}
}
