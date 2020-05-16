module github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/metric

go 1.14

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric => ../../exporter/metric

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.0.0-00010101000000-000000000000
	go.opentelemetry.io/otel v0.5.0
)
