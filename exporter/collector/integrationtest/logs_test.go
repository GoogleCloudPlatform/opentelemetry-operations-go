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
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/integrationtest/protos"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/integrationtest/testcases"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock"
)

func TestLogs(t *testing.T) {
	ctx := context.Background()
	timestamp := time.Now()

	for _, test := range testcases.LogsTestCases {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			logs := test.LoadOTLPLogsInput(t, timestamp)

			testServer, err := cloudmock.NewLoggingTestServer()
			require.NoError(t, err)
			go testServer.Serve()
			defer testServer.Shutdown()

			testServerExporter := NewLogTestExporter(ctx, t, testServer, test.CreateLogConfig())

			require.NoError(
				t,
				testServerExporter.PushLogs(ctx, logs),
				"Failed to export logs to local test server",
			)

			expectFixture := test.LoadLogExpectFixture(
				t,
				timestamp,
			)
			sort.Slice(expectFixture.WriteLogEntriesRequests, func(i, j int) bool {
				return expectFixture.WriteLogEntriesRequests[i].LogName < expectFixture.WriteLogEntriesRequests[j].LogName
			})

			fixture := &protos.LogExpectFixture{
				WriteLogEntriesRequests: testServer.CreateWriteLogEntriesRequests(),
			}
			sort.Slice(fixture.WriteLogEntriesRequests, func(i, j int) bool {
				return fixture.WriteLogEntriesRequests[i].LogName < fixture.WriteLogEntriesRequests[j].LogName
			})

			diff := DiffLogProtos(
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
