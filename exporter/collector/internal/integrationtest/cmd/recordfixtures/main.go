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

type FakeTesting struct {
	testing.TB
}

func (t *FakeTesting) Logf(format string, args ...interface{}) {
	log.Printf(format, args...)
}
func (t *FakeTesting) Errorf(format string, args ...interface{}) {
	panic(fmt.Errorf(format, args...))
}
func (t *FakeTesting) FailNow() {
	t.Errorf("FailNow()")
}
func (t *FakeTesting) Helper()      {}
func (t *FakeTesting) Name() string { return "record fixtures" }

func main() {
	t := &FakeTesting{}
	ctx := context.Background()
	endTime := time.Now()
	startTime := endTime.Add(-time.Second)

	testServer, err := integrationtest.NewMetricTestServer()
	if err != nil {
		panic(err)
	}
	go testServer.Serve()
	defer testServer.Shutdown()

	for _, test := range integrationtest.TestCases {
		if test.Skip {
			continue
		}
		func() {
			metrics := test.LoadOTLPMetricsInput(t, startTime, endTime)
			testServerExporter := testServer.NewExporter(ctx, t, *test.CreateConfig())
			inMemoryOCExporter, err := integrationtest.NewInMemoryOCViewExporter()
			require.NoError(t, err)
			defer inMemoryOCExporter.Shutdown(ctx)

			require.NoError(t, testServerExporter.PushMetrics(ctx, metrics), "failed to export metrics to local test server")
			require.NoError(t, testServerExporter.Shutdown(ctx))

			fixture := &integrationtest.MetricExpectFixture{
				CreateMetricDescriptorRequests:  testServer.CreateMetricDescriptorRequests(),
				CreateTimeSeriesRequests:        testServer.CreateTimeSeriesRequests(),
				CreateServiceTimeSeriesRequests: testServer.CreateServiceTimeSeriesRequests(),
				SelfObservabilityMetrics:        inMemoryOCExporter.Proto(),
			}
			test.SaveRecordedFixtures(t, fixture)
		}()
	}
}
