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
	"time"

	"cloud.google.com/go/trace/apiv2/tracepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

type TracesTestServer struct {
	lis net.Listener
	srv *grpc.Server
	// Endpoint where the gRPC server is listening
	Endpoint                string
	batchWriteSpansRequests []*tracepb.BatchWriteSpansRequest
	mu                      sync.Mutex
	Retries                 int
	userAgent               string
}

func (t *TracesTestServer) Shutdown() {
	t.srv.GracefulStop()
}

func (t *TracesTestServer) Serve() {
	//nolint:errcheck
	t.srv.Serve(t.lis)
}

type fakeTraceServiceServer struct {
	tracepb.UnimplementedTraceServiceServer
	tracesTestServer *TracesTestServer
	cfg              config
}

func (f *fakeTraceServiceServer) BatchWriteSpans(
	ctx context.Context,
	request *tracepb.BatchWriteSpansRequest,
) (*emptypb.Empty, error) {
	time.Sleep(f.cfg.delay)
	if f.cfg.responseErr != nil {
		f.tracesTestServer.Retries++
		return &emptypb.Empty{}, f.cfg.responseErr
	}
	f.tracesTestServer.appendBatchWriteSpansRequest(ctx, request)
	return &emptypb.Empty{}, nil
}

func (t *TracesTestServer) appendBatchWriteSpansRequest(ctx context.Context, req *tracepb.BatchWriteSpansRequest) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.batchWriteSpansRequests = append(t.batchWriteSpansRequests, req)
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		t.userAgent = strings.Join(md.Get("User-Agent"), ";")
	}
}

func (t *TracesTestServer) CreateBatchWriteSpansRequests() []*tracepb.BatchWriteSpansRequest {
	t.mu.Lock()
	defer t.mu.Unlock()
	reqs := t.batchWriteSpansRequests
	t.batchWriteSpansRequests = nil
	return reqs
}

// Pops out the UserAgent from the most recent BatchWriteSpans
func (t *TracesTestServer) UserAgent() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	ua := t.userAgent
	t.userAgent = ""
	return ua
}

func NewTracesTestServer(opts ...TraceServerOption) (*TracesTestServer, error) {
	srv := grpc.NewServer()
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}
	testServer := &TracesTestServer{
		Endpoint: lis.Addr().String(),
		Retries:  0,
		lis:      lis,
		srv:      srv,
	}
	var c config
	for _, option := range opts {
		c = option.apply(c)
	}
	tracepb.RegisterTraceServiceServer(
		srv,
		&fakeTraceServiceServer{tracesTestServer: testServer, cfg: c},
	)

	return testServer, nil
}

type TraceServerOption interface {
	apply(config) config
}

type config struct {
	responseErr error
	delay       time.Duration
}

type optionFunc func(config) config

func (fn optionFunc) apply(cfg config) config {
	return fn(cfg)
}

// WithDelay sets a delay on the test server before it responds.
func WithDelay(t time.Duration) TraceServerOption {
	return optionFunc(func(cfg config) config {
		cfg.delay = t
		return cfg
	})
}

func WithErrorResponse(err error) TraceServerOption {
	return optionFunc(func(cfg config) config {
		cfg.responseErr = err
		return cfg
	})
}
