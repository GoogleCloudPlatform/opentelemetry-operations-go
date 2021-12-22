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

package collector_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/integrationtest"
)

func TestMetrics(t *testing.T) {
	ctx := context.Background()
	endTime := time.Now()
	startTime := endTime.Add(-time.Second)

	for _, test := range integrationtest.TestCases {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			metrics := test.LoadOTLPMetricsInput(t, startTime, endTime)

			testServer, err := integrationtest.NewMetricTestServer()
			require.NoError(t, err)
			go testServer.Serve()
			defer testServer.Shutdown()
			testServerExporter := testServer.NewExporter(ctx, t, *test.CreateConfig())
			defer func() { require.NoError(t, testServerExporter.Shutdown(ctx)) }()

			require.NoError(
				t,
				testServerExporter.ConsumeMetrics(ctx, metrics),
				"Failed to export metrics to local test server",
			)
			actualCreateMetricDescriptorReq := testServer.CreateMetricDescriptorRequests()
			actualCreateTimeSeriesReq := testServer.CreateTimeSeriesRequests()
			actualCreateServiceTimeSeriesReq := testServer.CreateServiceTimeSeriesRequests()

			expectFixture := test.LoadExpectFixture(
				t,
				startTime,
				endTime,
			)
			diff := integrationtest.DiffProtos(
				&integrationtest.MetricExpectFixture{
					CreateTimeSeriesRequests:        actualCreateTimeSeriesReq,
					CreateMetricDescriptorRequests:  actualCreateMetricDescriptorReq,
					CreateServiceTimeSeriesRequests: actualCreateServiceTimeSeriesReq,
				},
				expectFixture,
			)
			require.Emptyf(
				t,
				diff,
				"Expected requests fixture and actual GCM requests differ:\n%v",
				diff,
			)
		})
	}
}
