// Deprecated: Use the standard W3C Trace Context propagator (go.opentelemetry.io/otel/propagation.TraceContext) instead.
module github.com/GoogleCloudPlatform/opentelemetry-operations-go/propagator

go 1.26.0

toolchain go1.27.1

require (
	github.com/google/go-cmp v0.7.0
	go.opentelemetry.io/otel v1.46.0
	go.opentelemetry.io/otel/trace v1.46.0
)

require github.com/cespare/xxhash/v2 v2.3.0 // indirect
