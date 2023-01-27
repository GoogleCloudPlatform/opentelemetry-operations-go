// Copyright 2019 OpenTelemetry Authors
// Copyright 2021 Google LLC
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

package trace

import (
	"context"
	"net"
	"regexp"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	tracepb "google.golang.org/genproto/googleapis/devtools/cloudtrace/v2"
	codepb "google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock"
)

func TestExporter_ExportSpan(t *testing.T) {
	// Initial test precondition
	testServer, err := cloudmock.NewTracesTestServer()
	go testServer.Serve()
	defer testServer.Shutdown()
	assert.NoError(t, err)
	clientOpt := []option.ClientOption{
		option.WithEndpoint(testServer.Endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	// Create Google Cloud Trace Exporter
	exporter, err := New(
		WithProjectID("PROJECT_ID_NOT_REAL"),
		WithTraceClientOptions(clientOpt))
	assert.NoError(t, err)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter))
	//nolint:errcheck
	defer tp.Shutdown(context.Background())
	otel.SetTracerProvider(tp)

	_, span := otel.Tracer("test-tracer").Start(context.Background(), "test-span")
	// NOTE: https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/api.md#set-status
	// Status message MUST only be used with error code, so this will be dropped.
	span.SetStatus(codes.Ok, "Status message")
	span.End()
	assert.True(t, span.SpanContext().IsValid())

	_, span = otel.Tracer("test-tracer").Start(
		context.Background(),
		"test-span-with-error-status",
		trace.WithLinks(trace.Link{SpanContext: genSpanContext()}),
	)
	span.SetStatus(codes.Error, "Error Message")
	span.End()

	batch := testServer.CreateBatchWriteSpansRequests()
	assert.Equal(t, 2, len(batch))
	assert.Equal(t, batch[0].GetName(), "projects/PROJECT_ID_NOT_REAL")
	assert.Equal(t, batch[1].GetName(), "projects/PROJECT_ID_NOT_REAL")
	assert.Equal(t, 1, len(batch[0].GetSpans()))
	assert.Equal(t, 1, len(batch[1].GetSpans()))
	// Note: Go returns empty string for an unset member.
	assert.EqualValues(t, codepb.Code_OK, batch[0].GetSpans()[0].GetStatus().Code)
	assert.EqualValues(t, "", batch[0].GetSpans()[0].GetStatus().Message)
	assert.EqualValues(t, codepb.Code_UNKNOWN, batch[1].GetSpans()[0].GetStatus().Code)
	assert.EqualValues(t, "Error Message", batch[1].GetSpans()[0].GetStatus().Message)
	assert.Nil(t, batch[0].GetSpans()[0].Links)
	assert.Len(t, batch[1].GetSpans()[0].Links.Link, 1)
}

type errorHandler struct {
	errs []error
}

func (e *errorHandler) Handle(err error) {
	e.errs = append(e.errs, err)
}

func TestExporter_Retry(t *testing.T) {
	// Initial test precondition
	testServer, err := cloudmock.NewTracesTestServer(cloudmock.WithErrorResponse(&googleapi.Error{Code: int(grpccodes.Unavailable)}))
	go testServer.Serve()
	defer testServer.Shutdown()
	assert.NoError(t, err)
	clientOpt := []option.ClientOption{
		option.WithEndpoint(testServer.Endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}
	handler := &errorHandler{}

	// Create Google Cloud Trace Exporter
	exporter, err := New(
		WithProjectID("PROJECT_ID_NOT_REAL"),
		WithTraceClientOptions(clientOpt),
		// handle bundle as soon as span is received
		WithErrorHandler(handler),
	)
	assert.NoError(t, err)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter))
	//nolint:errcheck
	defer tp.Shutdown(context.Background())
	otel.SetTracerProvider(tp)

	_, span := otel.Tracer("test-tracer").Start(context.Background(), "test-span")
	span.End()
	assert.True(t, span.SpanContext().IsValid())

	// wait for error to be handled
	batch := testServer.CreateBatchWriteSpansRequests()
	assert.Equal(t, 0, len(batch))
	if got, want := len(handler.errs), 1; got != want {
		t.Fatalf("len(exportErrors) = %q; want %q", got, want)
	}
	assert.Greater(t, testServer.Retries, 0)
}

