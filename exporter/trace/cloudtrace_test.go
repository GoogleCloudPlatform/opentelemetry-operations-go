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

package trace

import (
	"context"
	"log"
	"os"
	"regexp"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	export "go.opentelemetry.io/otel/sdk/export/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/googleinterns/cloud-operations-api-mock/cloudmock"
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/option"
	tracepb "google.golang.org/genproto/googleapis/devtools/cloudtrace/v2"
)

func TestExporter_ExportSpan(t *testing.T) {
	// Initial test precondition
	mock := cloudmock.NewCloudMock()
	defer mock.Shutdown()
	clientOpt := []option.ClientOption{option.WithGRPCConn(mock.ClientConn())}

	// Create Google Cloud Trace Exporter
	_, flush, err := InstallNewPipeline(
		[]Option{
			WithProjectID("PROJECT_ID_NOT_REAL"),
			WithTraceClientOptions(clientOpt),
			// handle bundle as soon as span is received
			WithBundleCountThreshold(1),
		},
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
	)
	assert.NoError(t, err)

	_, span := otel.Tracer("test-tracer").Start(context.Background(), "test-span")
	span.SetStatus(codes.Ok, "Status Message")
	span.End()
	assert.True(t, span.SpanContext().IsValid())

	_, span = otel.Tracer("test-tracer").Start(context.Background(), "test-span-with-error-status")
	span.SetStatus(codes.Error, "Error Message")
	span.End()

	// wait exporter to flush
	flush()
	assert.EqualValues(t, 2, mock.GetNumSpans())
	assert.EqualValues(t, "Status Message", mock.GetSpan(0).GetStatus().Message)
	assert.EqualValues(t, "Error Message", mock.GetSpan(1).GetStatus().Message)
}

func TestExporter_DisplayNameFormatter(t *testing.T) {
	// Initial test precondition
	mock := cloudmock.NewCloudMock()
	defer mock.Shutdown()
	clientOpt := []option.ClientOption{option.WithGRPCConn(mock.ClientConn())}

	spanName := "span1234"
	format := func(s *export.SpanSnapshot) string {
		return "TEST_FORMAT" + s.Name
	}

	// Create Google Cloud Trace Exporter
	_, flush, err := InstallNewPipeline(
		[]Option{
			WithProjectID("PROJECT_ID_NOT_REAL"),
			WithTraceClientOptions(clientOpt),
			WithBundleCountThreshold(1),
			WithDisplayNameFormatter(format),
		},
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
	)
	assert.NoError(t, err)

	_, span := otel.Tracer("test-tracer").Start(context.Background(), spanName)
	span.End()
	assert.True(t, span.SpanContext().IsValid())

	// wait exporter to flush
	flush()
	assert.EqualValues(t, 1, mock.GetNumSpans())
	assert.EqualValues(t, "TEST_FORMAT"+spanName, mock.GetSpan(0).DisplayName.Value)
}

func TestExporter_Timeout(t *testing.T) {
	// Initial test precondition
	mock := cloudmock.NewCloudMock()
	defer mock.Shutdown()
	mock.SetDelay(20 * time.Millisecond)
	clientOpt := []option.ClientOption{option.WithGRPCConn(mock.ClientConn())}
	var exportErrors []error

	// Create Google Cloud Trace Exporter
	_, flush, err := InstallNewPipeline(
		[]Option{
			WithProjectID("PROJECT_ID_NOT_REAL"),
			WithTraceClientOptions(clientOpt),
			WithTimeout(1 * time.Millisecond),
			// handle bundle as soon as span is received
			WithBundleCountThreshold(1),
			WithOnError(func(err error) {
				exportErrors = append(exportErrors, err)
			}),
		},
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
	)
	assert.NoError(t, err)

	_, span := otel.Tracer("test-tracer").Start(context.Background(), "test-span")
	span.End()
	assert.True(t, span.SpanContext().IsValid())

	// wait for error to be handled
	flush()
	assert.EqualValues(t, 0, mock.GetNumSpans())
	if got, want := len(exportErrors), 1; got != want {
		t.Fatalf("len(exportErrors) = %q; want %q", got, want)
	}
	got, want := exportErrors[0].Error(), "rpc error: code = (DeadlineExceeded|Unknown) desc = context deadline exceeded"
	if match, _ := regexp.MatchString(want, got); !match {
		t.Fatalf("err.Error() = %q; want %q", got, want)
	}
}

