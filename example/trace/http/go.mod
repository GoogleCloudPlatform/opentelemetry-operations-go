module github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/trace/http

go 1.22

toolchain go1.22.0

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.25.0
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/propagator v0.49.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.55.0
	go.opentelemetry.io/otel v1.30.0
	go.opentelemetry.io/otel/sdk v1.30.0
	go.opentelemetry.io/otel/trace v1.30.0
)

require (
	cloud.google.com/go/auth v0.12.1 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.7 // indirect
	cloud.google.com/go/compute/metadata v0.5.2 // indirect
	cloud.google.com/go/trace v1.11.2 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.49.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/s2a-go v0.1.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.4 // indirect
	github.com/googleapis/gax-go/v2 v2.13.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.54.0 // indirect
	go.opentelemetry.io/otel/metric v1.30.0 // indirect
	golang.org/x/crypto v0.32.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/oauth2 v0.25.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	google.golang.org/api v0.203.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20241007155032-5fefd90f89a9 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241206012308-a4fef0638583 // indirect
	google.golang.org/grpc v1.67.3 // indirect
	google.golang.org/protobuf v1.35.2 // indirect
)

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go => ../../..

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../../../exporter/trace

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping => ../../../internal/resourcemapping

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/propagator => ../../../propagator

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock => ../../../internal/cloudmock

retract (
	v1.0.0
	v1.0.0-RC2
	v1.0.0-RC1
)