func TestExporter_RetryEnabled_NoRetries(t *testing.T) {
	// Initial test precondition
	testServer, err := cloudmock.NewTracesTestServer()
	go testServer.Serve()
	defer testServer.Shutdown()
	assert.NoError(t, err)
	clientOpt := []option.ClientOption{
		option.WithEndpoint(testServer.Endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	// Create Google Cloud Trace Exporter
	exporter, err := New(
		WithProjectID("PROJECT_ID_NOT_REAL"),
		WithTraceClientOptions(clientOpt),
	)
	assert.NoError(t, err)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter))
	//nolint:errcheck
	defer tp.Shutdown(context.Background())
	otel.SetTracerProvider(tp)

	_, span := otel.Tracer("test-tracer").Start(context.Background(), "test-span")
	// NOTE: https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/api.md#set-status
	// Status message MUST only be used with error code, so this will be dropped.
	span.SetStatus(codes.Ok, "Status message")
	span.End()
	assert.True(t, span.SpanContext().IsValid())

	_, span = otel.Tracer("test-tracer").Start(
		context.Background(),
		"test-span-with-error-status",
		trace.WithLinks(trace.Link{SpanContext: genSpanContext()}),
	)
	span.SetStatus(codes.Error, "Error Message")
	span.End()

	batch := testServer.CreateBatchWriteSpansRequests()
	assert.Equal(t, 2, len(batch))
	assert.Equal(t, batch[0].GetName(), "projects/PROJECT_ID_NOT_REAL")
	assert.Equal(t, batch[1].GetName(), "projects/PROJECT_ID_NOT_REAL")
	assert.Equal(t, 1, len(batch[0].GetSpans()))
	assert.Equal(t, 1, len(batch[1].GetSpans()))
	// Note: Go returns empty string for an unset member.
	assert.EqualValues(t, codepb.Code_OK, batch[0].GetSpans()[0].GetStatus().Code)
	assert.EqualValues(t, "", batch[0].GetSpans()[0].GetStatus().Message)
	assert.EqualValues(t, codepb.Code_UNKNOWN, batch[1].GetSpans()[0].GetStatus().Code)
	assert.EqualValues(t, "Error Message", batch[1].GetSpans()[0].GetStatus().Message)
	assert.Nil(t, batch[0].GetSpans()[0].Links)
	assert.Len(t, batch[1].GetSpans()[0].Links.Link, 1)
	assert.Equal(t, testServer.Retries, 0)
}

func TestExporter_Timeout(t *testing.T) {
	// Initial test precondition
	testServer, err := cloudmock.NewTracesTestServer(cloudmock.WithDelay(20 * time.Millisecond))
	go testServer.Serve()
	defer testServer.Shutdown()
	assert.NoError(t, err)
	clientOpt := []option.ClientOption{
		option.WithEndpoint(testServer.Endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}
	handler := &errorHandler{}

	// Create Google Cloud Trace Exporter
	exporter, err := New(
		WithProjectID("PROJECT_ID_NOT_REAL"),
		WithTraceClientOptions(clientOpt),
		WithTimeout(time.Millisecond),
		// handle bundle as soon as span is received
		WithErrorHandler(handler),
	)
	assert.NoError(t, err)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter))
	//nolint:errcheck
	defer tp.Shutdown(context.Background())
	otel.SetTracerProvider(tp)

	_, span := otel.Tracer("test-tracer").Start(context.Background(), "test-span")
	span.End()
	assert.True(t, span.SpanContext().IsValid())

	// wait for error to be handled
	batch := testServer.CreateBatchWriteSpansRequests()
	assert.Equal(t, 0, len(batch))
	if got, want := len(handler.errs), 1; got != want {
		t.Fatalf("len(exportErrors) = %q; want %q", got, want)
	}
	got, want := handler.errs[0].Error(), "failed to export to Google Cloud Trace: context deadline exceeded"
	if match, _ := regexp.MatchString(want, got); !match {
		t.Fatalf("err.Error() = %q; want %q", got, want)
	}
}

// A mock server we can re-use for different kinds of unit tests against batch-write request.
type mock struct {
	tracepb.UnimplementedTraceServiceServer
	batchWriteSpans func(ctx context.Context, req *tracepb.BatchWriteSpansRequest) (*emptypb.Empty, error)
}

func (m *mock) BatchWriteSpans(ctx context.Context, req *tracepb.BatchWriteSpansRequest) (*emptypb.Empty, error) {
	return m.batchWriteSpans(ctx, req)
}

func TestExporter_ExportWithUserAgent(t *testing.T) {
	// Initialize the mock server
	server := grpc.NewServer()
	t.Cleanup(server.Stop)

	// Channel we shove our user agent string into.
	ch := make(chan []string, 1)

	m := mock{
		batchWriteSpans: func(ctx context.Context, req *tracepb.BatchWriteSpansRequest) (*emptypb.Empty, error) {
			md, _ := metadata.FromIncomingContext(ctx)
			ch <- md.Get("User-Agent")
			return &emptypb.Empty{}, nil
		},
	}
	tracepb.RegisterTraceServiceServer(server, &m)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	// GO GO gadget local server!
	//nolint:errcheck
	go server.Serve(lis)

	// Wire into buffer output.
	clientOpt := []option.ClientOption{
		option.WithEndpoint(lis.Addr().String()),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	// Create Google Cloud Trace Exporter
	exporter, err := New(
		WithProjectID("PROJECT_ID_NOT_REAL"),
		WithTraceClientOptions(clientOpt))
	assert.NoError(t, err)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter))

	otel.SetTracerProvider(tp)
	shutdown := func() {
		err := tp.Shutdown(context.Background())
		if err != nil {
			t.Fatalf("error shutting down tracer provider: %+v", err)
		}
	}

	_, span := otel.Tracer("test-tracer").Start(context.Background(), "test-span")
	span.SetStatus(codes.Ok, "Status Message")
	span.End()
	assert.True(t, span.SpanContext().IsValid())

	// wait exporter to shutdown
	shutdown()
	// Now check for user agent string in the buffer.
	ua := <-ch
	require.Regexp(t, "opentelemetry-go .*; google-cloud-trace-exporter .*", ua[0])
}
