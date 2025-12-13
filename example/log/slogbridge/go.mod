module github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/log/slogbridge

go 1.24.0

toolchain go1.24.2

require (
	github.com/go-viper/mapstructure/v2 v2.4.0
	go.opentelemetry.io/contrib/bridges/otelslog v0.13.0
	go.opentelemetry.io/contrib/detectors/gcp v1.38.0
	go.opentelemetry.io/otel v1.38.0
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.14.0
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.14.0
	go.opentelemetry.io/otel/log v0.14.0
	go.opentelemetry.io/otel/sdk v1.38.0
	go.opentelemetry.io/otel/sdk/log v0.14.0
)

require (
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.30.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.2 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	go.opentelemetry.io/proto/otlp v1.8.0 // indirect
	golang.org/x/net v0.46.1-0.20251013234738-63d1a5100f82 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.30.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251213004720-97cd9d5aeac2 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251213004720-97cd9d5aeac2 // indirect
	google.golang.org/grpc v1.77.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp => ../../../detectors/gcp
