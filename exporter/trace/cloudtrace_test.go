// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package trace_test

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	emptypb "github.com/golang/protobuf/ptypes/empty"
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/option"
	tracepb "google.golang.org/genproto/googleapis/devtools/cloudtrace/v2"
	"google.golang.org/grpc"

	"go.opentelemetry.io/otel/api/global"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type mockTraceServer struct {
	tracepb.TraceServiceServer
	mu            sync.Mutex
	spansUploaded []*tracepb.Span
	delay         time.Duration
	// onUpload is called every time BatchWriteSpans is called
	onUpload      func(ctx context.Context, spans []*tracepb.Span)
}

func (s *mockTraceServer) BatchWriteSpans(ctx context.Context, req *tracepb.BatchWriteSpansRequest) (*emptypb.Empty, error) {
	var err error
	if s.onUpload != nil {
		s.onUpload(ctx, req.Spans)
	}
	s.mu.Lock()
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-time.After(s.delay):
		s.spansUploaded = append(s.spansUploaded, req.Spans...)
	}
	s.mu.Unlock()
	return &emptypb.Empty{}, err
}

func (s *mockTraceServer) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.spansUploaded)
}

// clientOpt is the option tests should use to connect to the test server.
// It is initialized by TestMain.
var clientOpt []option.ClientOption

var (
	mockTrace mockTraceServer
)

func TestMain(m *testing.M) {
	flag.Parse()

	serv := grpc.NewServer()
	tracepb.RegisterTraceServiceServer(serv, &mockTrace)

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		_ = serv.Serve(lis)
	}()

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	clientOpt = []option.ClientOption{option.WithGRPCConn(conn)}

	os.Exit(m.Run())
}

func TestExporter_ExportSpans(t *testing.T) {
	// Initial test precondition
	mockTrace.spansUploaded = nil
	mockTrace.delay = 0

	// Create Google Cloud Trace Exporter
	exp, err := texporter.NewExporter(
		texporter.WithProjectID("PROJECT_ID_NOT_REAL"),
		texporter.WithTraceClientOptions(clientOpt),
		// handle bundle as soon as span is received
		texporter.WithBundleCountThreshold(1),
	)
	assert.NoError(t, err)

	tp, err := sdktrace.NewProvider(
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithBatcher(exp, // add following two options to ensure flush
			sdktrace.WithBatchTimeout(1),
			sdktrace.WithMaxExportBatchSize(1),
		))
	assert.NoError(t, err)

	global.SetTraceProvider(tp)
	_, span := global.TraceProvider().Tracer("test-tracer").Start(context.Background(), "test-span")
	span.End()
	assert.True(t, span.SpanContext().IsValid())

	// wait exporter to flush
	time.Sleep(20 * time.Millisecond)
	assert.EqualValues(t, 1, mockTrace.len())
}

func TestExporter_Timeout(t *testing.T) {
	// Initial test precondition
	mockTrace.spansUploaded = nil
	mockTrace.delay = 20 * time.Millisecond
	var exportErrors []error
	ch := make(chan error)

	// Create Google Cloud Trace Exporter
	exp, err := texporter.NewExporter(
		texporter.WithProjectID("PROJECT_ID_NOT_REAL"),
		texporter.WithTraceClientOptions(clientOpt),
		texporter.WithTimeout(1*time.Millisecond),
		// handle bundle as soon as span is received
		texporter.WithBundleCountThreshold(1),
		texporter.WithOnError(func(err error) {
			exportErrors = append(exportErrors, err)
			ch <- err
		}),
	)
	assert.NoError(t, err)

	tp, err := sdktrace.NewProvider(
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithSyncer(exp))
	assert.NoError(t, err)

	global.SetTraceProvider(tp)
	_, span := global.TraceProvider().Tracer("test-tracer").Start(context.Background(), "test-span")
	span.End()
	assert.True(t, span.SpanContext().IsValid())

	// wait for error to be handled
	<-ch
	assert.EqualValues(t, 0, mockTrace.len())
	if got, want := len(exportErrors), 1; got != want {
		t.Fatalf("len(exportErrors) = %q; want %q", got, want)
	}
	if got, want := exportErrors[0].Error(), "rpc error: code = DeadlineExceeded desc = context deadline exceeded"; got != want {
		t.Fatalf("err.Error() = %q; want %q", got, want)
	}
}

