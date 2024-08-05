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
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	traceapi "cloud.google.com/go/trace/apiv2"
	"cloud.google.com/go/trace/apiv2/tracepb"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"
)

// traceExporter is an implementation of trace.Exporter and trace.BatchExporter
// that uploads spans to Stackdriver Trace in batch.
type traceExporter struct {
	o *options
	// uploadFn defaults in uploadSpans; it can be replaced for tests.
	uploadFn  func(ctx context.Context, req *tracepb.BatchWriteSpansRequest) error
	client    *traceapi.Client
	projectID string
	overflowLogger
}

// otelStatsHandler is a singleton otelgrpc.clientHandler to be used across
// all dial connections to avoid the memory leak documented in
// https://github.com/open-telemetry/opentelemetry-go-contrib/issues/4226
//
// TODO: If 4226 has been fixed in opentelemetry-go-contrib, replace this
// singleton with inline usage for simplicity.
var (
	initOtelStatsHandlerOnce sync.Once
	otelStatsHandler         stats.Handler
)

// otelGRPCStatsHandler returns singleton otelStatsHandler for reuse across all
// dial connections.
func otelGRPCStatsHandler() stats.Handler {
	initOtelStatsHandlerOnce.Do(func() {
		// disable tracing in the trace exporter to ensure we don't create an infinite loop.
		otelStatsHandler = otelgrpc.NewClientHandler(otelgrpc.WithTracerProvider(noop.NewTracerProvider()))
	})
	return otelStatsHandler
}

func newTraceExporter(o *options) (*traceExporter, error) {
	clientOps := append([]option.ClientOption{
		option.WithGRPCDialOption(grpc.WithUserAgent(userAgent)),
		option.WithTelemetryDisabled(),
		option.WithGRPCDialOption(grpc.WithStatsHandler(otelGRPCStatsHandler()))},
		o.traceClientOptions...)
	client, err := traceapi.NewClient(o.context, clientOps...)
	if err != nil {
		return nil, fmt.Errorf("stackdriver: couldn't initiate trace client: %v", err)
	}

	e := &traceExporter{
		projectID:      o.projectID,
		client:         client,
		o:              o,
		overflowLogger: overflowLogger{delayDur: 5 * time.Second},
	}
	e.uploadFn = e.uploadSpans
	return e, nil
}

func (e *traceExporter) ExportSpans(ctx context.Context, spanData []sdktrace.ReadOnlySpan) error {
	// Ship the whole bundle o data.
	results := make(map[string][]*tracepb.Span)
	for _, sd := range spanData {
		span, project := e.protoFromReadOnlySpan(sd)
		results[project] = append(results[project], span)
	}
	var errs []error
	for projectID, spans := range results {
		req := &tracepb.BatchWriteSpansRequest{
			Name:  "projects/" + projectID,
			Spans: spans,
		}
		errs = append(errs, e.uploadFn(ctx, req))
	}
	return errors.Join(errs...)
}

// ConvertSpan converts a ReadOnlySpan to Stackdriver Trace.
func (e *traceExporter) ConvertSpan(_ context.Context, sd sdktrace.ReadOnlySpan) *tracepb.Span {
	span, _ := e.protoFromReadOnlySpan(sd)
	return span
}

func (e *traceExporter) Shutdown(ctx context.Context) error {
	return e.client.Close()
}

// uploadSpans sends a set of spans to Stackdriver.
func (e *traceExporter) uploadSpans(ctx context.Context, req *tracepb.BatchWriteSpansRequest) error {
	var cancel func()
	ctx, cancel = newContextWithTimeout(ctx, e.o.timeout)
	defer cancel()

	// TODO(ymotongpoo): add this part after OTel support NeverSampler
	// for tracer.Start() initialization.
	//
	// tracer := apitrace.Register()
	// ctx, span := tracer.Start(
	// 	ctx,
	// 	"go.opentelemetry.io/otel/exporters/stackdriver.uploadSpans",
	// )
	// defer span.End()
	// span.SetAttributes(kv.Int64("num_spans", int64(len(spans))))

	if e.o.destinationProjectQuota {
		ctx = metadata.NewOutgoingContext(ctx, metadata.New(map[string]string{"x-goog-user-project": strings.TrimPrefix(req.Name, "projects/")}))
	}

	err := e.client.BatchWriteSpans(ctx, req)
	if err != nil {
		// TODO(ymotongpoo): handle detailed error categories
		// span.SetStatus(codes.Unknown)
		e.o.handleError(fmt.Errorf("failed to export to Google Cloud Trace: %w", err))
	}
	return err
}

// overflowLogger ensures that at most one overflow error log message is
// written every 5 seconds.
type overflowLogger struct {
	delayDur time.Duration
}
