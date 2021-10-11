// Copyright 2018, OpenCensus Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stackdriver

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"go.opencensus.io/metric/metricdata"
	"google.golang.org/api/option"
	"google.golang.org/grpc"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"

	"github.com/golang/protobuf/ptypes/empty"
	timestamp "github.com/golang/protobuf/ptypes/timestamp"
	googlemetricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

func TestStatsAndMetricsEquivalence(t *testing.T) {
	startTime := time.Unix(1000, 0)
	startTimePb := &timestamp.Timestamp{Seconds: 1000}
	md := metricdata.Descriptor{
		Name:        "ocagent.io/latency",
		Description: "The latency of the various methods",
		Unit:        "ms",
		Type:        metricdata.TypeCumulativeInt64,
	}
	metricDescriptor := &metricspb.MetricDescriptor{
		Name:        "ocagent.io/latency",
		Description: "The latency of the various methods",
		Unit:        "ms",
		Type:        metricspb.MetricDescriptor_CUMULATIVE_INT64,
	}
	seenResources := make(map[*resourcepb.Resource]*monitoredrespb.MonitoredResource)

	// Generate some metricdata.Metric and metrics proto.
	var metrics []*metricdata.Metric
	var metricPbs []*metricspb.Metric
	for i := 0; i < 100; i++ {
		metric := &metricdata.Metric{
			Descriptor: md,
			TimeSeries: []*metricdata.TimeSeries{
				{
					StartTime: startTime,
					Points:    []metricdata.Point{metricdata.NewInt64Point(startTime.Add(time.Duration(1+i)*time.Second), int64(4*(i+2)))},
				},
			},
		}
		metricPb := &metricspb.Metric{
			MetricDescriptor: metricDescriptor,
			Timeseries: []*metricspb.TimeSeries{
				{
					StartTimestamp: startTimePb,
					Points: []*metricspb.Point{
						{
							Timestamp: &timestamp.Timestamp{Seconds: int64(1001 + i)},
							Value:     &metricspb.Point_Int64Value{Int64Value: int64(4 * (i + 2))},
						},
					},
				},
			},
		}
		metrics = append(metrics, metric)
		metricPbs = append(metricPbs, metricPb)
	}

	// Now perform some exporting.
	for i, metric := range metrics {
		se := &statsExporter{
			o: Options{ProjectID: "equivalence", MapResource: DefaultMapResource},
		}

		ctx := context.Background()
		sMD, err := se.metricToMpbMetricDescriptor(metric)
		if err != nil {
			t.Errorf("#%d: Stats.metricToMpbMetricDescriptor: %v", i, err)
		}
		sMDR := &monitoringpb.CreateMetricDescriptorRequest{
			Name:             fmt.Sprintf("projects/%s", se.o.ProjectID),
			MetricDescriptor: sMD,
		}
		inMD, err := se.protoToMonitoringMetricDescriptor(metricPbs[i], nil)
		if err != nil {
			t.Errorf("#%d: Stats.protoMetricDescriptorToMetricDescriptor: %v", i, err)
		}
		pMDR := &monitoringpb.CreateMetricDescriptorRequest{
			Name:             fmt.Sprintf("projects/%s", se.o.ProjectID),
			MetricDescriptor: inMD,
		}
		if diff := cmpMDReq(pMDR, sMDR); diff != "" {
			t.Fatalf("MetricDescriptor Mismatch -FromMetricsPb +FromMetrics: %s", diff)
		}

		stss, _ := se.metricToMpbTs(ctx, metric)
		sctreql := se.combineTimeSeriesToCreateTimeSeriesRequest(stss)
		allTss, _ := protoMetricToTimeSeries(ctx, se, se.getResource(nil, metricPbs[i], seenResources), metricPbs[i])
		pctreql := se.combineTimeSeriesToCreateTimeSeriesRequest(allTss)
		if diff := cmpTSReqs(pctreql, sctreql); diff != "" {
			t.Fatalf("TimeSeries Mismatch -FromMetricsPb +FromMetrics: %s", diff)
		}
	}
}

