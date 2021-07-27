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

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	tracepb "google.golang.org/genproto/googleapis/devtools/cloudtrace/v2"
)

func genSpanContext() trace.SpanContext {
	tracer := sdktrace.NewTracerProvider().Tracer("")
	_, span := tracer.Start(context.Background(), "")
	return span.SpanContext()
}

func TestTraceProto_linksProtoFromLinks(t *testing.T) {
	t.Run("Should be nil when no links", func(t *testing.T) {
		assert.Nil(t, linksProtoFromLinks([]sdktrace.Link{}))
	})

	t.Run("Can convert one link", func(t *testing.T) {
		spanContext := genSpanContext()
		link := sdktrace.Link{
			SpanContext: spanContext,
			Attributes: []attribute.KeyValue{
				attribute.String("hello", "world"),
			},
		}
		linksPb := linksProtoFromLinks([]sdktrace.Link{link})

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
		linksPb := linksProtoFromLinks(links)
		assert.NotNil(t, linksPb)
		assert.EqualValues(t, linksPb.DroppedLinksCount, 20)
		assert.Len(t, linksPb.Link, maxNumLinks)
	})
}
