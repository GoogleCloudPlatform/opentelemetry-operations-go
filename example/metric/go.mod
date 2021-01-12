module github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/metric

go 1.14

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric => ../../exporter/metric

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.13.0
	go.opentelemetry.io/otel v0.15.0
	go.opentelemetry.io/otel/sdk v0.15.0
)
