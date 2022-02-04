// Copyright 2021 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collector

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	cloudmetricpb "google.golang.org/genproto/googleapis/api/metric"
	cloudtracepb "google.golang.org/genproto/googleapis/devtools/cloudtrace/v2"
	cloudmonitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/metricstestutil"
)

type testServer struct {
	reqCh chan *cloudtracepb.BatchWriteSpansRequest
}

func (ts *testServer) BatchWriteSpans(ctx context.Context, req *cloudtracepb.BatchWriteSpansRequest) (*emptypb.Empty, error) {
	go func() { ts.reqCh <- req }()
	return &emptypb.Empty{}, nil
}

// Creates a new span.
func (ts *testServer) CreateSpan(context.Context, *cloudtracepb.Span) (*cloudtracepb.Span, error) {
	return nil, nil
}

func TestGoogleCloudTraceExport(t *testing.T) {
	type testCase struct {
		name        string
		cfg         Config
		expectedErr string
	}

	testCases := []testCase{
		{
			name: "Standard",
			cfg: Config{
				ProjectID: "idk",
				TraceConfig: TraceConfig{
					ClientConfig: ClientConfig{
						Endpoint:    "127.0.0.1:8080",
						UseInsecure: true,
					},
				},
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			srv := grpc.NewServer()
			reqCh := make(chan *cloudtracepb.BatchWriteSpansRequest)
			cloudtracepb.RegisterTraceServiceServer(srv, &testServer{reqCh: reqCh})

			lis, err := net.Listen("tcp", "localhost:8080")
			require.NoError(t, err)
			defer lis.Close()

			go srv.Serve(lis)
			sde, err := NewGoogleCloudTracesExporter(ctx, test.cfg, "latest", DefaultTimeout)
			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)
				return
			}
			require.NoError(t, err)
			defer func() { require.NoError(t, sde.Shutdown(ctx)) }()

			testTime := time.Now()
			spanName := "foobar"

			resource := pdata.NewResource()
			traces := pdata.NewTraces()
			rspans := traces.ResourceSpans().AppendEmpty()
			resource.CopyTo(rspans.Resource())
			ispans := rspans.InstrumentationLibrarySpans().AppendEmpty()
			span := ispans.Spans().AppendEmpty()
			span.SetName(spanName)
			span.SetStartTimestamp(pdata.NewTimestampFromTime(testTime))
			err = sde.PushTraces(ctx, traces)
			assert.NoError(t, err)

			r := <-reqCh
			assert.Len(t, r.Spans, 1)
			assert.Equal(t, spanName, r.Spans[0].GetDisplayName().Value)
			assert.Equal(t, timestamppb.New(testTime), r.Spans[0].StartTime)
		})
	}
}

type mockMetricServer struct {
	cloudmonitoringpb.MetricServiceServer

	descriptorReqCh chan *requestWithMetadata
	timeSeriesReqCh chan *requestWithMetadata
}

type requestWithMetadata struct {
	req      interface{}
	metadata metadata.MD
}

func (ms *mockMetricServer) CreateMetricDescriptor(ctx context.Context, req *cloudmonitoringpb.CreateMetricDescriptorRequest) (*cloudmetricpb.MetricDescriptor, error) {
	reqWithMetadata := &requestWithMetadata{req: req}
	metadata, ok := metadata.FromIncomingContext(ctx)
	if ok {
		reqWithMetadata.metadata = metadata
	}

	go func() { ms.descriptorReqCh <- reqWithMetadata }()
	return &cloudmetricpb.MetricDescriptor{}, nil
}

func (ms *mockMetricServer) CreateTimeSeries(ctx context.Context, req *cloudmonitoringpb.CreateTimeSeriesRequest) (*emptypb.Empty, error) {
	reqWithMetadata := &requestWithMetadata{req: req}
	metadata, ok := metadata.FromIncomingContext(ctx)
	if ok {
		reqWithMetadata.metadata = metadata
	}

	go func() { ms.timeSeriesReqCh <- reqWithMetadata }()
	return &emptypb.Empty{}, nil
}

