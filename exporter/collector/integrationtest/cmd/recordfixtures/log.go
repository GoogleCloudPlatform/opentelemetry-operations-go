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

// Script to record test expectation fixtures and save them to disk.

package main

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/integrationtest"
)

type FakeLogTesting struct {
	testing.TB
}

func (t *FakeLogTesting) Logf(format string, args ...interface{}) {
	log.Printf(format, args...)
}
func (t *FakeLogTesting) Errorf(format string, args ...interface{}) {
	panic(fmt.Errorf(format, args...))
}
func (t *FakeLogTesting) FailNow() {
	t.Errorf("FailNow()")
}
func (t *FakeLogTesting) Helper()      {}
func (t *FakeLogTesting) Name() string { return "record fixtures" }

func main() {
	t := &FakeLogTesting{}
	ctx := context.Background()
	startTime := time.Now()

	testServer, err := integrationtest.NewLoggingTestServer()
	if err != nil {
		panic(err)
	}
	go testServer.Serve()
	defer testServer.Shutdown()

	for _, test := range integrationtest.LogsTestCases {
		if test.Skip {
			continue
		}
		func() {
			logs := test.LoadOTLPLogsInput(t, startTime)
			testServerExporter := testServer.NewExporter(ctx, t, test.CreateLogConfig())

			require.NoError(t, testServerExporter.PushLogs(ctx, logs), "failed to export metrics to local test server")
			require.NoError(t, testServerExporter.Shutdown(ctx))

			require.NoError(t, err)
			fixture := &integrationtest.LogExpectFixture{
				WriteLogEntriesRequests: testServer.CreateWriteLogEntriesRequests(),
			}
			test.SaveRecordedLogFixtures(t, fixture)
		}()
	}
}
