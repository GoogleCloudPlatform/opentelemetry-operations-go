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

package integrationtest

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/stretchr/testify/require"
	tracepb "google.golang.org/genproto/googleapis/devtools/cloudtrace/v2"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type TracesTestServer struct {
	lis net.Listener
	srv *grpc.Server
	// Endpoint where the gRPC server is listening
	Endpoint                string
	batchWriteSpansRequests []*tracepb.BatchWriteSpansRequest
	mu                      sync.Mutex
}

func (t *TracesTestServer) Shutdown() {
	t.srv.GracefulStop()
}

func (t *TracesTestServer) Serve() {
	t.srv.Serve(t.lis)
}

type fakeTraceServiceServer struct {
	tracepb.UnimplementedTraceServiceServer
	tracesTestServer *TracesTestServer
}

func (f *fakeTraceServiceServer) BatchWriteSpans(
	ctx context.Context,
	request *tracepb.BatchWriteSpansRequest,
) (*emptypb.Empty, error) {
	f.tracesTestServer.appendBatchWriteSpansRequest(request)
	return &emptypb.Empty{}, nil
}

func (t *TracesTestServer) appendBatchWriteSpansRequest(req *tracepb.BatchWriteSpansRequest) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.batchWriteSpansRequests = append(t.batchWriteSpansRequests, req)
}

func (t *TracesTestServer) CreateBatchWriteSpansRequests() []*tracepb.BatchWriteSpansRequest {
	t.mu.Lock()
	defer t.mu.Unlock()
	reqs := t.batchWriteSpansRequests
	t.batchWriteSpansRequests = nil
	return reqs
}

func NewTracesTestServer() (*TracesTestServer, error) {
	srv := grpc.NewServer()
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}
	testServer := &TracesTestServer{
		Endpoint: lis.Addr().String(),
		lis:      lis,
		srv:      srv,
	}
	tracepb.RegisterTraceServiceServer(
		srv,
		&fakeTraceServiceServer{tracesTestServer: testServer},
	)

	return testServer, nil
}

func (s *TracesTestServer) NewExporter(
	ctx context.Context,
	t testing.TB,
	cfg collector.Config,
) *collector.TraceExporter {

	cfg.TraceConfig.ClientConfig.Endpoint = s.Endpoint
	cfg.TraceConfig.ClientConfig.UseInsecure = true
	cfg.ProjectID = "fakeprojectid"

	exporter, err := collector.NewGoogleCloudTracesExporter(
		ctx,
		cfg,
		"latest",
		collector.DefaultTimeout,
	)
	require.NoError(t, err)
	t.Logf("Collector TracesTestServer exporter started, pointing at %v", cfg.TraceConfig.ClientConfig.Endpoint)
	return exporter
}