// This test creates and uses a "Stackdriver backend" which receives
// CreateTimeSeriesRequest and CreateMetricDescriptor requests
// that the Stackdriver Metrics Proto client then sends to, as it would
// send to Google Stackdriver backends.
//
// This test ensures that the final responses sent by direct stats(metricdata.Metric) exporting
// are exactly equal to those from metricdata.Metric-->OpenCensus-Proto.Metrics exporting.
func TestEquivalenceStatsVsMetricsUploads(t *testing.T) {
	server, addr, doneFn := createFakeServer(t)
	defer doneFn()

	// Now create a gRPC connection to the fake Stackdriver server.
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to make a gRPC connection to the server: %v", err)
	}
	defer conn.Close()

	// Finally create the OpenCensus stats exporter
	exporterOptions := Options{
		ProjectID:               "equivalence",
		MonitoringClientOptions: []option.ClientOption{option.WithGRPCConn(conn)},

		// Setting this time delay threshold to a very large value
		// so that batching is performed deterministically and flushing is
		// fully controlled by us.
		BundleDelayThreshold: 2 * time.Hour,
		MapResource:          DefaultMapResource,
	}
	se, err := newStatsExporter(exporterOptions)
	if err != nil {
		t.Fatalf("Failed to create the statsExporter: %v", err)
	}

	startTime := time.Unix(1000, 0)
	startTimePb := &timestamp.Timestamp{Seconds: 1000}

	// Generate the metricdata.Metric.
	metrics := []*metricdata.Metric{
		{
			Descriptor: metricdata.Descriptor{
				Name:        "ocagent.io/calls",
				Description: "The number of the various calls",
				Unit:        "1",
				Type:        metricdata.TypeCumulativeInt64,
			},
			TimeSeries: []*metricdata.TimeSeries{
				{
					StartTime: startTime,
					Points:    []metricdata.Point{metricdata.NewInt64Point(startTime.Add(time.Duration(1)*time.Second), 8)},
				},
			},
		},
		{
			Descriptor: metricdata.Descriptor{
				Name:        "ocagent.io/latency",
				Description: "The latency of the various methods",

				Unit: "ms",
				Type: metricdata.TypeCumulativeDistribution,
			},
			TimeSeries: []*metricdata.TimeSeries{
				{
					StartTime: startTime,
					Points: []metricdata.Point{
						metricdata.NewDistributionPoint(
							startTime.Add(time.Duration(2)*time.Second),
							&metricdata.Distribution{
								Count:         1,
								Sum:           125.9,
								Buckets:       []metricdata.Bucket{{Count: 0}, {Count: 1}, {Count: 0}, {Count: 0}, {Count: 0}, {Count: 0}, {Count: 0}},
								BucketOptions: &metricdata.BucketOptions{Bounds: []float64{100, 500, 1000, 2000, 4000, 8000, 16000}},
							}),
					},
				},
			},
		},
		{
			Descriptor: metricdata.Descriptor{
				Name:        "ocagent.io/connections",
				Description: "The count of various connections instantaneously",
				Unit:        "1",
				Type:        metricdata.TypeGaugeInt64,
			},
			TimeSeries: []*metricdata.TimeSeries{
				{
					Points: []metricdata.Point{metricdata.NewInt64Point(startTime.Add(time.Duration(3)*time.Second), 99)},
				},
			},
		},
		{
			Descriptor: metricdata.Descriptor{
				Name:        "ocagent.io/uptime",
				Description: "The total uptime at any instance",
				Unit:        "ms",
				Type:        metricdata.TypeCumulativeFloat64,
			},
			TimeSeries: []*metricdata.TimeSeries{
				{
					StartTime: startTime,
					Points:    []metricdata.Point{metricdata.NewFloat64Point(startTime.Add(time.Duration(1)*time.Second), 199903.97)},
				},
			},
		},
	}

	se.ExportMetrics(context.Background(), metrics)
	se.Flush()

	// Examining the stackdriver metrics that are available.
	var stackdriverTimeSeriesFromMetrics []*monitoringpb.CreateTimeSeriesRequest
	server.forEachStackdriverTimeSeries(func(sdt *monitoringpb.CreateTimeSeriesRequest) {
		stackdriverTimeSeriesFromMetrics = append(stackdriverTimeSeriesFromMetrics, sdt)
	})
	var stackdriverMetricDescriptorsFromMetrics []*monitoringpb.CreateMetricDescriptorRequest
	server.forEachStackdriverMetricDescriptor(func(sdmd *monitoringpb.CreateMetricDescriptorRequest) {
		stackdriverMetricDescriptorsFromMetrics = append(stackdriverMetricDescriptorsFromMetrics, sdmd)
	})

	// Reset the stackdriverTimeSeries to enable fresh collection
	// and then comparison with the results from metrics uploads.
	server.resetStackdriverTimeSeries()
	server.resetStackdriverMetricDescriptors()

	// Generate the proto Metrics.
	var metricPbs []*metricspb.Metric
	metricPbs = append(metricPbs,
		&metricspb.Metric{
			MetricDescriptor: &metricspb.MetricDescriptor{
				Name:        "ocagent.io/calls",
				Description: "The number of the various calls",
				Unit:        "1",
				Type:        metricspb.MetricDescriptor_CUMULATIVE_INT64,
			},
			Timeseries: []*metricspb.TimeSeries{
				{
					StartTimestamp: startTimePb,
					Points: []*metricspb.Point{
						{
							Timestamp: &timestamp.Timestamp{Seconds: int64(1001)},
							Value:     &metricspb.Point_Int64Value{Int64Value: int64(8)},
						},
					},
				},
			},
		},
		&metricspb.Metric{
			MetricDescriptor: &metricspb.MetricDescriptor{
				Name:        "ocagent.io/latency",
				Description: "The latency of the various methods",
				Unit:        "ms",
				Type:        metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION,
			},
			Timeseries: []*metricspb.TimeSeries{
				{
					StartTimestamp: startTimePb,
					Points: []*metricspb.Point{
						{
							Timestamp: &timestamp.Timestamp{Seconds: int64(1002)},
							Value: &metricspb.Point_DistributionValue{
								DistributionValue: &metricspb.DistributionValue{
									Count: 1,
									Sum:   125.9,
									BucketOptions: &metricspb.DistributionValue_BucketOptions{
										Type: &metricspb.DistributionValue_BucketOptions_Explicit_{
											Explicit: &metricspb.DistributionValue_BucketOptions_Explicit{Bounds: []float64{100, 500, 1000, 2000, 4000, 8000, 16000}},
										},
									},
									Buckets: []*metricspb.DistributionValue_Bucket{{Count: 0}, {Count: 1}, {Count: 0}, {Count: 0}, {Count: 0}, {Count: 0}, {Count: 0}},
								},
							},
						},
					},
				},
			},
		},
		&metricspb.Metric{
			MetricDescriptor: &metricspb.MetricDescriptor{
				Name:        "ocagent.io/connections",
				Description: "The count of various connections instantaneously",
				Unit:        "1",
				Type:        metricspb.MetricDescriptor_GAUGE_INT64,
			},
			Timeseries: []*metricspb.TimeSeries{
				{
					StartTimestamp: startTimePb,
					Points: []*metricspb.Point{
						{
							Timestamp: &timestamp.Timestamp{Seconds: int64(1003)},
							Value:     &metricspb.Point_Int64Value{Int64Value: 99},
						},
					},
				},
			},
		},
		&metricspb.Metric{
			MetricDescriptor: &metricspb.MetricDescriptor{
				Name:        "ocagent.io/uptime",
				Description: "The total uptime at any instance",
				Unit:        "ms",
				Type:        metricspb.MetricDescriptor_CUMULATIVE_DOUBLE,
			},
			Timeseries: []*metricspb.TimeSeries{
				{
					StartTimestamp: startTimePb,
					Points: []*metricspb.Point{
						{
							Timestamp: &timestamp.Timestamp{Seconds: int64(1001)},
							Value:     &metricspb.Point_DoubleValue{DoubleValue: 199903.97},
						},
					},
				},
			},
		})

	// Export the proto Metrics to the Stackdriver backend.
	se.PushMetricsProto(context.Background(), nil, nil, metricPbs)
	se.Flush()

	var stackdriverTimeSeriesFromMetricsPb []*monitoringpb.CreateTimeSeriesRequest
	server.forEachStackdriverTimeSeries(func(sdt *monitoringpb.CreateTimeSeriesRequest) {
		stackdriverTimeSeriesFromMetricsPb = append(stackdriverTimeSeriesFromMetricsPb, sdt)
	})
	var stackdriverMetricDescriptorsFromMetricsPb []*monitoringpb.CreateMetricDescriptorRequest
	server.forEachStackdriverMetricDescriptor(func(sdmd *monitoringpb.CreateMetricDescriptorRequest) {
		stackdriverMetricDescriptorsFromMetricsPb = append(stackdriverMetricDescriptorsFromMetricsPb, sdmd)
	})

	if len(stackdriverTimeSeriesFromMetrics) == 0 {
		t.Fatalf("Failed to export timeseries with metrics")
	}

	if len(stackdriverTimeSeriesFromMetricsPb) == 0 {
		t.Fatalf("Failed to export timeseries with metrics pb")
	}

	// The results should be equal now
	if diff := cmpTSReqs(stackdriverTimeSeriesFromMetricsPb, stackdriverTimeSeriesFromMetrics); diff != "" {
		t.Fatalf("Unexpected CreateTimeSeriesRequests -FromMetricsPb +FromMetrics: %s", diff)
	}

	// Examining the metric descriptors too.
	if diff := cmpMDReqs(stackdriverMetricDescriptorsFromMetricsPb, stackdriverMetricDescriptorsFromMetrics); diff != "" {
		t.Fatalf("Unexpected CreateMetricDescriptorRequests -FromMetricsPb +FromMetrics: %s", diff)
	}
}

