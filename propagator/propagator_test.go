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
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	validTraceIDStr = "d36a105d7002f0dee73c0dfb9553764a"
	validSpanIDStr  = "139592093"
	xctcCamel       = "X-Cloud-Trace-Context"
)

var (
	validTraceID = trace.TraceID{0xd3, 0x6a, 0x10, 0x5d, 0x70, 0x02, 0xf0, 0xde, 0xe7, 0x3c, 0x0d, 0xfb, 0x95, 0x53, 0x76, 0x4a}
	validSpanID  = trace.SpanID{0x00, 0x00, 0x00, 0x00, 0x08, 0x52, 0x01, 0x9d}
)

func TestGetHeaderValue(t *testing.T) {
	// carrier.Get called in getHeaderValue should handle case sensitivity by spec.
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/context/api-propagators.md#get
	testCases := []string{
		"X-Cloud-Trace-Context",
		"x-cloud-trace-context",
		"X-CLOUD-TRACE-CONTEXT",
	}

	dummyHeader := "dummy"

	for _, tc := range testCases {
		propagator := &CloudTraceFormatPropagator{}
		carrier := propagation.HeaderCarrier{}
		carrier.Set(tc, dummyHeader)

		value := propagator.getHeaderValue(carrier)
		if value != dummyHeader {
			t.Errorf("Expected %s, but got %s", dummyHeader, value)
		}
	}
}

func TestValidTraceContextHeaderFormats(t *testing.T) {
	headers := []struct {
		TraceID  string
		SpanID   string
		FlagPart string
		n        string
	}{
		{
			validTraceIDStr,
			validSpanIDStr,
			";o=1",
			"1",
		},
		{
			validTraceIDStr,
			validSpanIDStr,
			";o=0",
			"0",
		},
		{
			validTraceIDStr,
			validSpanIDStr,
			"",
			"",
		},
	}
	for _, h := range headers {
		header := fmt.Sprintf("%s/%s%s", h.TraceID, h.SpanID, h.FlagPart)
		match := traceContextHeaderRe.FindStringSubmatch(header)
		if len(match) < 2 {
			t.Errorf("%s: did not match", header)
		}
		names := traceContextHeaderRe.SubexpNames()

		for i, n := range match {
			switch names[i] {
			case "trace_id":
				if n != h.TraceID {
					t.Errorf("Expected %s, but got %s", h.TraceID, n)
				}
			case "span_id":
				if n != h.SpanID {
					t.Errorf("Expected %s, but got %s", h.SpanID, n)
				}
			case "trace_flags":
				if n != h.n {
					t.Errorf("Expected %s, but got %s", h.n, n)
				}
			}
		}
	}
}

func TestOneWayPropagatorExtract(t *testing.T) {
	propagator := CloudTraceOneWayPropagator{}
	testCases := []struct {
		name    string
		headers map[string]string
		want    trace.SpanContextConfig
	}{
		{
			"xctc without flag",
			map[string]string{
				"x-cloud-trace-context": fmt.Sprintf("%s/%s", validTraceIDStr, validSpanIDStr)},
			trace.SpanContextConfig{
				TraceID:    validTraceID,
				SpanID:     validSpanID,
				TraceFlags: 00,
			},
		},
		{
			"xctc with flag",
			map[string]string{
				"x-cloud-trace-context": fmt.Sprintf("%s/%s;o=1", validTraceIDStr, validSpanIDStr)},
			trace.SpanContextConfig{
				TraceID:    validTraceID,
				SpanID:     validSpanID,
				TraceFlags: 01,
			},
		},
	}

	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com", nil)
			for k, v := range c.headers {
				req.Header.Set(k, v)
			}

			ctx := context.Background()
			ctx = propagator.Extract(ctx, propagation.HeaderCarrier(req.Header))

			sc := trace.SpanContextFromContext(ctx)

			if sc.TraceID() != c.want.TraceID {
				t.Errorf("TraceID mismatch: expected %v, but got %v", c.want.TraceID, sc.TraceID())
			}
			if sc.SpanID() != c.want.SpanID {
				t.Errorf("SpanID mismatch: expected %v, but got %v", c.want.SpanID, sc.SpanID())
			}
			if sc.TraceFlags() != c.want.TraceFlags {
				t.Errorf("FlagPart mismatch: expected %v, but got %v", c.want.TraceFlags, sc.TraceFlags())
			}
			if sc.TraceState().String() != c.headers["tracestate"] {
				t.Errorf("TraceState mismatch: expected %v, but got %v", c.headers["tracestate"], sc.TraceState().String())
			}
		})
	}
}

func TestOneWayPropagatorInject(t *testing.T) {
	propagator := CloudTraceOneWayPropagator{}

	testCases := []struct {
		wantHeaders map[string]string
		name        string
		scc         trace.SpanContextConfig
	}{
		{
			name: "valid TraceID and SpanID with sampled flag doean't inject",
			scc: trace.SpanContextConfig{
				TraceID:    validTraceID,
				SpanID:     validSpanID,
				TraceFlags: trace.FlagsSampled,
			},
			wantHeaders: map[string]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			header := http.Header{}
			ctx := trace.ContextWithSpanContext(
				context.Background(),
				trace.NewSpanContext(tc.scc),
			)
			propagator.Inject(ctx, propagation.HeaderCarrier(header))

			for h, v := range tc.wantHeaders {
				result, want := header.Get(h), v
				if diff := cmp.Diff(want, result); diff != "" {
					t.Errorf("%v, header: %s diff: %s", tc.name, h, diff)
				}
			}
		})
	}
}

