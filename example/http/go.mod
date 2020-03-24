module github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/http

go 1.13

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../../exporter/trace

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v0.0.0-00010101000000-000000000000
	go.opentelemetry.io/otel v0.2.3
	google.golang.org/grpc v1.27.1
)
