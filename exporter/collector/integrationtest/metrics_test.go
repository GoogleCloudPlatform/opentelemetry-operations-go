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
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMetrics(t *testing.T) {
	ctx := context.Background()
	endTime := time.Now()
	startTime := endTime.Add(-time.Second)

	for _, test := range MetricsTestCases {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			test.SkipIfNeeded(t)

			metrics := test.LoadOTLPMetricsInput(t, startTime, endTime)

			testServer, err := NewMetricTestServer()
			require.NoError(t, err)
			go testServer.Serve()
			defer testServer.Shutdown()
			testServerExporter := testServer.NewExporter(ctx, t, test.CreateMetricConfig())
			// For collecting self observability metrics
			inMemoryOCExporter, err := NewInMemoryOCViewExporter()
			require.NoError(t, err)
			defer inMemoryOCExporter.Shutdown(ctx)

			err = testServerExporter.PushMetrics(ctx, metrics)
			if !test.ExpectErr {
				require.NoError(t, err, "Failed to export metrics to local test server")
			} else {
				require.Error(t, err, "Did not see expected error")
			}
			require.NoError(t, testServerExporter.Shutdown(ctx))

			expectFixture := test.LoadMetricExpectFixture(
				t,
				startTime,
				endTime,
			)
			sort.Slice(expectFixture.CreateTimeSeriesRequests, func(i, j int) bool {
				return expectFixture.CreateTimeSeriesRequests[i].Name < expectFixture.CreateTimeSeriesRequests[j].Name
			})
			sort.Slice(expectFixture.CreateMetricDescriptorRequests, func(i, j int) bool {
				return expectFixture.CreateMetricDescriptorRequests[i].Name < expectFixture.CreateMetricDescriptorRequests[j].Name
			})
			sort.Slice(expectFixture.CreateServiceTimeSeriesRequests, func(i, j int) bool {
				return expectFixture.CreateServiceTimeSeriesRequests[i].Name < expectFixture.CreateServiceTimeSeriesRequests[j].Name
			})

			selfObsMetrics, err := inMemoryOCExporter.Proto(ctx)
			require.NoError(t, err)
			fixture := &MetricExpectFixture{
				CreateTimeSeriesRequests:        testServer.CreateTimeSeriesRequests(),
				CreateMetricDescriptorRequests:  testServer.CreateMetricDescriptorRequests(),
				CreateServiceTimeSeriesRequests: testServer.CreateServiceTimeSeriesRequests(),
				SelfObservabilityMetrics:        selfObsMetrics,
			}
			sort.Slice(fixture.CreateTimeSeriesRequests, func(i, j int) bool {
				return fixture.CreateTimeSeriesRequests[i].Name < fixture.CreateTimeSeriesRequests[j].Name
			})
			sort.Slice(fixture.CreateMetricDescriptorRequests, func(i, j int) bool {
				return fixture.CreateMetricDescriptorRequests[i].Name < fixture.CreateMetricDescriptorRequests[j].Name
			})
			sort.Slice(fixture.CreateServiceTimeSeriesRequests, func(i, j int) bool {
				return fixture.CreateServiceTimeSeriesRequests[i].Name < fixture.CreateServiceTimeSeriesRequests[j].Name
			})
			diff := DiffMetricProtos(
				t,
				fixture,
				expectFixture,
				func(i, j int) bool {
					return fixture.CreateTimeSeriesRequests[i].Name < fixture.CreateTimeSeriesRequests[j].Name
				},
				func(i, j int) bool {
					return fixture.CreateMetricDescriptorRequests[i].Name < fixture.CreateMetricDescriptorRequests[j].Name
				},
				func(i, j int) bool {
					return fixture.CreateServiceTimeSeriesRequests[i].Name < fixture.CreateServiceTimeSeriesRequests[j].Name
				},
				func(i, j int) bool {
					return expectFixture.CreateTimeSeriesRequests[i].Name < expectFixture.CreateTimeSeriesRequests[j].Name
				},
				func(i, j int) bool {
					return expectFixture.CreateMetricDescriptorRequests[i].Name < expectFixture.CreateMetricDescriptorRequests[j].Name
				},
				func(i, j int) bool {
					return expectFixture.CreateServiceTimeSeriesRequests[i].Name < expectFixture.CreateServiceTimeSeriesRequests[j].Name
				},
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