func TestCloudTraceContextHeaderExtract(t *testing.T) {
	testCases := []struct {
		key      string
		traceID  string
		spanID   string
		flagPart string
	}{
		{
			"X-Cloud-Trace-Context",
			validTraceIDStr,
			validSpanIDStr,
			"1",
		},
		{
			"X-Cloud-Trace-Context",
			validTraceIDStr,
			validSpanIDStr,
			"0",
		},
		{
			"x-cloud-trace-context",
			validTraceIDStr,
			validSpanIDStr,
			"0",
		},
	}

	for _, c := range testCases {
		req := httptest.NewRequest("GET", "http://example.com", nil)
		value := fmt.Sprintf("%s/%s;o=%s", c.traceID, c.spanID, c.flagPart)
		req.Header.Set(c.key, value)

		ctx := context.Background()
		propagator := CloudTraceFormatPropagator{}
		ctx = propagator.Extract(ctx, propagation.HeaderCarrier(req.Header))

		sc := trace.SpanContextFromContext(ctx)

		sid, err := strconv.ParseUint(sc.SpanID().String(), 16, 64)
		if err != nil {
			t.Errorf("SpanID can't be convert to uint64: %v", err)
		}
		sidStr := fmt.Sprintf("%d", sid)

		flag := fmt.Sprintf("%d", sc.TraceFlags())

		if sc.TraceID().String() != c.traceID {
			t.Errorf("TraceID unmatch: expected %v, but got %v", c.traceID, sc.TraceID())
		}
		if sidStr != c.spanID {
			t.Errorf("SpanID unmatch: expected %v, but got %v", c.spanID, sidStr)
		}
		if flag != c.flagPart {
			t.Errorf("FlagPart unmatch: expected %v, but got %v", c.flagPart, flag)
		}
	}
}

func TestCloudTraceContextHeaderInject(t *testing.T) {
	propagator := CloudTraceFormatPropagator{}

	testCases := []struct {
		wantHeaders map[string]string
		name        string
		scc         trace.SpanContextConfig
	}{
		{
			name: "valid TraceID and SpanID with sampled flag",
			scc: trace.SpanContextConfig{
				TraceID:    validTraceID,
				SpanID:     validSpanID,
				TraceFlags: trace.FlagsSampled,
			},
			wantHeaders: map[string]string{
				TraceContextHeaderName: fmt.Sprintf("%s/%s;o=1", validTraceIDStr, validSpanIDStr),
			},
		},
		{
			name: "valid TraceID and SpanID without sampled flag",
			scc: trace.SpanContextConfig{
				TraceID: validTraceID,
				SpanID:  validSpanID,
			},
			wantHeaders: map[string]string{
				TraceContextHeaderName: fmt.Sprintf("%s/%s;o=0", validTraceIDStr, validSpanIDStr),
			},
		},
		{
			name: "valid TraceID and SpanID with sampled flag and proper camelcase header name",
			scc: trace.SpanContextConfig{
				TraceID:    validTraceID,
				SpanID:     validSpanID,
				TraceFlags: trace.FlagsSampled,
			},
			wantHeaders: map[string]string{
				xctcCamel: fmt.Sprintf("%s/%s;o=1", validTraceIDStr, validSpanIDStr),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			header := http.Header{}
			ctx := trace.ContextWithSpanContext(
				context.Background(),
				trace.NewSpanContext(tc.scc),
			)
			propagator.Inject(ctx, propagation.HeaderCarrier(header))

			for h, v := range tc.wantHeaders {
				result, want := header.Get(h), v
				if diff := cmp.Diff(want, result); diff != "" {
					t.Errorf("%v, header: %s diff: %s", tc.name, h, diff)
				}
			}
		})
	}
}

func TestCloudTraceFormatPropagator(t *testing.T) {
	testCases := []struct {
		name     string
		key      string
		traceID  string
		spanID   string
		flagPart string
		scc      trace.SpanContextConfig
	}{
		{
			"camelcase, valid traceID and SpanID with flag",
			"X-Cloud-Trace-Context",
			validTraceIDStr,
			validSpanIDStr,
			";o=1",
			trace.SpanContextConfig{
				TraceID:    validTraceID,
				SpanID:     validSpanID,
				TraceFlags: trace.FlagsSampled,
				Remote:     true,
			},
		},
		{
			"lowercase, valid traceID and SpanID with flag",
			"x-cloud-trace-context",
			validTraceIDStr,
			validSpanIDStr,
			";o=1",
			trace.SpanContextConfig{
				TraceID:    validTraceID,
				SpanID:     validSpanID,
				TraceFlags: trace.FlagsSampled,
				Remote:     true,
			},
		},
		{
			"camelcase, valid traceID and SpanID without flag",
			"X-Cloud-Trace-Context",
			validTraceIDStr,
			validSpanIDStr,
			"",
			trace.SpanContextConfig{
				TraceID: validTraceID,
				SpanID:  validSpanID,
				Remote:  true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com", nil)
			req.Header.Set(tc.key, fmt.Sprintf("%s/%s%s", tc.traceID, tc.spanID, tc.flagPart))

			sc, err := SpanContextFromRequest(req)
			if err != nil {
				t.Errorf("%v: CloudTraceFormatPropagator returned non nil: %v", tc.name, err)
			}
			want := trace.NewSpanContext(tc.scc)
			if diff := cmp.Diff(want, sc); diff != "" {
				t.Errorf("%v: CloudTraceFormatPropagator returned diff: %v", tc.name, diff)
			}
		})
	}
}