type fakeMetricsServer struct {
	monitoringpb.MetricServiceServer
	mu                           sync.RWMutex
	stackdriverTimeSeries        []*monitoringpb.CreateTimeSeriesRequest
	stackdriverMetricDescriptors []*monitoringpb.CreateMetricDescriptorRequest
}

func createFakeServer(t *testing.T) (*fakeMetricsServer, string, func()) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to bind to an available address: %v", err)
	}
	server := new(fakeMetricsServer)
	srv := grpc.NewServer()
	monitoringpb.RegisterMetricServiceServer(srv, server)
	go func() {
		_ = srv.Serve(ln)
	}()
	stop := func() {
		srv.Stop()
		_ = ln.Close()
	}
	_, agentPortStr, _ := net.SplitHostPort(ln.Addr().String())
	return server, "localhost:" + agentPortStr, stop
}

func (server *fakeMetricsServer) forEachStackdriverTimeSeries(fn func(sdt *monitoringpb.CreateTimeSeriesRequest)) {
	server.mu.RLock()
	defer server.mu.RUnlock()

	for _, sdt := range server.stackdriverTimeSeries {
		fn(sdt)
	}
}

func (server *fakeMetricsServer) forEachStackdriverMetricDescriptor(fn func(sdmd *monitoringpb.CreateMetricDescriptorRequest)) {
	server.mu.RLock()
	defer server.mu.RUnlock()

	for _, sdmd := range server.stackdriverMetricDescriptors {
		fn(sdmd)
	}
}

func (server *fakeMetricsServer) resetStackdriverTimeSeries() {
	server.mu.Lock()
	server.stackdriverTimeSeries = server.stackdriverTimeSeries[:0]
	server.mu.Unlock()
}

func (server *fakeMetricsServer) resetStackdriverMetricDescriptors() {
	server.mu.Lock()
	server.stackdriverMetricDescriptors = server.stackdriverMetricDescriptors[:0]
	server.mu.Unlock()
}

func (server *fakeMetricsServer) CreateMetricDescriptor(ctx context.Context, req *monitoringpb.CreateMetricDescriptorRequest) (*googlemetricpb.MetricDescriptor, error) {
	server.mu.Lock()
	server.stackdriverMetricDescriptors = append(server.stackdriverMetricDescriptors, req)
	server.mu.Unlock()
	return req.MetricDescriptor, nil
}

func (server *fakeMetricsServer) CreateTimeSeries(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest) (*empty.Empty, error) {
	server.mu.Lock()
	server.stackdriverTimeSeries = append(server.stackdriverTimeSeries, req)
	server.mu.Unlock()
	return new(empty.Empty), nil
}
