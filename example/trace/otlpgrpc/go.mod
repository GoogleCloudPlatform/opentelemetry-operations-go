module github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/trace/otlpgrpc

go 1.24.0

toolchain go1.24.2

// [START opentelemetry_otlp_grpc_deps]
require (
	// [START_EXCLUDE]
	go.opentelemetry.io/otel v1.40.0
	// [END_EXCLUDE]
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.38.0
	// [START_EXCLUDE silent]
	go.opentelemetry.io/otel/sdk v1.40.0
	go.opentelemetry.io/otel/trace v1.40.0

	// [START opentelemetyry_otlp_grpc_auth_deps]
	// When using gRPC based OTLP exporter, the auth is built in.
	google.golang.org/grpc v1.75.1
// [END opentelemetyry_otlp_grpc_auth_deps]
// [END_EXCLUDE]
)

// [END opentelemetry_otlp_grpc_deps]

require (
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.2 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.38.0 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.opentelemetry.io/proto/otlp v1.8.0 // indirect
	golang.org/x/net v0.44.0 // indirect
	golang.org/x/oauth2 v0.31.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250922171735-9219d122eba9 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250922171735-9219d122eba9 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
)
