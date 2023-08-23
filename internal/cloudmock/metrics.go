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

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type MetricsTestServer struct {
	lis                         net.Listener
	srv                         *grpc.Server
	Endpoint                    string
	userAgent                   string
	createMetricDescriptorReqs  []*monitoringpb.CreateMetricDescriptorRequest
	createTimeSeriesReqs        []*monitoringpb.CreateTimeSeriesRequest
	createServiceTimeSeriesReqs []*monitoringpb.CreateTimeSeriesRequest
	RetryCount                  int
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

// Pops out the UserAgent from the most recent CreateTimeSeriesRequests or CreateServiceTimeSeriesRequests.
func (m *MetricsTestServer) UserAgent() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ua := m.userAgent
	m.userAgent = ""
	return ua
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

func (m *MetricsTestServer) appendCreateMetricDescriptorReq(ctx context.Context, req *monitoringpb.CreateMetricDescriptorRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createMetricDescriptorReqs = append(m.createMetricDescriptorReqs, req)
}
func (m *MetricsTestServer) appendCreateTimeSeriesReq(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createTimeSeriesReqs = append(m.createTimeSeriesReqs, req)
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		m.userAgent = strings.Join(md.Get("User-Agent"), ";")
	}
}
func (m *MetricsTestServer) appendCreateServiceTimeSeriesReq(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createServiceTimeSeriesReqs = append(m.createServiceTimeSeriesReqs, req)
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		m.userAgent = strings.Join(md.Get("User-Agent"), ";")
	}
}

func (m *MetricsTestServer) Serve() error {
	return m.srv.Serve(m.lis)
}

type fakeMetricServiceServer struct {
	monitoringpb.UnimplementedMetricServiceServer
	metricsTestServer *MetricsTestServer
}

// CreateTimeSeries simulates a call to GCM.
// Failed calls can be simulated by putting error codes in the project name for the request,
// such as "notfound", "unavailable", and "deadline_exceeded".
// For WAL testing, unavailable and deadline exceeded calls will retry once, failing the first
// time but succeeding on the second call (eg, network outage).
func (f *fakeMetricServiceServer) CreateTimeSeries(
	ctx context.Context,
	req *monitoringpb.CreateTimeSeriesRequest,
) (*emptypb.Empty, error) {
	code := codes.OK
	if strings.Contains(req.Name, "notfound") {
		code = codes.NotFound
	} else if strings.Contains(req.Name, "unavailable") && f.metricsTestServer.RetryCount == 0 {
		f.metricsTestServer.RetryCount++
		code = codes.Unavailable
	} else if strings.Contains(req.Name, "deadline_exceeded") && f.metricsTestServer.RetryCount == 0 {
		f.metricsTestServer.RetryCount++
		code = codes.DeadlineExceeded
	}

	successPointCount := int32(len(req.TimeSeries))
	if code == codes.NotFound || code == codes.Unavailable || code == codes.DeadlineExceeded {
		successPointCount = 0
	} else {
		f.metricsTestServer.appendCreateTimeSeriesReq(ctx, req)
	}

	statusResp, _ := status.New(code, "").WithDetails(
		&monitoringpb.CreateTimeSeriesSummary{
			TotalPointCount:   int32(len(req.TimeSeries)),
			SuccessPointCount: successPointCount,
		})

	return &emptypb.Empty{}, statusResp.Err()
}

func (f *fakeMetricServiceServer) CreateServiceTimeSeries(
	ctx context.Context,
	req *monitoringpb.CreateTimeSeriesRequest,
) (*emptypb.Empty, error) {
	f.metricsTestServer.appendCreateServiceTimeSeriesReq(ctx, req)
	return &emptypb.Empty{}, nil
}

func (f *fakeMetricServiceServer) CreateMetricDescriptor(
	ctx context.Context,
	req *monitoringpb.CreateMetricDescriptorRequest,
) (*metricpb.MetricDescriptor, error) {
	f.metricsTestServer.appendCreateMetricDescriptorReq(ctx, req)
	return &metricpb.MetricDescriptor{}, nil
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
