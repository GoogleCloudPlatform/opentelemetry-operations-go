module github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/trace/http

go 1.13

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../../../exporter/trace

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v0.2.0
	go.opentelemetry.io/otel v0.8.0
	google.golang.org/grpc v1.30.0
)