func TestGoogleCloudMetricExport(t *testing.T) {
	srv := grpc.NewServer()

	descriptorReqCh := make(chan *requestWithMetadata)
	timeSeriesReqCh := make(chan *requestWithMetadata)

	mockServer := &mockMetricServer{descriptorReqCh: descriptorReqCh, timeSeriesReqCh: timeSeriesReqCh}
	cloudmonitoringpb.RegisterMetricServiceServer(srv, mockServer)

	lis, err := net.Listen("tcp", "localhost:8080")
	require.NoError(t, err)
	defer lis.Close()

	go srv.Serve(lis)

	config := DefaultConfig()
	config.ProjectID = "idk"
	config.MetricConfig.ClientConfig.Endpoint = "127.0.0.1:8080"
	config.UserAgent = "MyAgent {{version}}"
	config.MetricConfig.ClientConfig.UseInsecure = true
	config.MetricConfig.InstrumentationLibraryLabels = false
	config.MetricConfig.ClientConfig.GetClientOptions = func() []option.ClientOption {
		// Example with overridden client options
		return []option.ClientOption{
			option.WithoutAuthentication(),
			option.WithTelemetryDisabled(),
		}
	}

	sde, err := NewGoogleCloudMetricsExporter(context.Background(), config, zap.NewNop(), "v0.0.1", DefaultTimeout)
	shutdownOnce := sync.Once{}
	shutdown := func() {
		require.NoError(t, sde.Shutdown(context.Background()))
	}
	require.NoError(t, err)
	defer shutdownOnce.Do(shutdown)

	md := agentmetricspb.ExportMetricsServiceRequest{
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				"cloud.platform":          "gcp_kubernetes_engine",
				"cloud.availability_zone": "us-central1",
				"k8s.node.name":           "foo",
				"k8s.cluster.name":        "test",
			},
		},
		Metrics: []*metricspb.Metric{
			metricstestutil.Gauge(
				"test_gauge1",
				[]string{"k0"},
				metricstestutil.Timeseries(
					time.Now(),
					[]string{"v0"},
					metricstestutil.Double(time.Now(), 1))),
			metricstestutil.Gauge(
				"test_gauge2",
				[]string{"k0", "k1"},
				metricstestutil.Timeseries(
					time.Now(),
					[]string{"v0", "v1"},
					metricstestutil.Double(time.Now(), 12))),
			metricstestutil.Gauge(
				"test_gauge3",
				[]string{"k0", "k1", "k2"},
				metricstestutil.Timeseries(
					time.Now(),
					[]string{"v0", "v1", "v2"},
					metricstestutil.Double(time.Now(), 123))),
			metricstestutil.Gauge(
				"test_gauge4",
				[]string{"k0", "k1", "k2", "k3"},
				metricstestutil.Timeseries(
					time.Now(),
					[]string{"v0", "v1", "v2", "v3"},
					metricstestutil.Double(time.Now(), 1234))),
			metricstestutil.Gauge(
				"test_gauge5",
				[]string{"k4", "k5"},
				metricstestutil.Timeseries(
					time.Now(),
					[]string{"v4", "v5"},
					metricstestutil.Double(time.Now(), 34))),
		},
	}
	md.Metrics[2].Resource = &resourcepb.Resource{
		Labels: map[string]string{
			"cloud.platform":          "gcp_kubernetes_engine",
			"cloud.availability_zone": "us-central1",
			"k8s.node.name":           "bar",
			"k8s.cluster.name":        "test",
		},
	}
	md.Metrics[3].Resource = &resourcepb.Resource{}
	md.Metrics[4].Resource = &resourcepb.Resource{}

	assert.NoError(t, sde.PushMetrics(context.Background(), internaldata.OCToMetrics(md.Node, md.Resource, md.Metrics)), err)
	shutdownOnce.Do(shutdown)

	expectedNames := map[string]struct{}{
		"workload.googleapis.com/test_gauge1": {},
		"workload.googleapis.com/test_gauge2": {},
		"workload.googleapis.com/test_gauge3": {},
		"workload.googleapis.com/test_gauge4": {},
		"workload.googleapis.com/test_gauge5": {},
	}
	for i := 0; i < 5; i++ {
		drm := <-descriptorReqCh
		assert.Regexp(t, "MyAgent v0\\.0\\.1", drm.metadata["user-agent"])
		dr := drm.req.(*cloudmonitoringpb.CreateMetricDescriptorRequest)
		assert.Contains(t, expectedNames, dr.MetricDescriptor.Type)
		delete(expectedNames, dr.MetricDescriptor.Type)
	}

	trm := <-timeSeriesReqCh
	assert.Regexp(t, "MyAgent v0\\.0\\.1", trm.metadata["user-agent"])
	tr := trm.req.(*cloudmonitoringpb.CreateTimeSeriesRequest)
	require.Len(t, tr.TimeSeries, 5)

	resourceFoo := map[string]string{
		"node_name":    "foo",
		"cluster_name": "test",
		"location":     "us-central1",
	}

	resourceBar := map[string]string{
		"node_name":    "bar",
		"cluster_name": "test",
		"location":     "us-central1",
	}

	// We no longer map to global resource by default.
	// Instead, we map to generic_node
	resourceDefault := map[string]string{
		"location":  "global",
		"namespace": "",
		"node_id":   "",
	}

	expectedTimeSeries := map[string]struct {
		value          float64
		labels         map[string]string
		resourceLabels map[string]string
	}{
		"workload.googleapis.com/test_gauge1": {
			value:          float64(1),
			labels:         map[string]string{"k0": "v0"},
			resourceLabels: resourceFoo,
		},
		"workload.googleapis.com/test_gauge2": {
			value:          float64(12),
			labels:         map[string]string{"k0": "v0", "k1": "v1"},
			resourceLabels: resourceFoo,
		},
		"workload.googleapis.com/test_gauge3": {
			value:          float64(123),
			labels:         map[string]string{"k0": "v0", "k1": "v1", "k2": "v2"},
			resourceLabels: resourceBar,
		},
		"workload.googleapis.com/test_gauge4": {
			value:          float64(1234),
			labels:         map[string]string{"k0": "v0", "k1": "v1", "k2": "v2", "k3": "v3"},
			resourceLabels: resourceDefault,
		},
		"workload.googleapis.com/test_gauge5": {
			value:          float64(34),
			labels:         map[string]string{"k4": "v4", "k5": "v5"},
			resourceLabels: resourceDefault,
		},
	}
	for i := 0; i < 5; i++ {
		require.Contains(t, expectedTimeSeries, tr.TimeSeries[i].Metric.Type)
		ts := expectedTimeSeries[tr.TimeSeries[i].Metric.Type]
		assert.Equal(t, ts.labels, tr.TimeSeries[i].Metric.Labels)
		require.Len(t, tr.TimeSeries[i].Points, 1)
		assert.Equal(t, ts.value, tr.TimeSeries[i].Points[0].Value.GetDoubleValue())
		assert.Equal(t, ts.resourceLabels, tr.TimeSeries[i].Resource.Labels)
	}
}
