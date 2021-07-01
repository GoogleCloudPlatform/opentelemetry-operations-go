package endtoendserver

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/genproto/googleapis/rpc/code"
)

var scenarioHanders = map[string]scenarioHandler{
	"/basicPropagator": (*Server).unimplementedHandler,
	"/basicTrace":      (*Server).basicTraceHandler,
	"/complexTrace":    (*Server).complexTraceHandler,
	"/health":          (*Server).healthHandler,
}

type scenarioHandler func(*Server, context.Context, request) *response

type request struct {
	scenario string
	testID   string
	headers  map[string]string
	data     []byte
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

	tracer := otel.GetTracerProvider().Tracer(instrumentingModuleName)
	func(ctx context.Context) {
		_, span := tracer.Start(ctx, "basicTrace",
			trace.WithAttributes(attribute.String(testIDKey, req.testID)))
		defer span.End()
	}(ctx)

	return &response{
		statusCode: code.Code_OK,
	}
}

// complexTraceHandler creates a complex trace and returns an OK response.
func (S *Server) complexTraceHandler(ctx context.Context, req request) *response {
	if req.testID == "" {
		log.Printf("request is missing required field 'testID'. request: %+v", req)
		return &response{
			statusCode: code.Code_INVALID_ARGUMENT,
			data:       []byte("request is missing required field 'testID'"),
		}
	}

	tracer := otel.GetTracerProvider().Tracer(instrumentingModuleName)
	func(ctx context.Context) {
		ctx, span := tracer.Start(ctx, "complexTrace/root",
			trace.WithAttributes(attribute.String(testIDKey, req.testID)))
		defer span.End()

		func(ctx context.Context) {
			ctx, span := tracer.Start(ctx, "complexTrace/child1",
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(attribute.String(testIDKey, req.testID)))
			defer span.End()

			func(ctx context.Context) {
				_, span := tracer.Start(ctx, "complexTrace/child2",
					trace.WithSpanKind(trace.SpanKindClient),
					trace.WithAttributes(attribute.String(testIDKey, req.testID)))
				defer span.End()
			}(ctx)
		}(ctx)

		func(ctx context.Context) {
			_, span := tracer.Start(ctx, "complexTrace/child3",
				trace.WithAttributes(attribute.String(testIDKey, req.testID)))
			defer span.End()

		}(ctx)

	}(ctx)
	return &response{statusCode: code.Code_OK}
}

// unimplementedHandler returns an UNIMPLEMENTED response without creating any traces.
func (s *Server) unimplementedHandler(ctx context.Context, req request) *response {
	log.Printf("received unhandled scenario %q", req.scenario)
	return &response{statusCode: code.Code_UNIMPLEMENTED}
}
