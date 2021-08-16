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
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TraceContextHeaderName is the HTTP header field for Google Cloud Trace
// https://cloud.google.com/trace/docs/setup#force-trace
const TraceContextHeaderName = "x-cloud-trace-context"

// traceContextHeaderFormat is the regular expression pattern for valid Cloud Trace header value
const traceContextHeaderFormat = "^(?P<trace_id>[0-9a-f]{32})/(?P<span_id>[0-9]{1,20})(;o=(?P<trace_flags>[0-9]))?$"

// traceContextHeaderRe is a regular expression object of TraceContextHeaderFormat.
var traceContextHeaderRe = regexp.MustCompile(traceContextHeaderFormat)

// traceContextHeaders is the list of headers that are propagated. Cloud Trace only requires
// one element in the list.
var traceContextHeaders = []string{TraceContextHeaderName}

type errInvalidHeader struct {
	header string
}

func (e errInvalidHeader) Error() string {
	return fmt.Sprintf("invalid header %s", e.header)
}

// CloudTraceFormatPropagator is a TextMapPropagator that injects/extracts a context to/from the carrier
// following Google Cloud Trace format.
type CloudTraceFormatPropagator struct{}

// New returns a new CloudTraceFormatPropagator.
func New() CloudTraceFormatPropagator {
	return CloudTraceFormatPropagator{}
}

func (p CloudTraceFormatPropagator) getHeaderValue(carrier propagation.TextMapCarrier) string {
	return carrier.Get(TraceContextHeaderName)
}

// Inject injects a context to the carrier following Google Cloud Trace format.
// In this method, SpanID is expected to be stored in big endian.
func (p CloudTraceFormatPropagator) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()

	// https://cloud.google.com/trace/docs/setup#force-trace
	// Trace ID: 32-char hexadecimal value representing a 128-bit number.
	// Span ID: decimal representation of the unsigned interger.
	ary := sc.SpanID()
	sid := binary.BigEndian.Uint64(ary[:])

	flag, err := strconv.Atoi(sc.TraceFlags().String())
	if err != nil {
		return
	}
	header := fmt.Sprintf("%s/%d;o=%d",
		sc.TraceID().String(),
		sid,
		flag,
	)
	carrier.Set(TraceContextHeaderName, header)
}

// spanContextFromHeader creates trace.SpanContext from XCTC header value.
func (p CloudTraceFormatPropagator) spanContextFromHeader(header string) (trace.SpanContext, error) {
	match := traceContextHeaderRe.FindStringSubmatch(header)
	if match == nil {
		return trace.SpanContext{}, errInvalidHeader{header}
	}
	names := traceContextHeaderRe.SubexpNames()
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
		return trace.SpanContext{}, errInvalidHeader{header}
	}

	// https://cloud.google.com/trace/docs/setup#force-trace
	// Trace ID: 32-char hexadecimal value representing a 128-bit number.
	// Span ID: decimal representation of the unsigned interger.
	tid, err := trace.TraceIDFromHex(traceID)
	if err != nil {
		log.Printf("CloudTraceFormatPropagator: invalid trace id %#v: %v", traceID, err)
		return trace.SpanContext{}, errInvalidHeader{header}
	}
	sidUint, err := strconv.ParseUint(spanID, 10, 64)
	if err != nil {
		log.Printf("CloudTraceFormatPropagator: on span ID conversion: %v", err)
		return trace.SpanContext{}, errInvalidHeader{header}
	}
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, sidUint)
	ary := [8]byte{}
	copy(ary[:], buf)
	sid := trace.SpanID(ary)

	// XCTC's TRACE_TRUE option
	// https://cloud.google.com/trace/docs/setup#force-trace
	tf := trace.TraceFlags(0x01)
	if traceFlags == "0" || traceFlags == "" {
		tf = trace.TraceFlags(0x00)
	}
	scConfig := trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: tf,
		Remote:     true,
	}
	return trace.NewSpanContext(scConfig), nil
}

// SpanContextFromRequest extracts a trace.SpanContext from the HTTP request req.
// In this method, SpanID is expected to be stored in big endian.
func (p CloudTraceFormatPropagator) SpanContextFromRequest(req *http.Request) (trace.SpanContext, error) {
	h := req.Header.Get(TraceContextHeaderName)
	return p.spanContextFromHeader(h)
}

// Extract extacts a context from the carrier if the header contains Google Cloud Trace header format.
// In this method, SpanID is expected to be stored in big endian.
func (p CloudTraceFormatPropagator) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	header := p.getHeaderValue(carrier)
	if header == "" {
		return ctx
	}
	sc, err := p.spanContextFromHeader(header)
	if err != nil {
		log.Printf("CloudTraceFormatPropagator: %v", err)
		return ctx
	}
	return trace.ContextWithRemoteSpanContext(ctx, sc)
}

// Fields just returns the header name.
func (p CloudTraceFormatPropagator) Fields() []string {
	return traceContextHeaders
}

// Confirming if CloudTraceFormatPropagator satisifies the TextMapPropagator interface.
var _ propagation.TextMapPropagator = CloudTraceFormatPropagator{}
