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
	for {
		select {
		case req := <-m.createMetricDescriptorChan:
			reqs = append(reqs, req)
		default:
			return reqs
		}
	}
}

// Pops out the CreateTimeSeriesRequests which the test server has received so far
func (m *MetricsTestServer) CreateTimeSeriesRequests() []*monitoringpb.CreateTimeSeriesRequest {
	reqs := []*monitoringpb.CreateTimeSeriesRequest{}
	for {
		select {
		case req := <-m.createTimeSeriesChan:
			reqs = append(reqs, req)
		default:
			return reqs
		}
	}
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
