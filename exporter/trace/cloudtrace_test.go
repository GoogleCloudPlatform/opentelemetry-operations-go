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

	"github.com/googleinterns/cloud-operations-api-mock/cloudmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	tracepb "google.golang.org/genproto/googleapis/devtools/cloudtrace/v2"
	codepb "google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestExporter_ExportSpan(t *testing.T) {
	// Initial test precondition
	mock := cloudmock.NewCloudMock()
	clientOpt := []option.ClientOption{option.WithGRPCConn(mock.ClientConn())}

	// Create Google Cloud Trace Exporter
	_, shutdown, err := InstallNewPipeline(
		[]Option{
			WithProjectID("PROJECT_ID_NOT_REAL"),
			WithTraceClientOptions(clientOpt),
			WithDefaultTraceAttributes(map[string]interface{}{"TEST_ATTRIBUTE": "TEST_VALUE"}),
		},
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	assert.NoError(t, err)

	_, span := otel.Tracer("test-tracer").Start(context.Background(), "test-span")
	// NOTE: https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/api.md#set-status
	// Status message MUST only be used with error code, so this will be dropped.
	span.SetStatus(codes.Ok, "Status message")
	span.End()
	assert.True(t, span.SpanContext().IsValid())

	_, span = otel.Tracer("test-tracer").Start(context.Background(), "test-span-with-error-status")
	span.SetStatus(codes.Error, "Error Message")
	span.End()

	// wait exporter to shutdown (closes grpc connection)
	shutdown()
	assert.EqualValues(t, 2, mock.GetNumSpans())
	// Note: Go returns empty string for an unset member.
	assert.EqualValues(t, codepb.Code_OK, mock.GetSpan(0).GetStatus().Code)
	assert.EqualValues(t, "", mock.GetSpan(0).GetStatus().Message)
	assert.EqualValues(t, codepb.Code_UNKNOWN, mock.GetSpan(1).GetStatus().Code)
	assert.EqualValues(t, "Error Message", mock.GetSpan(1).GetStatus().Message)
	assert.EqualValues(t, "TEST_VALUE", mock.GetSpan(0).Attributes.AttributeMap["TEST_ATTRIBUTE"].GetStringValue().Value)
}

func TestExporter_DisplayNameNoFormatter(t *testing.T) {
	// Initial test precondition
	mock := cloudmock.NewCloudMock()
	clientOpt := []option.ClientOption{option.WithGRPCConn(mock.ClientConn())}

	spanName := "span1234"

	// Create Google Cloud Trace Exporter
	_, shutdown, err := InstallNewPipeline(
		[]Option{
			WithProjectID("PROJECT_ID_NOT_REAL"),
			WithTraceClientOptions(clientOpt),
			WithDisplayNameFormatter(nil),
		},
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	assert.NoError(t, err)

	_, span := otel.Tracer("test-tracer").Start(context.Background(), spanName)
	span.End()
	assert.True(t, span.SpanContext().IsValid())

	// wait exporter to shutdown (closes grpc connection)
	shutdown()
	assert.EqualValues(t, 1, mock.GetNumSpans())
	assert.EqualValues(t, spanName, mock.GetSpan(0).DisplayName.Value)
}

func TestExporter_DisplayNameFormatter(t *testing.T) {
	// Initial test precondition
	mock := cloudmock.NewCloudMock()
	clientOpt := []option.ClientOption{option.WithGRPCConn(mock.ClientConn())}

	spanName := "span1234"
	format := func(s *sdktrace.SpanSnapshot) string {
		return "TEST_FORMAT" + s.Name
	}

	// Create Google Cloud Trace Exporter
	_, shutdown, err := InstallNewPipeline(
		[]Option{
			WithProjectID("PROJECT_ID_NOT_REAL"),
			WithTraceClientOptions(clientOpt),
			WithDisplayNameFormatter(format),
		},
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	assert.NoError(t, err)

	_, span := otel.Tracer("test-tracer").Start(context.Background(), spanName)
	span.End()
	assert.True(t, span.SpanContext().IsValid())

	// wait exporter to shutdown (closes grpc connection)
	shutdown()
	assert.EqualValues(t, 1, mock.GetNumSpans())
	assert.EqualValues(t, "TEST_FORMAT"+spanName, mock.GetSpan(0).DisplayName.Value)
}

func TestExporter_Timeout(t *testing.T) {
	// Initial test precondition
	mock := cloudmock.NewCloudMock()
	mock.SetDelay(20 * time.Millisecond)
	clientOpt := []option.ClientOption{option.WithGRPCConn(mock.ClientConn())}
	var exportErrors []error

	// Create Google Cloud Trace Exporter
	_, shutdown, err := InstallNewPipeline(
		[]Option{
			WithProjectID("PROJECT_ID_NOT_REAL"),
			WithTraceClientOptions(clientOpt),
			WithTimeout(1 * time.Millisecond),
			// handle bundle as soon as span is received
			WithOnError(func(err error) {
				exportErrors = append(exportErrors, err)
			}),
		},
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	assert.NoError(t, err)

	_, span := otel.Tracer("test-tracer").Start(context.Background(), "test-span")
	span.End()
	assert.True(t, span.SpanContext().IsValid())

	// wait for error to be handled
	shutdown() // closed grpc connection
	assert.EqualValues(t, 0, mock.GetNumSpans())
	if got, want := len(exportErrors), 1; got != want {
		t.Fatalf("len(exportErrors) = %q; want %q", got, want)
	}
	got, want := exportErrors[0].Error(), "rpc error: code = (DeadlineExceeded|Unknown) desc = context deadline exceeded"
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
	go server.Serve(lis)

	// Wire into buffer output.
	clientOpt := []option.ClientOption{
		option.WithEndpoint(lis.Addr().String()),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithInsecure()),
	}
	// Create Google Cloud Trace Exporter
	_, shutdown, err := InstallNewPipeline(
		[]Option{
			WithProjectID("PROJECT_ID_NOT_REAL"),
			WithTraceClientOptions(clientOpt),
		},
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	assert.NoError(t, err)

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
