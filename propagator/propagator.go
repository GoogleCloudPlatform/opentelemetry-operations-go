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
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TraceContextHeaderName is the HTTP header field for Google Cloud Trace
// https://cloud.google.com/trace/docs/setup#force-trace
const TraceContextHeaderName = "x-cloud-trace-context"

// TraceContextHeaderFormat is the regular expression pattan for valid Cloud Trace header value
const TraceContextHeaderFormat = "(?P<trace_id>[0-9a-f]{32})/(?P<span_id>[0-9]{1,20});o=(?P<trace_flags>[0-9])"

// TraceContextHeaderRe is a regular expression object of TraceContextHeaderFormat.
var TraceContextHeaderRe = regexp.MustCompile(TraceContextHeaderFormat)

var traceContextHeaders = []string{TraceContextHeaderName}

// CloudTraceFormatPropagator is a propagator for Cloud Trace format.
var CloudTraceFormatPropagator = &cloudTraceFormatPropagator{}

type cloudTraceFormatPropagator struct{}

func (p *cloudTraceFormatPropagator) getHeaderValue(carrier propagation.TextMapCarrier) string {
	header := carrier.Get(TraceContextHeaderName)
	if header != "" {
		return header
	}

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
func (p *cloudTraceFormatPropagator) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()

	header := fmt.Sprintf("%s/%s;o=%s",
		sc.TraceID().String(),
		sc.SpanID().String(),
		sc.TraceFlags().String(),
	)
	carrier.Set(TraceContextHeaderName, header)
}

// Extract extacts a context from the carrier if the header contains Google Cloud Trace header format.
func (p *cloudTraceFormatPropagator) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	header := p.getHeaderValue(carrier)
	if header != "" {
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
	if traceID == strings.Repeat("0", 32) || spanID == "0" {
		return ctx
	}

	tid, err := trace.TraceIDFromHex(traceID)
	if err != nil {
		return ctx
	}
	sid, err := trace.SpanIDFromHex(spanID)
	if err != nil {
		return ctx
	}
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
func (p *cloudTraceFormatPropagator) Fields() []string {
	return traceContextHeaders
}

var _ propagation.TextMapPropagator = (*cloudTraceFormatPropagator)(nil)
