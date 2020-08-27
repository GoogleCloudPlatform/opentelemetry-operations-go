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
	"fmt"
	"math"
	"strconv"
	"time"
	"unicode/utf8"

	timestamppb "github.com/golang/protobuf/ptypes/timestamp"
	wrapperspb "github.com/golang/protobuf/ptypes/wrappers"
	tracepb "google.golang.org/genproto/googleapis/devtools/cloudtrace/v2"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"

	"go.opentelemetry.io/otel/label"
	opentelemetry "go.opentelemetry.io/otel/sdk"
	export "go.opentelemetry.io/otel/sdk/export/trace"
)

const (
	maxAnnotationEventsPerSpan = 32
	// TODO(ymotongpoo): uncomment this after gRPC trace get supported.
	// maxMessageEventsPerSpan    = 128
	maxAttributeStringValue = 256
	agentLabel              = "g.co/agent"

	// Attributes recorded on the span for the requests.
	// Only trace exporters will need them.
	HostAttribute       = "http.host"
	MethodAttribute     = "http.method"
	PathAttribute       = "http.path"
	URLAttribute        = "http.url"
	UserAgentAttribute  = "http.user_agent"
	StatusCodeAttribute = "http.status_code"

	labelHTTPHost       = `/http/host`
	labelHTTPMethod     = `/http/method`
	labelHTTPStatusCode = `/http/status_code`
	labelHTTPPath       = `/http/path`
	labelHTTPUserAgent  = `/http/user_agent`

	version = "0.1.0"
)

var userAgent = fmt.Sprintf("opentelemetry-go %s; google-cloud-trace-exporter %s", opentelemetry.Version(), version)

func generateDisplayName(s *export.SpanData, format DisplayNameFormatter) string {
	if format != nil {
		return format(s)
	}
	switch s.SpanKind {
	// TODO(ymotongpoo): add cases for "Send" and "Recv".
	default:
		return fmt.Sprintf("Span.%s-%s", s.SpanKind, s.Name)
	}
}

// If there are duplicate keys present in the list of attributes,
// then the first value found for the key is preserved.
func injectLabelsFromResources(sd *export.SpanData) {
	if sd.Resource.Len() == 0 {
		return
	}
	uniqueAttrs := make(map[label.Key]bool, len(sd.Attributes))
	for _, attr := range sd.Attributes {
		uniqueAttrs[attr.Key] = true
	}
	for _, attr := range sd.Resource.Attributes() {
		if uniqueAttrs[attr.Key] {
			continue // skip resource attributes which conflict with span attributes
		}
		uniqueAttrs[attr.Key] = true
		sd.Attributes = append(sd.Attributes, attr)
	}
}

func protoFromSpanData(s *export.SpanData, projectID string, format DisplayNameFormatter) *tracepb.Span {
	if s == nil {
		return nil
	}

	injectLabelsFromResources(s)

	traceIDString := s.SpanContext.TraceID.String()
	spanIDString := s.SpanContext.SpanID.String()

	displayName := generateDisplayName(s, format)

	sp := &tracepb.Span{
		Name:                    "projects/" + projectID + "/traces/" + traceIDString + "/spans/" + spanIDString,
		SpanId:                  spanIDString,
		DisplayName:             trunc(displayName, 128),
		StartTime:               timestampProto(s.StartTime),
		EndTime:                 timestampProto(s.EndTime),
		SameProcessAsParentSpan: &wrapperspb.BoolValue{Value: !s.HasRemoteParent},
	}
	if s.ParentSpanID != s.SpanContext.SpanID && s.ParentSpanID.IsValid() {
		sp.ParentSpanId = s.ParentSpanID.String()
	}
	if s.StatusCode != codes.OK {
		sp.Status = &statuspb.Status{Code: int32(s.StatusCode), Message: s.StatusMessage}
	}
	if s.StatusCode == codes.OK && s.StatusMessage != "" {
		sp.Status = &statuspb.Status{Code: int32(s.StatusCode), Message: s.StatusMessage}
	}

	copyAttributes(&sp.Attributes, s.Attributes)
	// NOTE(ymotongpoo): omitting copyMonitoringReesourceAttributes()

	var annotations, droppedAnnotationsCount int
	es := s.MessageEvents
	for i, e := range es {
		if annotations >= maxAnnotationEventsPerSpan {
			droppedAnnotationsCount = len(es) - i
			break
		}
		annotation := &tracepb.Span_TimeEvent_Annotation{Description: trunc(e.Name, maxAttributeStringValue)}
		copyAttributes(&annotation.Attributes, e.Attributes)
		event := &tracepb.Span_TimeEvent{
			Time:  timestampProto(e.Time),
			Value: &tracepb.Span_TimeEvent_Annotation_{Annotation: annotation},
		}
		annotations++
		if sp.TimeEvents == nil {
			sp.TimeEvents = &tracepb.Span_TimeEvents{}
		}
		sp.TimeEvents.TimeEvent = append(sp.TimeEvents.TimeEvent, event)
	}

	if sp.Attributes == nil {
		sp.Attributes = &tracepb.Span_Attributes{
			AttributeMap: make(map[string]*tracepb.AttributeValue),
		}
	}

	// Only set the agent label if it is not already set. That enables the
	// OpenTelemery service/collector to set the agent label based on the library that
	// sent the span to the service.
	if _, hasAgent := sp.Attributes.AttributeMap[agentLabel]; !hasAgent {
		sp.Attributes.AttributeMap[agentLabel] = &tracepb.AttributeValue{
			Value: &tracepb.AttributeValue_StringValue{
				StringValue: trunc(userAgent, maxAttributeStringValue),
			},
		}
	}

	// TODO(ymotongpoo): add implementations for Span_TimeEvent_MessageEvent_
	// once OTel finish implementations for gRPC.

	if droppedAnnotationsCount != 0 {
		if sp.TimeEvents == nil {
			sp.TimeEvents = &tracepb.Span_TimeEvents{}
		}
		sp.TimeEvents.DroppedAnnotationsCount = clip32(droppedAnnotationsCount)
	}

	// TODO(ymotongpoo): add implementations for Links

	return sp
}

