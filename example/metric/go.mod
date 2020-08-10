module github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/metric

go 1.14

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric => ../../exporter/metric

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.2.1
	go.opentelemetry.io/otel v0.10.0
	go.opentelemetry.io/otel/sdk v0.10.0
	google.golang.org/api v0.25.0 // indirect
)
