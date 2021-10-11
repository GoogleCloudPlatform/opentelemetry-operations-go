// Copyright 2017, OpenCensus Authors
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

package stackdriver

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.opencensus.io/trace"
	tracepb "google.golang.org/genproto/googleapis/devtools/cloudtrace/v2"
)

func TestBundling(t *testing.T) {
	exporter := newTraceExporterWithClient(Options{
		ProjectID:            "fakeProjectID",
		BundleDelayThreshold: time.Second / 10,
		BundleCountThreshold: 10,
	}, nil)

	ch := make(chan []*tracepb.Span)
	exporter.uploadFn = func(spans []*tracepb.Span) {
		ch <- spans
	}
	trace.RegisterExporter(exporter)

	for i := 0; i < 35; i++ {
		_, span := trace.StartSpan(context.Background(), "span", trace.WithSampler(trace.AlwaysSample()))
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
	workers := 2
	spansPerWorker := 10
	delay := 2 * time.Second
	exporter := newTraceExporterWithClient(Options{
		ProjectID:            "fakeProjectID",
		BundleCountThreshold: spansPerWorker,
		BundleDelayThreshold: delay,
		NumberOfWorkers:      workers,
	}, nil)

	wg := sync.WaitGroup{}
	waitCh := make(chan struct{})
	wg.Add(workers)

	var exportMap sync.Map // maintain a collection of the spans exported
	exporter.uploadFn = func(spans []*tracepb.Span) {
		for _, s := range spans {
			exportMap.Store(s.SpanId, true)
		}
		wg.Done()

		// Don't complete the function until the WaitGroup is done.
		// This ensures the semaphore limiting the concurrent uploads is not
		// released by one goroutine completing before the other.
		wg.Wait()
	}
	trace.RegisterExporter(exporter)

	totalSpans := workers * spansPerWorker
	var expectedSpanIDs []string
	go func() {
		// Release enough spans to form two bundles
		for i := 0; i < totalSpans; i++ {
			_, span := trace.StartSpan(context.Background(), "span", trace.WithSampler(trace.AlwaysSample()))
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

func TestNewContext_Timeout(t *testing.T) {
	e := newTraceExporterWithClient(Options{
		Timeout: 10 * time.Millisecond,
	}, nil)
	ctx, cancel := newContextWithTimeout(e.o.Context, e.o.Timeout)
	defer cancel()
	select {
	case <-time.After(60 * time.Second):
		t.Fatal("should have timed out")
	case <-ctx.Done():
	}
}

func TestTraceSpansBufferMaxBytes(t *testing.T) {
	e := newTraceExporterWithClient(Options{
		Context:                  context.Background(),
		Timeout:                  10 * time.Millisecond,
		TraceSpansBufferMaxBytes: 20000,
	}, nil)
	waitCh := make(chan struct{})
	exported := 0
	e.uploadFn = func(spans []*tracepb.Span) {
		<-waitCh
		exported++
	}
	for i := 0; i < 10; i++ {
		e.ExportSpan(makeSampleSpanData(""))
	}
	close(waitCh)
	e.Flush()
	if exported != 2 {
		t.Errorf("exported = %d; want 2", exported)
	}
}

func TestTraceSpansUserAgent(t *testing.T) {
	e := newTraceExporterWithClient(Options{
		UserAgent: "OpenCensus Service",
		Context:   context.Background(),
		Timeout:   10 * time.Millisecond,
	}, nil)

	var got string
	// set user-agent attribute based on provided option
	e.uploadFn = func(spans []*tracepb.Span) {
		got = spans[0].Attributes.AttributeMap[agentLabel].GetStringValue().Value
	}
	e.ExportSpan(makeSampleSpanData(""))
	e.Flush()
	if want := "OpenCensus Service"; want != got {
		t.Fatalf("UserAgent Attribute = %q; want %q", got, want)
	}

	// if user-agent is already set, do not override
	e.uploadFn = func(spans []*tracepb.Span) {
		got = spans[0].Attributes.AttributeMap[agentLabel].GetStringValue().Value
	}
	e.ExportSpan(makeSampleSpanData("My Test Application"))
	e.Flush()
	if want := "My Test Application"; want != got {
		t.Fatalf("UserAgent Attribute = %q; want %q", got, want)
	}
}

func makeSampleSpanData(userAgent string) *trace.SpanData {
	sd := &trace.SpanData{
		Annotations:   make([]trace.Annotation, 32),
		Links:         make([]trace.Link, 32),
		MessageEvents: make([]trace.MessageEvent, 128),
		Attributes:    make(map[string]interface{}),
	}

	if userAgent != "" {
		sd.Attributes[agentLabel] = userAgent
	}

	for i := 0; i < 32; i++ {
		sd.Attributes[fmt.Sprintf("attribute-%d", i)] = ""
	}
	return sd
}