func TestBundling(t *testing.T) {
	mockTrace.mu.Lock()
	mockTrace.spansUploaded = nil
	mockTrace.delay = 0
	ch := make(chan []*tracepb.Span)
	mockTrace.onUpload = func(ctx context.Context, spans []*tracepb.Span) {
		ch <- spans
	}
	mockTrace.mu.Unlock()

	exporter, err := texporter.NewExporter(
		texporter.WithProjectID("PROJECT_ID_NOT_REAL"),
		texporter.WithTraceClientOptions(clientOpt),
		texporter.WithBundleDelayThreshold(time.Second / 10),
		texporter.WithBundleCountThreshold(10),
	)
	assert.NoError(t, err) 

	tp, err := sdktrace.NewProvider(
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithSyncer(exporter))
	assert.NoError(t, err)

	global.SetTraceProvider(tp)

	for i := 0; i < 35; i++ {
		_, span := global.TraceProvider().Tracer("test-tracer").Start(context.Background(), "test-span")
		span.End()
	}

	// Read the first three bundles.
	<-ch
	<-ch
	<-ch

	// Test that the fourth bundle isn't sent early.
	select {
	case <-ch:
		t.Errorf("bundle sent too early")
	case <-time.After(time.Second / 20):
		<-ch
	}

	// Test that there aren't extra bundles.
	select {
	case <-ch:
		t.Errorf("too many bundles sent")
	case <-time.After(time.Second / 5):
	}
}


func TestBundling_ConcurrentExports(t *testing.T) {
	mockTrace.mu.Lock()
	mockTrace.spansUploaded = nil
	mockTrace.delay = 0
	var exportMap sync.Map // maintain a collection of the spans exported
	wg := sync.WaitGroup{}
	mockTrace.onUpload = func(ctx context.Context, spans []*tracepb.Span) {
		for _, s := range spans {
			exportMap.Store(s.SpanId, true)
		}
		wg.Done()

		// Don't complete the function until the WaitGroup is done.
		// This ensures the semaphore limiting the concurrent uploads is not
		// released by one goroutine completing before the other.
		wg.Wait()
	}
	mockTrace.mu.Unlock()

	workers := 3
	spansPerWorker := 50
	delay := 2 * time.Second
	exporter, err := texporter.NewExporter(
		texporter.WithProjectID("PROJECT_ID_NOT_REAL"),
		texporter.WithTraceClientOptions(clientOpt),
		texporter.WithBundleDelayThreshold(delay),
		texporter.WithBundleCountThreshold(spansPerWorker),
		texporter.WithMaxNumberOfWorkers(workers),
	)
	assert.NoError(t, err) 

	tp, err := sdktrace.NewProvider(
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithSyncer(exporter))
	assert.NoError(t, err)

	global.SetTraceProvider(tp)

	waitCh := make(chan struct{})
	wg.Add(workers)

	totalSpans := workers * spansPerWorker
	var expectedSpanIDs []string
	go func() {
		// Release enough spans to form two bundles
		for i := 0; i < totalSpans; i++ {
			_, span := global.TraceProvider().Tracer("test-tracer").Start(context.Background(), "test-span")
			expectedSpanIDs = append(expectedSpanIDs, span.SpanContext().SpanID.String())
			span.End()
		}

		// Wait for the desired concurrency before completing
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
	case <-time.After(delay / 2): // fail before a time-based flush is triggered
		t.Fatal("timed out waiting for concurrent uploads")
	}

	// all the spans are accounted for
	var exportedSpans []string
	exportMap.Range(func(key, value interface{}) bool {
		exportedSpans = append(exportedSpans, key.(string))
		return true
	})
	if len(exportedSpans) != totalSpans {
		t.Errorf("got %d spans, want %d", len(exportedSpans), totalSpans)
	}
	for _, id := range expectedSpanIDs {
		if _, ok := exportMap.Load(id); !ok {
			t.Errorf("want %s; missing from exported spans", id)
		}
	}
}
