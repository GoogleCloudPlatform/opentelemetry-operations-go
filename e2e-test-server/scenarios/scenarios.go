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

package scenarios

import (
	"context"
	"errors"
	"log"

	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/genproto/googleapis/rpc/code"
)

var scenarioHandlers = map[string]scenarioHandler{
	"/basicPropagator": &unimplementedHandler{},
	"/basicTrace":      &basicTraceHandler{},
	"/complexTrace":    &complexTraceHandler{},
	"/detectResource":  &detectResourceHandler{},
	"/health":          &healthHandler{},
}

type request struct {
	scenario string
	testID   string
}

type response struct {
	data       []byte
	statusCode code.Code
	traceID    trace.TraceID
}

type scenarioHandler interface {
	handle(context.Context, request, *sdktrace.TracerProvider) *response
	tracerProvider() (*sdktrace.TracerProvider, error)
}

// healthHandler returns an OK response without creating any traces.
type healthHandler struct{}

func (*healthHandler) handle(ctx context.Context, req request, tracerProvider *sdktrace.TracerProvider) *response {
	return &response{statusCode: code.Code_OK}
}

func (*healthHandler) tracerProvider() (*sdktrace.TracerProvider, error) {
	return newTracerProvider(resource.Empty())
}

// basicTraceHandler creates a basic trace and returns an OK response.
type basicTraceHandler struct{}

func (*basicTraceHandler) handle(ctx context.Context, req request, tracerProvider *sdktrace.TracerProvider) *response {
	if req.testID == "" {
		log.Printf("request is missing required field 'testID'. request: %+v", req)
		return &response{
			statusCode: code.Code_INVALID_ARGUMENT,
			data:       []byte("request is missing required field 'testID'"),
		}
	}

	tracer := tracerProvider.Tracer(instrumentingModuleName)
	_, span := tracer.Start(ctx, "basicTrace",
		trace.WithAttributes(attribute.String(testIDKey, req.testID)))
	span.End()

	return &response{statusCode: code.Code_OK, traceID: span.SpanContext().TraceID()}
}

func (*basicTraceHandler) tracerProvider() (*sdktrace.TracerProvider, error) {
	return newTracerProvider(resource.Empty())
}

// complexTraceHandler creates a complex trace and returns an OK response.
type complexTraceHandler struct{}

func (*complexTraceHandler) handle(ctx context.Context, req request, tracerProvider *sdktrace.TracerProvider) *response {
	if req.testID == "" {
		log.Printf("request is missing required field 'testID'. request: %+v", req)
		return &response{
			statusCode: code.Code_INVALID_ARGUMENT,
			data:       []byte("request is missing required field 'testID'"),
		}
	}

	tracer := tracerProvider.Tracer(instrumentingModuleName)
	ctx, rootSpan := tracer.Start(ctx, "complexTrace/root",
		trace.WithAttributes(attribute.String(testIDKey, req.testID)))
	defer rootSpan.End()

	func(ctx context.Context) {
		ctx, span := tracer.Start(ctx, "complexTrace/child1",
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(attribute.String(testIDKey, req.testID)))
		span.End()

		func(ctx context.Context) {
			_, span := tracer.Start(ctx, "complexTrace/child2",
				trace.WithSpanKind(trace.SpanKindClient),
				trace.WithAttributes(attribute.String(testIDKey, req.testID)))
			span.End()
		}(ctx)
	}(ctx)

	func(ctx context.Context) {
		_, span := tracer.Start(ctx, "complexTrace/child3",
			trace.WithAttributes(attribute.String(testIDKey, req.testID)))
		span.End()
	}(ctx)

	return &response{statusCode: code.Code_OK, traceID: rootSpan.SpanContext().TraceID()}
}

func (*complexTraceHandler) tracerProvider() (*sdktrace.TracerProvider, error) {
	return newTracerProvider(resource.Empty())
}

// detectResourceHandler creates a basic trace with resource info and returns an OK response.
type detectResourceHandler struct{}

func (*detectResourceHandler) handle(ctx context.Context, req request, tracerProvider *sdktrace.TracerProvider) *response {
	if req.testID == "" {
		log.Printf("request is missing required field 'testID'. request: %+v", req)
		return &response{
			statusCode: code.Code_INVALID_ARGUMENT,
			data:       []byte("request is missing required field 'testID'"),
		}
	}

	tracer := tracerProvider.Tracer(instrumentingModuleName)
	_, span := tracer.Start(ctx, "resourceDetectionTrace",
		trace.WithAttributes(attribute.String(testIDKey, req.testID)))
	span.End()

	return &response{statusCode: code.Code_OK, traceID: span.SpanContext().TraceID()}
}

func (*detectResourceHandler) tracerProvider() (*sdktrace.TracerProvider, error) {
	res, err := resource.New(context.Background(),
		resource.WithDetectors(gcp.NewDetector()),
		resource.WithFromEnv(),
	)
	if errors.Is(err, resource.ErrPartialResource) || errors.Is(err, resource.ErrSchemaURLConflict) {
		log.Println(err)
	} else if err != nil {
		return nil, err
	}
	log.Printf("Detected Resource: %+v\n", res.String())
	return newTracerProvider(res)
}

// unimplementedHandler returns an UNIMPLEMENTED response without creating any traces.
type unimplementedHandler struct{}

func (*unimplementedHandler) handle(ctx context.Context, req request, tracerProvider *sdktrace.TracerProvider) *response {
	log.Printf("received unhandled scenario %q", req.scenario)
	return &response{statusCode: code.Code_UNIMPLEMENTED}
}

func (*unimplementedHandler) tracerProvider() (*sdktrace.TracerProvider, error) {
	return newTracerProvider(resource.Empty())
}
