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

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/integrationtest/protos"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/integrationtest/testcases"
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

			testServerExporter := NewLogTestExporter(ctx, t, testServer, test.CreateLogConfig(), test.ConfigureLogsExporter)

			require.NoError(
				t,
				testServerExporter.PushLogs(ctx, logs),
				"Failed to export logs to local test server",
			)

			expectFixture := test.LoadLogExpectFixture(
				t,
				timestamp,
			)

			// sort the entries in each request
			for listIndex := 0; listIndex < len(expectFixture.WriteLogEntriesRequests); listIndex++ {
				sort.Slice(expectFixture.WriteLogEntriesRequests[listIndex].Entries, func(i, j int) bool {
					if expectFixture.WriteLogEntriesRequests[listIndex].Entries[i].LogName != expectFixture.WriteLogEntriesRequests[listIndex].Entries[j].LogName {
						return expectFixture.WriteLogEntriesRequests[listIndex].Entries[i].LogName < expectFixture.WriteLogEntriesRequests[listIndex].Entries[j].LogName
					}
					return expectFixture.WriteLogEntriesRequests[listIndex].Entries[i].String() < expectFixture.WriteLogEntriesRequests[listIndex].Entries[j].String()
				})
			}
			// sort each request. if the requests have the same name (or just as likely, they both have no name set at the request level),
			// peek at the first entry's logname in the request
			sort.Slice(expectFixture.WriteLogEntriesRequests, func(i, j int) bool {
				if expectFixture.WriteLogEntriesRequests[i].LogName != expectFixture.WriteLogEntriesRequests[j].LogName {
					return expectFixture.WriteLogEntriesRequests[i].LogName < expectFixture.WriteLogEntriesRequests[j].LogName
				}
				return expectFixture.WriteLogEntriesRequests[i].Entries[0].LogName < expectFixture.WriteLogEntriesRequests[j].Entries[0].LogName
			})

			fixture := &protos.LogExpectFixture{
				WriteLogEntriesRequests: testServer.CreateWriteLogEntriesRequests(),
				UserAgent:               testServer.UserAgent(),
			}
			// sort the entries in each request
			for listIndex := 0; listIndex < len(fixture.WriteLogEntriesRequests); listIndex++ {
				sort.Slice(fixture.WriteLogEntriesRequests[listIndex].Entries, func(i, j int) bool {
					if fixture.WriteLogEntriesRequests[listIndex].Entries[i].LogName < fixture.WriteLogEntriesRequests[listIndex].Entries[j].LogName {
						return fixture.WriteLogEntriesRequests[listIndex].Entries[i].LogName < fixture.WriteLogEntriesRequests[listIndex].Entries[j].LogName
					}
					return fixture.WriteLogEntriesRequests[listIndex].Entries[i].String() < fixture.WriteLogEntriesRequests[listIndex].Entries[j].String()
				})
			}
			// sort each request. if the requests have the same name (or just as likely, they both have no name set at the request level),
			// peek at the first entry's logname in the request
			sort.Slice(fixture.WriteLogEntriesRequests, func(i, j int) bool {
				if fixture.WriteLogEntriesRequests[i].LogName != fixture.WriteLogEntriesRequests[j].LogName {
					return fixture.WriteLogEntriesRequests[i].LogName < fixture.WriteLogEntriesRequests[j].LogName
				}
				return fixture.WriteLogEntriesRequests[i].Entries[0].LogName < fixture.WriteLogEntriesRequests[j].Entries[0].LogName
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
