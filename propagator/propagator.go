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

package propagator

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TraceContextHeaderName is the HTTP header field for Google Cloud Trace
// https://cloud.google.com/trace/docs/setup#force-trace
const TraceContextHeaderName = "x-cloud-trace-context"

// TraceContextHeaderFormat is the regular expression pattan for valid Cloud Trace header value
const TraceContextHeaderFormat = "(?P<trace_id>[0-9a-f]{32})/(?P<span_id>[0-9]{1,20})(;o=(?P<trace_flags>[0-9]))?"

// TraceContextHeaderRe is a regular expression object of TraceContextHeaderFormat.
var TraceContextHeaderRe = regexp.MustCompile(TraceContextHeaderFormat)

var traceContextHeaders = []string{TraceContextHeaderName}

type CloudTraceFormatPropagator struct{}

func New() propagation.TextMapPropagator {
	return CloudTraceFormatPropagator{}
}

func (p CloudTraceFormatPropagator) getHeaderValue(carrier propagation.TextMapCarrier) string {
	header := carrier.Get(TraceContextHeaderName)
	if header != "" {
		return header
	}

	// Currently, header name can be in different cases (eg. x-cloud-trace-context),
	// the following part is for handling those cases.
	for _, key := range carrier.Keys() {
		if strings.ToLower(key) == TraceContextHeaderName {
			header = carrier.Get(key)
			if header != "" {
				return header
			}
		}
	}
	return ""
}

// Inject injects a context to the carrier following Google Cloud Trace format.
func (p CloudTraceFormatPropagator) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()

	// https://cloud.google.com/trace/docs/setup#force-trace
	// Trace ID: 32-char hexadecimal value representing a 128-bit number.
	// Span ID: decimal representation of the unsigned interger.
	sidHex := sc.SpanID().String()
	sid, err := strconv.ParseUint(sidHex, 16, 64)
	if err != nil {
		return
	}
	header := fmt.Sprintf("%s/%d;o=%s",
		sc.TraceID().String(),
		sid,
		sc.TraceFlags().String(),
	)
	carrier.Set(TraceContextHeaderName, header)
}

// Extract extacts a context from the carrier if the header contains Google Cloud Trace header format.
func (p CloudTraceFormatPropagator) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	header := p.getHeaderValue(carrier)
	if header == "" {
		return ctx
	}

	match := TraceContextHeaderRe.FindStringSubmatch(header)
	if match == nil {
		return ctx
	}
	names := TraceContextHeaderRe.SubexpNames()
	var traceID, spanID, traceFlags string
	for i, n := range names {
		switch n {
		case "trace_id":
			traceID = match[i]
		case "span_id":
			spanID = match[i]
		case "trace_flags":
			traceFlags = match[i]
		}
	}
	// non-recording Span
	if traceID == strings.Repeat("0", 32) || spanID == "0" {
		return ctx
	}

	// https://cloud.google.com/trace/docs/setup#force-trace
	// Trace ID: 32-char hexadecimal value representing a 128-bit number.
	// Span ID: decimal representation of the unsigned interger.
	tid, err := trace.TraceIDFromHex(traceID)
	if err != nil {
		log.Printf("CloudTraceFormatPropagator: invalid trace id %#v: %v", traceID, err)
		return ctx
	}
	sidUint, err := strconv.ParseUint(spanID, 10, 64)
	if err != nil {
		log.Printf("CloudTraceFormatPropagator: on span ID conversion: %v", err)
		return ctx
	}
	sid, err := trace.SpanIDFromHex(fmt.Sprintf("%016x", sidUint))
	if err != nil {
		log.Printf("CloudTraceFormatPropagator: on SpanIDFromHex %v", err)
		return ctx
	}
	// XCTC's TRA
	// https://cloud.google.com/trace/docs/setup#force-trace
	tf := trace.TraceFlags(0x01)
	if traceFlags == "0" {
		tf = trace.TraceFlags(0x00)
	}
	scConfig := trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: tf,
		Remote:     true,
	}
	sc := trace.NewSpanContext(scConfig)
	return trace.ContextWithRemoteSpanContext(ctx, sc)
}

// Fields just returns the header name.
func (p CloudTraceFormatPropagator) Fields() []string {
	return traceContextHeaders
}

var _ propagation.TextMapPropagator = CloudTraceFormatPropagator{}
