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

package endtoendserver

import (
	"context"
	"log"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/genproto/googleapis/rpc/code"
)

var scenarioHandlers = map[string]scenarioHandler{
	"/basicPropagator": (*Server).unimplementedHandler,
	"/basicTrace":      (*Server).basicTraceHandler,
	"/complexTrace":    (*Server).complexTraceHandler,
	"/health":          (*Server).healthHandler,
}

type scenarioHandler func(*Server, context.Context, request) *response

type request struct {
	scenario string
	testID   string
}

type response struct {
	statusCode code.Code
	data       []byte
}

// healthHandler returns an OK response without creating any traces.
func (s *Server) healthHandler(ctx context.Context, req request) *response {
	return &response{statusCode: code.Code_OK}
}

// basicTraceHandler creates a basic trace and returns an OK response.
func (s *Server) basicTraceHandler(ctx context.Context, req request) *response {
	if req.testID == "" {
		log.Printf("request is missing required field 'testID'. request: %+v", req)
		return &response{
			statusCode: code.Code_INVALID_ARGUMENT,
			data:       []byte("request is missing required field 'testID'"),
		}
	}

	tracer := s.traceProvider.Tracer(instrumentingModuleName)
	_, span := tracer.Start(ctx, "basicTrace",
		trace.WithAttributes(attribute.String(testIDKey, req.testID)))
	span.End()

	return &response{statusCode: code.Code_OK}
}

// complexTraceHandler creates a complex trace and returns an OK response.
func (s *Server) complexTraceHandler(ctx context.Context, req request) *response {
	if req.testID == "" {
		log.Printf("request is missing required field 'testID'. request: %+v", req)
		return &response{
			statusCode: code.Code_INVALID_ARGUMENT,
			data:       []byte("request is missing required field 'testID'"),
		}
	}

	tracer := s.traceProvider.Tracer(instrumentingModuleName)
	func(ctx context.Context) {
		ctx, span := tracer.Start(ctx, "complexTrace/root",
			trace.WithAttributes(attribute.String(testIDKey, req.testID)))
		defer span.End()

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
	}(ctx)

	return &response{statusCode: code.Code_OK}
}

// unimplementedHandler returns an UNIMPLEMENTED response without creating any traces.
func (s *Server) unimplementedHandler(ctx context.Context, req request) *response {
	log.Printf("received unhandled scenario %q", req.scenario)
	return &response{statusCode: code.Code_UNIMPLEMENTED}
}