func TestBundling(t *testing.T) {
	mock := cloudmock.NewCloudMock()
	defer mock.Shutdown()
	clientOpt := []option.ClientOption{option.WithGRPCConn(mock.ClientConn())}

	ch := make(chan []*tracepb.Span)
	mock.SetOnUpload(func(ctx context.Context, spans []*tracepb.Span) {
		ch <- spans
	})

	_, _, err := InstallNewPipeline(
		[]Option{
			WithProjectID("PROJECT_ID_NOT_REAL"),
			WithTraceClientOptions(clientOpt),
			WithBundleDelayThreshold(time.Second / 10),
			WithBundleCountThreshold(10),
		},
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
	)
	assert.NoError(t, err)

	for i := 0; i < 35; i++ {
		_, span := otel.Tracer("test-tracer").Start(context.Background(), "test-span")
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
	mock := cloudmock.NewCloudMock()
	defer mock.Shutdown()
	clientOpt := []option.ClientOption{option.WithGRPCConn(mock.ClientConn())}

	var exportMap sync.Map // maintain a collection of the spans exported
	wg := sync.WaitGroup{}
	mock.SetOnUpload(func(ctx context.Context, spans []*tracepb.Span) {
		for _, s := range spans {
			exportMap.Store(s.SpanId, true)
		}
		wg.Done()

		// Don't complete the function until the WaitGroup is done.
		// This ensures the semaphore limiting the concurrent uploads is not
		// released by one goroutine completing before the other.
		wg.Wait()
	})

	workers := 3
	spansPerWorker := 50
	delay := 2 * time.Second
	_, flush, err := InstallNewPipeline(
		[]Option{
			WithProjectID("PROJECT_ID_NOT_REAL"),
			WithTraceClientOptions(clientOpt),
			WithBundleDelayThreshold(delay),
			WithBundleCountThreshold(spansPerWorker),
			WithMaxNumberOfWorkers(workers),
		},
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
	)
	assert.NoError(t, err)

	waitCh := make(chan struct{})
	wg.Add(workers)

	totalSpans := workers * spansPerWorker
	var expectedSpanIDs []string
	go func() {
		// Release enough spans to form two bundles
		for i := 0; i < totalSpans; i++ {
			_, span := otel.Tracer("test-tracer").Start(context.Background(), "test-span")
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

	flush()
}

func TestBundling_Oversized(t *testing.T) {
	mock := cloudmock.NewCloudMock()
	defer mock.Shutdown()
	clientOpt := []option.ClientOption{option.WithGRPCConn(mock.ClientConn())}

	var uploaded bool
	mock.SetOnUpload(func(ctx context.Context, spans []*tracepb.Span) { uploaded = true })

	exp, err := NewExporter(
		WithProjectID("PROJECT_ID_NOT_REAL"),
		WithTraceClientOptions(clientOpt),
		WithBundleByteLimit(1),
	)
	assert.NoError(t, err)
	exp.traceExporter.overflowLogger.delayDur = 100 * time.Millisecond

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithSyncer(exp),
	)
	otel.SetTracerProvider(tp)

	buf := &logBuffer{logInputChan: make(chan []byte, 100)}
	log.SetOutput(buf)
	defer func() {
		log.SetOutput(os.Stderr)
	}()

	go func() {
		for i := 0; i < 10; i++ {
			_, span := otel.Tracer("test-tracer").Start(context.Background(), "test-span")
			span.SetName("a")
			span.End()
		}
	}()

	if got, want := <-buf.logInputChan, regexp.MustCompile(`OpenTelemetry Cloud Trace exporter: failed to upload spans: oversized item: 10`); !want.Match([]byte(got)) {
		t.Errorf("log: got %s, want %s", got, want)
	}
	if got, want := uploaded, false; got != want {
		t.Errorf("uploaded: got %v, want %v", got, want)
	}
}

type logBuffer struct {
	logInputChan chan []byte
}

func (lb *logBuffer) Write(b []byte) (n int, err error) {
	lb.logInputChan <- b
	return len(b), nil
}