// timestampProto creates a timestamp proto for a time.Time.
func timestampProto(t time.Time) *timestamppb.Timestamp {
	return &timestamppb.Timestamp{
		Seconds: t.Unix(),
		Nanos:   int32(t.Nanosecond()),
	}
}

// copyAttributes copies a map of attributes to a proto map field.
// It creates the map if it is nil.
func copyAttributes(out **tracepb.Span_Attributes, in []label.KeyValue) {
	if len(in) == 0 {
		return
	}
	if *out == nil {
		*out = &tracepb.Span_Attributes{}
	}
	if (*out).AttributeMap == nil {
		(*out).AttributeMap = make(map[string]*tracepb.AttributeValue)
	}
	var dropped int32
	for _, kv := range in {
		av := attributeValue(kv)
		if av == nil {
			continue
		}
		switch kv.Key {
		case PathAttribute:
			(*out).AttributeMap[labelHTTPPath] = av
		case HostAttribute:
			(*out).AttributeMap[labelHTTPHost] = av
		case MethodAttribute:
			(*out).AttributeMap[labelHTTPMethod] = av
		case UserAgentAttribute:
			(*out).AttributeMap[labelHTTPUserAgent] = av
		case StatusCodeAttribute:
			(*out).AttributeMap[labelHTTPStatusCode] = av
		default:
			if len(kv.Key) > 128 {
				dropped++
				continue
			}
			(*out).AttributeMap[string(kv.Key)] = av
		}
	}
	(*out).DroppedAttributesCount = dropped
}

func attributeValue(keyValue label.KeyValue) *tracepb.AttributeValue {
	v := keyValue.Value
	switch v.Type() {
	case label.BOOL:
		return &tracepb.AttributeValue{
			Value: &tracepb.AttributeValue_BoolValue{BoolValue: v.AsBool()},
		}
	case label.INT64:
		return &tracepb.AttributeValue{
			Value: &tracepb.AttributeValue_IntValue{IntValue: v.AsInt64()},
		}
	case label.FLOAT64:
		// TODO: set double value if Google Cloud Trace support it in the future.
		return &tracepb.AttributeValue{
			Value: &tracepb.AttributeValue_StringValue{
				StringValue: trunc(strconv.FormatFloat(v.AsFloat64(), 'f', -1, 64),
					maxAttributeStringValue)},
		}
	case label.STRING:
		return &tracepb.AttributeValue{
			Value: &tracepb.AttributeValue_StringValue{StringValue: trunc(v.AsString(), maxAttributeStringValue)},
		}
	}
	return nil
}

// trunc returns a TruncatableString truncated to the given limit.
func trunc(s string, limit int) *tracepb.TruncatableString {
	if len(s) > limit {
		b := []byte(s[:limit])
		for {
			r, size := utf8.DecodeLastRune(b)
			if r == utf8.RuneError && size == 1 {
				b = b[:len(b)-1]
			} else {
				break
			}
		}
		return &tracepb.TruncatableString{
			Value:              string(b),
			TruncatedByteCount: clip32(len(s) - len(b)),
		}
	}
	return &tracepb.TruncatableString{
		Value:              s,
		TruncatedByteCount: 0,
	}
}

// clip32 clips an int to the range of an int32.
func clip32(x int) int32 {
	if x < math.MinInt32 {
		return math.MinInt32
	}
	if x > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(x)
}
