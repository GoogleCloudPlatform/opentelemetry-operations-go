module github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/trace/otlpgrpc

go 1.25.0

toolchain go1.25.7

// [START opentelemetry_otlp_grpc_deps]
require (
	// [START_EXCLUDE]
	go.opentelemetry.io/otel v1.43.0
	// [END_EXCLUDE]
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.38.0
	// [START_EXCLUDE silent]
	go.opentelemetry.io/otel/sdk v1.43.0
	go.opentelemetry.io/otel/trace v1.43.0

	// [START opentelemetyry_otlp_grpc_auth_deps]
	// When using gRPC based OTLP exporter, the auth is built in.
	google.golang.org/grpc v1.79.3
// [END opentelemetyry_otlp_grpc_auth_deps]
// [END_EXCLUDE]
)

// [END opentelemetry_otlp_grpc_deps]

require (
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.2 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.38.0 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/proto/otlp v1.8.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/oauth2 v0.34.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)
