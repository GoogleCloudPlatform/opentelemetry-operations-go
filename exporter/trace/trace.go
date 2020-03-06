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
	"fmt"

	traceclient "cloud.google.com/go/trace/apiv2"
	tracepb "google.golang.org/genproto/googleapis/devtools/cloudtrace/v2"

	export "go.opentelemetry.io/otel/sdk/export/trace"
)

// traceExporter is an imeplementation of trace.Exporter and trace.BatchExporter
// that uploads spans to Stackdriver Trace in batch.
type traceExporter struct {
	o         *options
	projectID string
	// uploadFn defaults in uploadSpans; it can be replaced for tests.
	uploadFn func(ctx context.Context, spans []*tracepb.Span)
	client   *traceclient.Client
}

func newTraceExporter(o *options) (*traceExporter, error) {
	client, err := traceclient.NewClient(o.Context, o.TraceClientOptions...)
	if err != nil {
		return nil, fmt.Errorf("stackdriver: couldn't initiate trace client: %v", err)
	}
	e := &traceExporter{
		projectID: o.ProjectID,
		client:    client,
		o:         o,
	}
	e.uploadFn = e.uploadSpans
	return e, nil
}

// ExportSpan exports a SpanData to Stackdriver Trace.
func (e *traceExporter) ExportSpan(ctx context.Context, sd *export.SpanData) {
	protoSpan := protoFromSpanData(sd, e.projectID)
	e.uploadFn(ctx, []*tracepb.Span{protoSpan})
}

// ExportSpans exports a slice of SpanData to Stackdriver Trace in batch
func (e *traceExporter) ExportSpans(ctx context.Context, sds []*export.SpanData) {
	pbSpans := make([]*tracepb.Span, len(sds))
	for i, sd := range sds {
		pbSpans[i] = protoFromSpanData(sd, e.projectID)
	}
	e.uploadFn(ctx, pbSpans)
}

// uploadSpans sends a set of spans to Stackdriver.
func (e *traceExporter) uploadSpans(ctx context.Context, spans []*tracepb.Span) {
	req := tracepb.BatchWriteSpansRequest{
		Name:  "projects/" + e.projectID,
		Spans: spans,
	}

	var cancel func()
	ctx, cancel = newContextWithTimeout(ctx, e.o.Timeout)
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
	// span.SetAttributes(key.New("num_spans").Int64(int64(len(spans))))

	err := e.client.BatchWriteSpans(ctx, &req)
	if err != nil {
		// TODO(ymotongpoo): handle detailed error categories
		// span.SetStatus(codes.Unknown)
		e.o.handleError(err)
	}
}
