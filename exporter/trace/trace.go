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
	"log"
	"strings"
	"sync"
	"time"

	traceclient "cloud.google.com/go/trace/apiv2"
	"google.golang.org/api/support/bundler"
	tracepb "google.golang.org/genproto/googleapis/devtools/cloudtrace/v2"
	"google.golang.org/protobuf/proto"

	export "go.opentelemetry.io/otel/sdk/export/trace"
)

// traceExporter is an implementation of trace.Exporter and trace.BatchExporter
// that uploads spans to Stackdriver Trace in batch.
type traceExporter struct {
	o         *options
	projectID string
	bundler   *bundler.Bundler
	// uploadFn defaults in uploadSpans; it can be replaced for tests.
	uploadFn func(ctx context.Context, spans []*tracepb.Span)
	overflowLogger
	client *traceclient.Client
}

const defaultBundleDelayThreshold = 2 * time.Second
const defaultBundleCountThreshold = 50
const defaultBundleByteThreshold = 15000
const defaultBundleByteLimit = 0
const defaultBufferedByteLimit = 8 * 1024 * 1024

func newTraceExporter(o *options) (*traceExporter, error) {
	client, err := traceclient.NewClient(o.Context, o.TraceClientOptions...)
	if err != nil {
		return nil, fmt.Errorf("stackdriver: couldn't initiate trace client: %v", err)
	}
	e := &traceExporter{
		projectID:      o.ProjectID,
		client:         client,
		o:              o,
		overflowLogger: overflowLogger{delayDur: 5 * time.Second},
	}
	b := bundler.NewBundler((*tracepb.Span)(nil), func(bundle interface{}) {
		e.uploadFn(context.Background(), bundle.([]*tracepb.Span))
	})

	if o.BundleDelayThreshold > 0 {
		b.DelayThreshold = o.BundleDelayThreshold
	} else {
		b.DelayThreshold = defaultBundleDelayThreshold
	}

	if o.BundleCountThreshold > 0 {
		b.BundleCountThreshold = o.BundleCountThreshold
	} else {
		b.BundleCountThreshold = defaultBundleCountThreshold
	}

	if o.BundleByteThreshold > 0 {
		b.BundleByteThreshold = o.BundleByteThreshold
	} else {
		b.BundleByteThreshold = defaultBundleByteThreshold
	}

	if o.BundleByteLimit > 0 {
		b.BundleByteLimit = o.BundleByteLimit
	} else {
		b.BundleByteLimit = defaultBundleByteLimit
	}

	if o.BufferMaxBytes > 0 {
		b.BufferedByteLimit = o.BufferMaxBytes
	} else {
		b.BufferedByteLimit = defaultBufferedByteLimit
	}

	if o.MaxNumberOfWorkers > 0 {
		b.HandlerLimit = o.MaxNumberOfWorkers
	}

	e.bundler = b
	e.uploadFn = e.uploadSpans
	return e, nil
}

func (e *traceExporter) checkBundlerError(err error) {
	switch err {
	case nil:
		return
	case bundler.ErrOversizedItem:
		e.overflowLogger.log(true)
	case bundler.ErrOverflow:
		e.overflowLogger.log(false)
	default:
		e.o.handleError(err)
	}
}

// ExportSpan exports a SpanData to Stackdriver Trace.
func (e *traceExporter) ExportSpan(_ context.Context, sd *export.SpanData) {
	protoSpan := protoFromSpanData(sd, e.projectID, e.o.DisplayNameFormatter)
	protoSize := proto.Size(protoSpan)
	err := e.bundler.Add(protoSpan, protoSize)
	e.checkBundlerError(err)
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
	// span.SetAttributes(kv.Int64("num_spans", int64(len(spans))))

	err := e.client.BatchWriteSpans(ctx, &req)
	if err != nil {
		// TODO(ymotongpoo): handle detailed error categories
		// span.SetStatus(codes.Unknown)
		e.o.handleError(err)
	}
}

func (e *traceExporter) Flush() {
	e.bundler.Flush()
}

// overflowLogger ensures that at most one overflow error log message is
// written every 5 seconds.
type overflowLogger struct {
	mu         sync.Mutex
	pause      bool
	delayDur   time.Duration
	bufferErrs int
	oversized  int
}

func (o *overflowLogger) log(oversized bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.pause {
		o.delay()
	}

	if oversized {
		o.oversized++
	} else {
		o.bufferErrs++
	}
}

func (o *overflowLogger) delay() {
	o.pause = true
	time.AfterFunc(o.delayDur, func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		o.pause = false
		logBufferErrors(o.bufferErrs, o.oversized)
		o.bufferErrs = 0
		o.oversized = 0
	})
}

func logBufferErrors(bufferFull, oversized int) {
	if bufferFull == 0 && oversized == 0 {
		return
	}

	msgs := make([]string, 0, 2)
	if bufferFull > 0 {
		msgs = append(msgs, fmt.Sprintf("buffer full: %v", bufferFull))
	}
	if oversized > 0 {
		msgs = append(msgs, fmt.Sprintf("oversized item: %v", oversized))
	}

	log.Printf("OpenTelemetry Cloud Trace exporter: failed to upload spans: %s\n", strings.Join(msgs, ", "))
}
