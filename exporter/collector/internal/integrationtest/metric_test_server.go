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
	"net"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"google.golang.org/genproto/googleapis/api/metric"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
)

type MetricsTestServer struct {
	// Address where the gRPC server is listening
	Endpoint string

	createMetricDescriptorReqs []*monitoringpb.CreateMetricDescriptorRequest
	createTimeSeriesReqs       []*monitoringpb.CreateTimeSeriesRequest

	lis net.Listener
	srv *grpc.Server
	mu  sync.Mutex
}

func (m *MetricsTestServer) Shutdown() {
	// this will close mts.lis
	m.srv.GracefulStop()
}

// Pops out the CreateMetricDescriptorRequests which the test server has received so far
func (m *MetricsTestServer) CreateMetricDescriptorRequests() []*monitoringpb.CreateMetricDescriptorRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	reqs := m.createMetricDescriptorReqs
	m.createMetricDescriptorReqs = nil
	return reqs
}

// Pops out the CreateTimeSeriesRequests which the test server has received so far
func (m *MetricsTestServer) CreateTimeSeriesRequests() []*monitoringpb.CreateTimeSeriesRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	reqs := m.createTimeSeriesReqs
	m.createTimeSeriesReqs = nil
	return reqs
}

func (m *MetricsTestServer) appendCreateMetricDescriptorReq(req *monitoringpb.CreateMetricDescriptorRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createMetricDescriptorReqs = append(m.createMetricDescriptorReqs, req)
}
func (m *MetricsTestServer) appendCreateTimeSeriesReq(req *monitoringpb.CreateTimeSeriesRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createTimeSeriesReqs = append(m.createTimeSeriesReqs, req)
}

func (m *MetricsTestServer) Serve() error {
	return m.srv.Serve(m.lis)
}

type fakeMetricServiceServer struct {
	monitoringpb.UnimplementedMetricServiceServer
	appendCreateMetricDescriptorReq func(*monitoringpb.CreateMetricDescriptorRequest)
	appendCreateTimeSeriesReq       func(*monitoringpb.CreateTimeSeriesRequest)
}

func (f *fakeMetricServiceServer) CreateTimeSeries(
	ctx context.Context,
	req *monitoringpb.CreateTimeSeriesRequest,
) (*emptypb.Empty, error) {
	f.appendCreateTimeSeriesReq(req)
	return &emptypb.Empty{}, nil
}

func (f *fakeMetricServiceServer) CreateMetricDescriptor(
	ctx context.Context,
	req *monitoringpb.CreateMetricDescriptorRequest,
) (*metric.MetricDescriptor, error) {
	f.appendCreateMetricDescriptorReq(req)
	return &metric.MetricDescriptor{}, nil
}

func NewMetricTestServer() (*MetricsTestServer, error) {
	srv := grpc.NewServer()
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}
	testServer := &MetricsTestServer{
		Endpoint: lis.Addr().String(),
		lis:      lis,
		srv:      srv,
	}

	monitoringpb.RegisterMetricServiceServer(
		srv,
		&fakeMetricServiceServer{
			appendCreateMetricDescriptorReq: testServer.appendCreateMetricDescriptorReq,
			appendCreateTimeSeriesReq:       testServer.appendCreateTimeSeriesReq,
		},
	)

	return testServer, nil
}

func CreateConfig(factory component.ExporterFactory) *collector.Config {
	cfg := factory.CreateDefaultConfig().(*collector.Config)
	// If not set it will use ADC
	cfg.ProjectID = os.Getenv("PROJECT_ID")
	// Disable queued retries as there is no way to flush them
	cfg.RetrySettings.Enabled = false
	cfg.QueueSettings.Enabled = false
	return cfg
}

func CreateMetricsTestServerExporter(
	ctx context.Context,
	t testing.TB,
	testServer *MetricsTestServer,
) component.MetricsExporter {
	factory := collector.NewFactory()
	cfg := CreateConfig(factory)
	cfg.Endpoint = testServer.Endpoint
	cfg.UseInsecure = true
	cfg.ProjectID = "fakeprojectid"

	exporter, err := factory.CreateMetricsExporter(
		ctx,
		componenttest.NewNopExporterCreateSettings(),
		cfg,
	)
	require.NoError(t, err)
	require.NoError(t, exporter.Start(ctx, componenttest.NewNopHost()))
	t.Logf("Collector MetricsTestServer exporter started, pointing at %v", cfg.Endpoint)
	return exporter
}
