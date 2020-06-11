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
	"sync"
	"time"

	traceclient "cloud.google.com/go/trace/apiv2"
	tracepb "google.golang.org/genproto/googleapis/devtools/cloudtrace/v2"
	"google.golang.org/api/support/bundler"
	"github.com/golang/protobuf/proto"

	export "go.opentelemetry.io/otel/sdk/export/trace"
)

// traceExporter is an imeplementation of trace.Exporter and trace.BatchExporter
// that uploads spans to Stackdriver Trace in batch.
type traceExporter struct {
	o         *options
	projectID string
	bundler   *bundler.Bundler
	// uploadFn defaults in uploadSpans; it can be replaced for tests.
	uploadFn func(ctx context.Context, spans []*tracepb.Span)
	overflowLogger
	client   *traceclient.Client
}

const defaultBufferedByteLimit = 8 * 1024 * 1024
const defaultBundleDelayThreshold = 2 * time.Second
const defaultBundleCountThreshold = 50
const bundleByteThresholdMultiplier = 300
const bundleByteLimitMultiplier = 1000

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
	b := bundler.NewBundler((*contextAndSpans)(nil), func(bundle interface{}) {
		ctxSpans := bundle.([]*contextAndSpans)
		ctxToSpansMap := make(map[context.Context][]*tracepb.Span)
		// upload spans with same context in batch
		for _, cs := range ctxSpans {
			ctxToSpansMap[cs.ctx] = append(ctxToSpansMap[cs.ctx], cs.spans...)
		}
		for ctx, spans := range ctxToSpansMap {
			e.uploadFn(ctx, spans)
		}
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
	b.BundleByteThreshold = b.BundleCountThreshold * bundleByteThresholdMultiplier
	b.BundleByteLimit = b.BundleCountThreshold * bundleByteLimitMultiplier
	if o.TraceSpansBufferMaxBytes > 0 {
		b.BufferedByteLimit = o.TraceSpansBufferMaxBytes
	} else {
		b.BufferedByteLimit = defaultBufferedByteLimit
	}
	if o.MaxNumberOfWorkers > 0 {
		b.HandlerLimit = o.MaxNumberOfWorkers
	}

	e.bundler = b
	if o.UploadFunction == nil {
		e.uploadFn = e.uploadSpans
	} else {
		e.uploadFn = o.UploadFunction
	}
	return e, nil
}


func (e *traceExporter) checkBundlerError(err error) {
	switch err {
	case nil:
		return
	case bundler.ErrOversizedItem:
		fallthrough
	case bundler.ErrOverflow:
		e.overflowLogger.log()
	default:
		e.o.handleError(err)
	}
}

// ExportSpan exports a SpanData to Stackdriver Trace.
func (e *traceExporter) ExportSpan(ctx context.Context, sd *export.SpanData) {
	protoSpan := protoFromSpanData(sd, e.projectID)
	protoSize := proto.Size(protoSpan)
	err := e.bundler.Add(&contextAndSpans{
		ctx: ctx, 
		spans: []*tracepb.Span{protoSpan},
	}, protoSize)
	e.checkBundlerError(err)
}

// ExportSpans exports a slice of SpanData to Stackdriver Trace in batch
func (e *traceExporter) ExportSpans(ctx context.Context, sds []*export.SpanData) {
	pbSpans := make([]*tracepb.Span, len(sds))
	var protoSize int = 0
	for i, sd := range sds {
		pbSpans[i] = protoFromSpanData(sd, e.projectID)
		protoSize += proto.Size(pbSpans[i])
	}
	err := e.bundler.Add(&contextAndSpans{ctx, pbSpans}, protoSize)
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
	// span.SetAttributes(key.New("num_spans").Int64(int64(len(spans))))

	err := e.client.BatchWriteSpans(ctx, &req)
	if err != nil {
		// TODO(ymotongpoo): handle detailed error categories
		// span.SetStatus(codes.Unknown)
		e.o.handleError(err)
	}
}

// contextAndSpan stores both a context and spans for use with a bundler.
type contextAndSpans struct {
	ctx context.Context
	spans []*tracepb.Span
}

// overflowLogger ensures that at most one overflow error log message is
// written every 5 seconds.
type overflowLogger struct {
	mu    sync.Mutex
	pause bool
	accum int
}

func (o *overflowLogger) delay() {
	o.pause = true
	time.AfterFunc(5*time.Second, func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		switch {
		case o.accum == 0:
			o.pause = false
		case o.accum == 1:
			log.Println("OpenCensus Stackdriver exporter: failed to upload span: buffer full")
			o.accum = 0
			o.delay()
		default:
			log.Printf("OpenCensus Stackdriver exporter: failed to upload %d spans: buffer full", o.accum)
			o.accum = 0
			o.delay()
		}
	})
}

func (o *overflowLogger) log() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.pause {
		log.Println("OpenCensus Stackdriver exporter: failed to upload span: buffer full")
		o.delay()
	} else {
		o.accum++
	}
}