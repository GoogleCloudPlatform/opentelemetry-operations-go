module github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/trace/http

go 1.14

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../../../exporter/trace

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.0.0-RC2
	github.com/davecgh/go-spew v1.1.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.21.0
	go.opentelemetry.io/otel v1.0.0-RC2
	go.opentelemetry.io/otel/sdk v1.0.0-RC2
	go.opentelemetry.io/otel/trace v1.0.0-RC2
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
)
