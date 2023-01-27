// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package trace

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/trace/apiv2/tracepb"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func testExporter() *traceExporter {
	return &traceExporter{
		o: &options{
			context:      context.Background(),
			mapAttribute: defaultAttributeMapping,
		},
	}
}

func genSpanContext() trace.SpanContext {
	tracer := sdktrace.NewTracerProvider().Tracer("")
	_, span := tracer.Start(context.Background(), "")
	return span.SpanContext()
}

func TestTraceProto_attributesFromSpans(t *testing.T) {
	t.Run("Should include instrumentation library", func(t *testing.T) {
		e := testExporter()
		startTime := time.Unix(1585674086, 1234)
		endTime := startTime.Add(10 * time.Second)
		traceState, _ := trace.ParseTraceState("key1=val1,key2=val2")
		rawSpan := tracetest.SpanStub{
			SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
				TraceID:    trace.TraceID{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F},
				SpanID:     trace.SpanID{0xFF, 0xFE, 0xFD, 0xFC, 0xFB, 0xFA, 0xF9, 0xF8},
				TraceState: traceState,
			}),
			SpanKind:  trace.SpanKindServer,
			Name:      "span data to span data",
			StartTime: startTime,
			EndTime:   endTime,
			Status: sdktrace.Status{
				Code:        codes.Error,
				Description: "utterly unrecognized",
			},
			Attributes: []attribute.KeyValue{
				attribute.Int64("timeout_ns", 12e9),
				attribute.Bool("conflict", true),
			},
			Resource: resource.NewWithAttributes(
				"http://example.com/custom-resource-schema",
				attribute.String("rk1", "rv1"),
				attribute.Int64("rk2", 5),
				attribute.StringSlice("rk3", []string{"sv1", "sv2"}),
				attribute.Bool("conflict", false),
			),
			InstrumentationLibrary: instrumentation.Library{
				Name:    "lib-name",
				Version: "v0.0.1",
			},
		}
		span := e.ConvertSpan(context.Background(), rawSpan.Snapshot())
		assert.NotNil(t, span)

		// Ensure agent key is created.
		assert.Contains(t, span.Attributes.AttributeMap, "g.co/agent")

		// Ensure resource keys are copied.
		assert.Contains(t, span.Attributes.AttributeMap, "rk1")
		assert.Contains(t, span.Attributes.AttributeMap, "rk2")
		// TODO - Support JSON rendering of list-attributes.
		// assert.Contains(t, span.Attributes.AttributeMap, "rk3")

		// Ensure instrumentation library values are copied.
		assert.Contains(t, span.Attributes.AttributeMap, "otel.scope.name")
		assert.Equal(t, span.Attributes.AttributeMap["otel.scope.name"].GetStringValue().Value, "lib-name")
		assert.Contains(t, span.Attributes.AttributeMap, "otel.scope.version")

		// Ensure monitored resource labels are created.
		assert.Contains(t, span.Attributes.AttributeMap, "g.co/r/generic_node/location")
		assert.Equal(t, span.Attributes.AttributeMap["g.co/r/generic_node/location"].GetStringValue().Value, "global")

		// Ensure span attribute "wins" over resource attribute.
		assert.Contains(t, span.Attributes.AttributeMap, "conflict")
		assert.Equal(t, span.Attributes.AttributeMap["conflict"].GetBoolValue(), true)
	})
}

func TestTraceProto_linksProtoFromLinks(t *testing.T) {
	t.Run("Should be nil when no links", func(t *testing.T) {
		e := testExporter()
		assert.Nil(t, e.linksProtoFromLinks([]sdktrace.Link{}))
	})

	t.Run("Can convert one link", func(t *testing.T) {
		e := testExporter()
		spanContext := genSpanContext()
		link := sdktrace.Link{
			SpanContext: spanContext,
			Attributes: []attribute.KeyValue{
				attribute.String("hello", "world"),
			},
		}
		linksPb := e.linksProtoFromLinks([]sdktrace.Link{link})

		assert.NotNil(t, linksPb)
		assert.EqualValues(t, linksPb.DroppedLinksCount, 0)
		assert.Len(t, linksPb.Link, 1)
		assert.Equal(t, spanContext.TraceID().String(), linksPb.Link[0].TraceId)
		assert.Equal(t, spanContext.SpanID().String(), linksPb.Link[0].SpanId)
		assert.Equal(t, tracepb.Span_Link_TYPE_UNSPECIFIED, linksPb.Link[0].Type)
		assert.Len(t, linksPb.Link[0].Attributes.AttributeMap, 1)
		assert.Contains(t, linksPb.Link[0].Attributes.AttributeMap, "hello")
		assert.Equal(
			t,
			"world",
			linksPb.Link[0].Attributes.AttributeMap["hello"].GetStringValue().Value,
		)
	})

	t.Run("Drops links when there are more than 128", func(t *testing.T) {
		e := testExporter()
		var links []sdktrace.Link
		for i := 0; i < 148; i++ {
			links = append(
				links,
				sdktrace.Link{
					SpanContext: genSpanContext(),
					Attributes: []attribute.KeyValue{
						attribute.String("hello", "world"),
					},
				})
		}
		linksPb := e.linksProtoFromLinks(links)
		assert.NotNil(t, linksPb)
		assert.EqualValues(t, linksPb.DroppedLinksCount, 20)
		assert.Len(t, linksPb.Link, maxNumLinks)
	})
}
