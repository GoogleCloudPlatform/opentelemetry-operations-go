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
	apioption "google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/integrationtest/protos"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/integrationtest/testcases"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock"
)

func TestCollectorMetrics(t *testing.T) {
	ctx := context.Background()
	endTime := time.Now()
	startTime := endTime.Add(-time.Second)

	for _, test := range testcases.MetricsTestCases {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			test.SkipIfNeeded(t)

			metrics := test.LoadOTLPMetricsInput(t, startTime, endTime)

			testServer, err := cloudmock.NewMetricTestServer()
			require.NoError(t, err)
			//nolint:errcheck
			go testServer.Serve()
			defer testServer.Shutdown()
			testServerExporter := NewMetricTestExporter(ctx, t, testServer, test.CreateCollectorMetricConfig())
			// For collecting self observability metrics
			inMemoryOCExporter, err := NewInMemoryOCViewExporter()
			require.NoError(t, err)
			//nolint:errcheck
			defer inMemoryOCExporter.Shutdown(ctx)

			err = testServerExporter.PushMetrics(ctx, metrics)
			if !test.ExpectErr {
				require.NoError(t, err, "Failed to export metrics to local test server")
			} else {
				require.Error(t, err, "Did not see expected error")
			}
			require.NoError(t, testServerExporter.Shutdown(ctx))

			if !test.ExpectRetries {
				require.Zero(t, testServer.RetryCount, "Server returned >0 retries when not expected")
			} else {
				require.NotZero(t, testServer.RetryCount, "Server returned 0 retries when expected >0")
			}

			expectFixture := test.LoadMetricFixture(
				t,
				test.ExpectFixturePath,
				startTime,
				endTime,
			)
			sort.Slice(expectFixture.CreateTimeSeriesRequests, func(i, j int) bool {
				return expectFixture.CreateTimeSeriesRequests[i].Name < expectFixture.CreateTimeSeriesRequests[j].Name
			})
			sort.Slice(expectFixture.CreateMetricDescriptorRequests, func(i, j int) bool {
				if expectFixture.CreateMetricDescriptorRequests[i].Name != expectFixture.CreateMetricDescriptorRequests[j].Name {
					return expectFixture.CreateMetricDescriptorRequests[i].Name < expectFixture.CreateMetricDescriptorRequests[j].Name
				}
				return expectFixture.CreateMetricDescriptorRequests[i].MetricDescriptor.Name < expectFixture.CreateMetricDescriptorRequests[j].MetricDescriptor.Name
			})
			sort.Slice(expectFixture.CreateServiceTimeSeriesRequests, func(i, j int) bool {
				return expectFixture.CreateServiceTimeSeriesRequests[i].Name < expectFixture.CreateServiceTimeSeriesRequests[j].Name
			})

			selfObsMetrics, err := inMemoryOCExporter.Proto(ctx)
			require.NoError(t, err)
			fixture := &protos.MetricExpectFixture{
				CreateTimeSeriesRequests:        testServer.CreateTimeSeriesRequests(),
				CreateMetricDescriptorRequests:  testServer.CreateMetricDescriptorRequests(),
				CreateServiceTimeSeriesRequests: testServer.CreateServiceTimeSeriesRequests(),
				SelfObservabilityMetrics:        selfObsMetrics,
			}
			sort.Slice(fixture.CreateTimeSeriesRequests, func(i, j int) bool {
				return fixture.CreateTimeSeriesRequests[i].Name < fixture.CreateTimeSeriesRequests[j].Name
			})
			sort.Slice(fixture.CreateMetricDescriptorRequests, func(i, j int) bool {
				if fixture.CreateMetricDescriptorRequests[i].Name != fixture.CreateMetricDescriptorRequests[j].Name {
					return fixture.CreateMetricDescriptorRequests[i].Name < fixture.CreateMetricDescriptorRequests[j].Name
				}
				return fixture.CreateMetricDescriptorRequests[i].MetricDescriptor.Name < fixture.CreateMetricDescriptorRequests[j].MetricDescriptor.Name
			})
			sort.Slice(fixture.CreateServiceTimeSeriesRequests, func(i, j int) bool {
				return fixture.CreateServiceTimeSeriesRequests[i].Name < fixture.CreateServiceTimeSeriesRequests[j].Name
			})
			diff := DiffMetricProtos(
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

			if len(test.CompareFixturePath) > 0 {
				compareFixture := test.LoadMetricFixture(
					t,
					test.CompareFixturePath,
					startTime,
					endTime,
				)
				sort.Slice(compareFixture.CreateTimeSeriesRequests, func(i, j int) bool {
					return compareFixture.CreateTimeSeriesRequests[i].Name < compareFixture.CreateTimeSeriesRequests[j].Name
				})
				sort.Slice(compareFixture.CreateMetricDescriptorRequests, func(i, j int) bool {
					if compareFixture.CreateMetricDescriptorRequests[i].Name != compareFixture.CreateMetricDescriptorRequests[j].Name {
						return compareFixture.CreateMetricDescriptorRequests[i].Name < compareFixture.CreateMetricDescriptorRequests[j].Name
					}
					return compareFixture.CreateMetricDescriptorRequests[i].MetricDescriptor.Name < compareFixture.CreateMetricDescriptorRequests[j].MetricDescriptor.Name
				})
				sort.Slice(compareFixture.CreateServiceTimeSeriesRequests, func(i, j int) bool {
					return compareFixture.CreateServiceTimeSeriesRequests[i].Name < compareFixture.CreateServiceTimeSeriesRequests[j].Name
				})

				diff := DiffMetricProtos(
					t,
					fixture,
					compareFixture,
				)
				if diff != "" {
					require.Fail(
						t,
						"Expected requests fixture and actual GCM requests differ",
						diff,
					)
				}
			}
		})
	}
}

