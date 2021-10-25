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

	"google.golang.org/genproto/googleapis/api/metric"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type MetricsTestServer struct {
	// Address where the gRPC server is listening
	Endpoint string

	createMetricDescriptorChan <-chan *monitoringpb.CreateMetricDescriptorRequest
	createTimeSeriesChan       <-chan *monitoringpb.CreateTimeSeriesRequest
	lis                        net.Listener
	srv                        *grpc.Server
}

func (m *MetricsTestServer) Shutdown() {
	// this will close mts.lis
	m.srv.GracefulStop()
}

// Pops out the CreateMetricDescriptorRequests which the test server has received so far
func (m *MetricsTestServer) CreateMetricDescriptorRequests() []*monitoringpb.CreateMetricDescriptorRequest {
	reqs := []*monitoringpb.CreateMetricDescriptorRequest{}
	for hasItems := true; hasItems; {
		select {
		case req := <-m.createMetricDescriptorChan:
			reqs = append(reqs, req)
		default:
			hasItems = false
		}
	}
	return reqs
}

// Pops out the CreateTimeSeriesRequests which the test server has received so far
func (m *MetricsTestServer) CreateTimeSeriesRequests() []*monitoringpb.CreateTimeSeriesRequest {
	reqs := []*monitoringpb.CreateTimeSeriesRequest{}
	for hasItems := true; hasItems; {
		select {
		case req := <-m.createTimeSeriesChan:
			reqs = append(reqs, req)
		default:
			hasItems = false
		}
	}
	return reqs
}

func (m *MetricsTestServer) Serve() error {
	return m.srv.Serve(m.lis)
}

type fakeMetricServiceServer struct {
	monitoringpb.UnimplementedMetricServiceServer
	createMetricDescriptorChan chan<- *monitoringpb.CreateMetricDescriptorRequest
	createTimeSeriesChan       chan<- *monitoringpb.CreateTimeSeriesRequest
}

func (f *fakeMetricServiceServer) CreateTimeSeries(
	ctx context.Context,
	req *monitoringpb.CreateTimeSeriesRequest,
) (*emptypb.Empty, error) {
	go func() { f.createTimeSeriesChan <- req }()
	return &emptypb.Empty{}, nil
}

func (f *fakeMetricServiceServer) CreateMetricDescriptor(
	ctx context.Context,
	req *monitoringpb.CreateMetricDescriptorRequest,
) (*metric.MetricDescriptor, error) {
	go func() { f.createMetricDescriptorChan <- req }()
	return &metric.MetricDescriptor{}, nil
}

func NewMetricTestServer() (*MetricsTestServer, error) {
	srv := grpc.NewServer()
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}
	createMetricDescriptorCh := make(chan *monitoringpb.CreateMetricDescriptorRequest, 10)
	createTimeSeriesCh := make(chan *monitoringpb.CreateTimeSeriesRequest, 10)
	monitoringpb.RegisterMetricServiceServer(
		srv,
		&fakeMetricServiceServer{
			createMetricDescriptorChan: createMetricDescriptorCh,
			createTimeSeriesChan:       createTimeSeriesCh,
		},
	)

	testServer := &MetricsTestServer{
		Endpoint:                   lis.Addr().String(),
		createMetricDescriptorChan: createMetricDescriptorCh,
		createTimeSeriesChan:       createTimeSeriesCh,
		lis:                        lis,
		srv:                        srv,
	}

	return testServer, nil
}
