// Copyright 2021 OpenTelemetry Authors
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

package collector

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	apitrace "go.opentelemetry.io/otel/trace"
)

func pdataResourceSpansToOTSpanData(rs ptrace.ResourceSpans) []sdktrace.ReadOnlySpan {
	resource := rs.Resource()
	var sds []sdktrace.ReadOnlySpan
	ss := rs.ScopeSpans()
	for i := 0; i < ss.Len(); i++ {
		s := ss.At(i)
		spans := s.Spans()
		for j := 0; j < spans.Len(); j++ {
			sd := pdataSpanToOTSpanData(spans.At(j), resource, s.Scope())
			sds = append(sds, sd)
		}
	}

	return sds
}

func pdataSpanToOTSpanData(
	span ptrace.Span,
	resource pcommon.Resource,
	is pcommon.InstrumentationScope,
) spanSnapshot {
	sc := apitrace.SpanContextConfig{
		TraceID: [16]byte(span.TraceID()),
		SpanID:  [8]byte(span.SpanID()),
	}
	parentSc := apitrace.SpanContextConfig{
		TraceID: [16]byte(span.TraceID()),
		SpanID:  [8]byte(span.ParentSpanID()),
	}
	startTime := time.Unix(0, int64(span.StartTimestamp()))
	endTime := time.Unix(0, int64(span.EndTimestamp()))
	// TODO: Decide if ignoring the error is fine.
	r, _ := sdkresource.New(
		context.Background(),
		sdkresource.WithAttributes(pdataAttributesToOTAttributes(pcommon.NewMap(), resource)...),
	)

	status := span.Status()
	return spanSnapshot{
		spanContext:          apitrace.NewSpanContext(sc),
		parent:               apitrace.NewSpanContext(parentSc),
		spanKind:             pdataSpanKindToOTSpanKind(span.Kind()),
		startTime:            startTime,
		endTime:              endTime,
		name:                 span.Name(),
		attributes:           pdataAttributesToOTAttributes(span.Attributes(), resource),
		links:                pdataLinksToOTLinks(span.Links()),
		events:               pdataEventsToOTMessageEvents(span.Events()),
		droppedAttributes:    int(span.DroppedAttributesCount()),
		droppedMessageEvents: int(span.DroppedEventsCount()),
		droppedLinks:         int(span.DroppedLinksCount()),
		resource:             r,
		instrumentationScope: instrumentationScopeLabels(is),
		status: sdktrace.Status{
			Code:        pdataStatusCodeToOTCode(status.Code()),
			Description: status.Message(),
		},
	}
}

func instrumentationScopeLabels(is pcommon.InstrumentationScope) instrumentation.Scope {
	scope := instrumentation.Scope{}
	if len(is.Name()) > 0 {
		scope.Name = is.Name()
	}
	if len(is.Version()) > 0 {
		scope.Version = is.Version()
	}
	return scope
}

func pdataSpanKindToOTSpanKind(k ptrace.SpanKind) apitrace.SpanKind {
	switch k {
	case ptrace.SpanKindUnspecified:
		return apitrace.SpanKindInternal
	case ptrace.SpanKindInternal:
		return apitrace.SpanKindInternal
	case ptrace.SpanKindServer:
		return apitrace.SpanKindServer
	case ptrace.SpanKindClient:
		return apitrace.SpanKindClient
	case ptrace.SpanKindProducer:
		return apitrace.SpanKindProducer
	case ptrace.SpanKindConsumer:
		return apitrace.SpanKindConsumer
	default:
		return apitrace.SpanKindUnspecified
	}
}

func pdataStatusCodeToOTCode(c ptrace.StatusCode) codes.Code {
	switch c {
	case ptrace.StatusCodeOk:
		return codes.Ok
	case ptrace.StatusCodeError:
		return codes.Error
	default:
		return codes.Unset
	}
}

func pdataAttributesToOTAttributes(attrs pcommon.Map, resource pcommon.Resource) []attribute.KeyValue {
	otAttrs := make([]attribute.KeyValue, 0, attrs.Len())
	appendAttrs := func(m pcommon.Map) {
		m.Range(func(k string, v pcommon.Value) bool {
			if (k == string(semconv.ServiceNameKey) ||
				k == string(semconv.ServiceNamespaceKey) ||
				k == string(semconv.ServiceInstanceIDKey)) &&
				len(v.AsString()) == 0 {
				return true
			}
			switch v.Type() {
			case pcommon.ValueTypeStr:
				otAttrs = append(otAttrs, attribute.String(k, v.Str()))
			case pcommon.ValueTypeBool:
				otAttrs = append(otAttrs, attribute.Bool(k, v.Bool()))
			case pcommon.ValueTypeInt:
				otAttrs = append(otAttrs, attribute.Int64(k, v.Int()))
			case pcommon.ValueTypeDouble:
				otAttrs = append(otAttrs, attribute.Float64(k, v.Double()))
			default:
				otAttrs = append(otAttrs, attribute.String(k, v.AsString()))
			}
			return true
		})
	}
	appendAttrs(resource.Attributes())
	appendAttrs(attrs)
	return otAttrs
}

func pdataLinksToOTLinks(links ptrace.SpanLinkSlice) []sdktrace.Link {
	size := links.Len()
	otLinks := make([]sdktrace.Link, 0, size)
	for i := 0; i < size; i++ {
		link := links.At(i)
		sc := apitrace.SpanContextConfig{}
		sc.TraceID = [16]byte(link.TraceID())
		sc.SpanID = [8]byte(link.SpanID())
		otLinks = append(otLinks, sdktrace.Link{
			SpanContext: apitrace.NewSpanContext(sc),
			Attributes:  pdataAttributesToOTAttributes(link.Attributes(), pcommon.NewResource()),
		})
	}
	return otLinks
}

func pdataEventsToOTMessageEvents(events ptrace.SpanEventSlice) []sdktrace.Event {
	size := events.Len()
	otEvents := make([]sdktrace.Event, 0, size)
	for i := 0; i < size; i++ {
		event := events.At(i)
		otEvents = append(otEvents, sdktrace.Event{
			Name:       event.Name(),
			Attributes: pdataAttributesToOTAttributes(event.Attributes(), pcommon.NewResource()),
			Time:       time.Unix(0, int64(event.Timestamp())),
		})
	}
	return otEvents
}
