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

package cloudmock

import (
	"context"
	"net"
	"strings"
	"sync"

	"google.golang.org/genproto/googleapis/api/metric"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type MetricsTestServer struct {
	lis                         net.Listener
	srv                         *grpc.Server
	Endpoint                    string
	createMetricDescriptorReqs  []*monitoringpb.CreateMetricDescriptorRequest
	createTimeSeriesReqs        []*monitoringpb.CreateTimeSeriesRequest
	createServiceTimeSeriesReqs []*monitoringpb.CreateTimeSeriesRequest
	mu                          sync.Mutex
}

func (m *MetricsTestServer) Shutdown() {
	// this will close mts.lis
	m.srv.GracefulStop()
}

// Pops out the CreateMetricDescriptorRequests which the test server has received so far.
func (m *MetricsTestServer) CreateMetricDescriptorRequests() []*monitoringpb.CreateMetricDescriptorRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	reqs := m.createMetricDescriptorReqs
	m.createMetricDescriptorReqs = nil
	return reqs
}

// Pops out the CreateTimeSeriesRequests which the test server has received so far.
func (m *MetricsTestServer) CreateTimeSeriesRequests() []*monitoringpb.CreateTimeSeriesRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	reqs := m.createTimeSeriesReqs
	m.createTimeSeriesReqs = nil
	return reqs
}

// Pops out the CreateServiceTimeSeriesRequests which the test server has received so far.
func (m *MetricsTestServer) CreateServiceTimeSeriesRequests() []*monitoringpb.CreateTimeSeriesRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	reqs := m.createServiceTimeSeriesReqs
	m.createServiceTimeSeriesReqs = nil
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
func (m *MetricsTestServer) appendCreateServiceTimeSeriesReq(req *monitoringpb.CreateTimeSeriesRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createServiceTimeSeriesReqs = append(m.createServiceTimeSeriesReqs, req)
}

func (m *MetricsTestServer) Serve() error {
	return m.srv.Serve(m.lis)
}

type fakeMetricServiceServer struct {
	monitoringpb.UnimplementedMetricServiceServer
	metricsTestServer *MetricsTestServer
}

func (f *fakeMetricServiceServer) CreateTimeSeries(
	ctx context.Context,
	req *monitoringpb.CreateTimeSeriesRequest,
) (*emptypb.Empty, error) {
	f.metricsTestServer.appendCreateTimeSeriesReq(req)

	var code codes.Code
	if strings.Contains(req.Name, "notfound") {
		code = codes.NotFound
	}

	statusResp := status.New(code, "FORCED_TEST_ERROR")
	if code == codes.NotFound {
		statusResp, _ = statusResp.WithDetails(&monitoringpb.CreateTimeSeriesSummary{
			TotalPointCount:   int32(len(req.TimeSeries)),
			SuccessPointCount: 0,
		})
	}

	return &emptypb.Empty{}, statusResp.Err()
}

func (f *fakeMetricServiceServer) CreateServiceTimeSeries(
	ctx context.Context,
	req *monitoringpb.CreateTimeSeriesRequest,
) (*emptypb.Empty, error) {
	f.metricsTestServer.appendCreateServiceTimeSeriesReq(req)
	return &emptypb.Empty{}, nil
}

func (f *fakeMetricServiceServer) CreateMetricDescriptor(
	ctx context.Context,
	req *monitoringpb.CreateMetricDescriptorRequest,
) (*metric.MetricDescriptor, error) {
	f.metricsTestServer.appendCreateMetricDescriptorReq(req)
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
		&fakeMetricServiceServer{metricsTestServer: testServer},
	)

	return testServer, nil
}