func TestSDKMetrics(t *testing.T) {
	ctx := context.Background()
	endTime := time.Now()
	startTime := endTime.Add(-time.Second)

	for _, test := range testcases.MetricsTestCases {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			test.SkipIfNeededForSDK(t)

			metrics := test.LoadOTLPMetricsInput(t, startTime, endTime)

			testServer, err := cloudmock.NewMetricTestServer()
			require.NoError(t, err)
			//nolint:errcheck
			go testServer.Serve()
			defer testServer.Shutdown()
			opts := append([]metric.Option{
				metric.WithProjectID("fakeprojectid"),
				metric.WithMonitoringClientOptions(
					apioption.WithEndpoint(testServer.Endpoint),
					apioption.WithoutAuthentication(),
					apioption.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
				)},
				test.MetricSDKExporterOptions...,
			)
			testServerExporter, err := metric.New(opts...)
			require.NoError(t, err)

			for _, m := range testcases.ConvertResourceMetrics(metrics) {
				err = testServerExporter.Export(ctx, m)
				if !test.ExpectErr {
					require.NoError(t, err, "Failed to export metrics to local test server")
				} else {
					require.Error(t, err, "Did not see expected error")
				}
			}
			require.NoError(t, testServerExporter.Shutdown(ctx))

			expectFixture := test.LoadMetricFixture(
				t,
				test.ExpectFixturePath,
				startTime,
				endTime,
			)
			sort.Slice(expectFixture.CreateTimeSeriesRequests, func(i, j int) bool {
				return expectFixture.CreateTimeSeriesRequests[i].Name < expectFixture.CreateTimeSeriesRequests[j].Name
			})
			sort.Slice(expectFixture.CreateMetricDescriptorRequests, func(i, j int) bool {
				if expectFixture.CreateMetricDescriptorRequests[i].Name != expectFixture.CreateMetricDescriptorRequests[j].Name {
					return expectFixture.CreateMetricDescriptorRequests[i].Name < expectFixture.CreateMetricDescriptorRequests[j].Name
				}
				return expectFixture.CreateMetricDescriptorRequests[i].MetricDescriptor.Name < expectFixture.CreateMetricDescriptorRequests[j].MetricDescriptor.Name
			})
			sort.Slice(expectFixture.CreateServiceTimeSeriesRequests, func(i, j int) bool {
				return expectFixture.CreateServiceTimeSeriesRequests[i].Name < expectFixture.CreateServiceTimeSeriesRequests[j].Name
			})

			// Do not test self-observability metrics with SDK exporters
			expectFixture.SelfObservabilityMetrics = nil

			fixture := &protos.MetricExpectFixture{
				CreateTimeSeriesRequests:        testServer.CreateTimeSeriesRequests(),
				CreateMetricDescriptorRequests:  testServer.CreateMetricDescriptorRequests(),
				CreateServiceTimeSeriesRequests: testServer.CreateServiceTimeSeriesRequests(),
				// Do not test self-observability metrics with SDK exporters
			}
			sort.Slice(fixture.CreateTimeSeriesRequests, func(i, j int) bool {
				return fixture.CreateTimeSeriesRequests[i].Name < fixture.CreateTimeSeriesRequests[j].Name
			})
			sort.Slice(fixture.CreateMetricDescriptorRequests, func(i, j int) bool {
				if fixture.CreateMetricDescriptorRequests[i].Name != fixture.CreateMetricDescriptorRequests[j].Name {
					return fixture.CreateMetricDescriptorRequests[i].Name < fixture.CreateMetricDescriptorRequests[j].Name
				}
				return fixture.CreateMetricDescriptorRequests[i].MetricDescriptor.Name < fixture.CreateMetricDescriptorRequests[j].MetricDescriptor.Name
			})
			sort.Slice(fixture.CreateServiceTimeSeriesRequests, func(i, j int) bool {
				return fixture.CreateServiceTimeSeriesRequests[i].Name < fixture.CreateServiceTimeSeriesRequests[j].Name
			})
			diff := DiffMetricProtos(
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

			if len(test.CompareFixturePath) > 0 {
				compareFixture := test.LoadMetricFixture(
					t,
					test.CompareFixturePath,
					startTime,
					endTime,
				)
				sort.Slice(compareFixture.CreateTimeSeriesRequests, func(i, j int) bool {
					return compareFixture.CreateTimeSeriesRequests[i].Name < compareFixture.CreateTimeSeriesRequests[j].Name
				})
				sort.Slice(compareFixture.CreateMetricDescriptorRequests, func(i, j int) bool {
					if compareFixture.CreateMetricDescriptorRequests[i].Name != compareFixture.CreateMetricDescriptorRequests[j].Name {
						return compareFixture.CreateMetricDescriptorRequests[i].Name < compareFixture.CreateMetricDescriptorRequests[j].Name
					}
					return compareFixture.CreateMetricDescriptorRequests[i].MetricDescriptor.Name < compareFixture.CreateMetricDescriptorRequests[j].MetricDescriptor.Name
				})
				sort.Slice(compareFixture.CreateServiceTimeSeriesRequests, func(i, j int) bool {
					return compareFixture.CreateServiceTimeSeriesRequests[i].Name < compareFixture.CreateServiceTimeSeriesRequests[j].Name
				})

				diff := DiffMetricProtos(
					t,
					fixture,
					compareFixture,
				)
				if diff != "" {
					require.Fail(
						t,
						"Expected requests fixture and actual GCM requests differ",
						diff,
					)
				}
			}
		})
	}
}
