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

package trace

import (
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// ReadOnlySpan is a copy of sdktrace.ReadOnlySpan without the private func.
// It exists so that callers outside the Go SDK (such as the OTel collector exporter)
// can create their own ReadOnlySpan instances, while retaining compatibility with
// the Go exporter API.
type ReadOnlySpan interface {
	Name() string
	SpanContext() trace.SpanContext
	Parent() trace.SpanContext
	SpanKind() trace.SpanKind
	StartTime() time.Time
	EndTime() time.Time
	Attributes() []attribute.KeyValue
	Links() []trace.Link
	Events() []sdktrace.Event
	Status() sdktrace.Status
	InstrumentationLibrary() instrumentation.Library
	Resource() *resource.Resource
	DroppedAttributes() int
	DroppedLinks() int
	DroppedEvents() int
	ChildSpanCount() int
}

// Ensure that our notion of ReadOnlySpan remains compatible with the SDK's.
var _ ReadOnlySpan = sdktrace.ReadOnlySpan(nil)
