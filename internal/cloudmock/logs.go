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
	"sync"

	logpb "cloud.google.com/go/logging/apiv2/loggingpb"
	"google.golang.org/grpc"
)

type LogsTestServer struct {
	lis net.Listener
	srv *grpc.Server
	// Endpoint where the gRPC server is listening
	Endpoint                string
	writeLogEntriesRequests []*logpb.WriteLogEntriesRequest
	mu                      sync.Mutex
}

func (l *LogsTestServer) Shutdown() {
	l.srv.GracefulStop()
}

func (l *LogsTestServer) Serve() {
	//nolint:errcheck
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
