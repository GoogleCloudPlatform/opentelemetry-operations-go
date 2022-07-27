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

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/integrationtest/protos"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/integrationtest/testcases"
)

func TestTraces(t *testing.T) {
	ctx := context.Background()
	endTime := time.Now()
	startTime := endTime.Add(-time.Second)

	for _, test := range testcases.TracesTestCases {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			test.SkipIfNeeded(t)

			traces := test.LoadOTLPTracesInput(t, startTime, endTime)
			testServer, err := NewTracesTestServer()
			require.NoError(t, err)
			go testServer.Serve()
			defer testServer.Shutdown()
			testServerExporter := testServer.NewExporter(ctx, t, test.CreateTraceConfig())

			err = testServerExporter.PushTraces(ctx, traces)
			if !test.ExpectErr {
				require.NoError(t, err, "Failed to export traces to local test server")
			} else {
				require.Error(t, err, "Did not see expected error")
			}
			require.NoError(t, testServerExporter.Shutdown(ctx))

			expectFixture := test.LoadTraceExpectFixture(
				t,
				startTime,
				endTime,
			)

			fixture := &protos.TraceExpectFixture{
				BatchWriteSpansRequest: testServer.CreateBatchWriteSpansRequests(),
			}

			diff := DiffTraceProtos(
				t,
				fixture,
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
