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

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	logpb "google.golang.org/genproto/googleapis/logging/v2"
	"google.golang.org/grpc"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
)

type LogsTestServer struct {
	// Address where the gRPC server is listening
	Endpoint string

	writeLogEntriesRequests []*logpb.WriteLogEntriesRequest

	lis net.Listener
	srv *grpc.Server
	mu  sync.Mutex
}

func (l *LogsTestServer) Shutdown() {
	l.srv.GracefulStop()
}

func (l *LogsTestServer) Serve() {
	l.srv.Serve(l.lis)
}

func (l *LogsTestServer) CreateWriteLogEntriesRequests() []*logpb.WriteLogEntriesRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	reqs := l.writeLogEntriesRequests
	l.writeLogEntriesRequests = nil
	return reqs
}

func (l *LogsTestServer) appendWriteLogEntriesRequest(req *logpb.WriteLogEntriesRequest) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writeLogEntriesRequests = append(l.writeLogEntriesRequests, req)
}

type fakeLoggingServiceServer struct {
	logpb.UnimplementedLoggingServiceV2Server
	logsTestServer *LogsTestServer
}

func (f *fakeLoggingServiceServer) WriteLogEntries(
	ctx context.Context,
	request *logpb.WriteLogEntriesRequest,
) (*logpb.WriteLogEntriesResponse, error) {
	f.logsTestServer.appendWriteLogEntriesRequest(request)
	return &logpb.WriteLogEntriesResponse{}, nil
}

func NewLoggingTestServer() (*LogsTestServer, error) {
	srv := grpc.NewServer()
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}
	testServer := &LogsTestServer{
		Endpoint: lis.Addr().String(),
		lis:      lis,
		srv:      srv,
	}
	logpb.RegisterLoggingServiceV2Server(
		srv,
		&fakeLoggingServiceServer{logsTestServer: testServer},
	)

	return testServer, nil
}

func (l *LogsTestServer) NewExporter(
	ctx context.Context,
	t testing.TB,
	cfg collector.Config,
) *collector.LogsExporter {

	cfg.LogConfig.ClientConfig.Endpoint = l.Endpoint
	cfg.LogConfig.ClientConfig.UseInsecure = true
	cfg.ProjectID = "fakeprojectid"

	exporter, err := collector.NewGoogleCloudLogsExporter(
		ctx,
		cfg,
		zap.NewNop(),
	)
	require.NoError(t, err)
	t.Logf("Collector LogsTestServer exporter started, pointing at %v", cfg.LogConfig.ClientConfig.Endpoint)
	return exporter
}
