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
	"net/http/httptest"
	"strconv"
	"testing"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestValidTraceContextHeaderFormats(t *testing.T) {

	headers := []struct {
		TraceID  string
		SpanID   string
		FlagPart string
		n        string
	}{
		{
			"d36a105d7002f0dee73c0dfb9553764a",
			"139592093",
			";o=1",
			"1",
		},
		{
			"d36a105d7002f0dee73c0dfb9553764a",
			"139592093",
			";o=0",
			"0",
		},
		{
			"d36a105d7002f0dee73c0dfb9553764a",
			"139592093",
			"",
			"",
		},
	}
	for _, h := range headers {
		header := fmt.Sprintf("%s/%s%s", h.TraceID, h.SpanID, h.FlagPart)
		match := TraceContextHeaderRe.FindStringSubmatch(header)
		if len(match) < 2 {
			t.Errorf("%s: did not match", header)
		}
		names := TraceContextHeaderRe.SubexpNames()

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

func TestCloudTraceContextHeaderExtract(t *testing.T) {
	headers := []struct {
		TraceID  string
		SpanID   string
		FlagPart string
	}{
		{
			"d36a105d7002f0dee73c0dfb9553764a",
			"139592093",
			"1",
		},
		{
			"d36a105d7002f0dee73c0dfb9553764a",
			"139592093",
			"0",
		},
	}

	for _, h := range headers {
		req := httptest.NewRequest("GET", "http://example.com", nil)
		value := fmt.Sprintf("%s/%s;o=%s", h.TraceID, h.SpanID, h.FlagPart)
		req.Header.Set("X-Cloud-Trace-Context", value)

		ctx := context.Background()
		propagator := New()
		ctx = propagator.Extract(ctx, propagation.HeaderCarrier(req.Header))

		sc := trace.SpanContextFromContext(ctx)

		sid, err := strconv.ParseUint(sc.SpanID().String(), 16, 64)
		if err != nil {
			t.Errorf("SpanID can't be convert to uint64: %v", err)
		}
		sidStr := fmt.Sprintf("%d", sid)

		flag := fmt.Sprintf("%d", sc.TraceFlags())

		if sc.TraceID().String() != h.TraceID {
			t.Errorf("TraceID unmatch: expected %v, but got %v", h.TraceID, sc.TraceID())
		}
		if sidStr != h.SpanID {
			t.Errorf("SpanID unmatch: expected %v, but got %v", h.SpanID, sidStr)
		}
		if flag != h.FlagPart {
			t.Errorf("FlagPart unmatch: expected %v, but got %v", h.FlagPart, flag)
		}
	}
}
