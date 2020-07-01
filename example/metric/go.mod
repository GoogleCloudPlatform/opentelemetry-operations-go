module github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/metric

go 1.14

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric => ../../exporter/metric

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.2.1
	go.opentelemetry.io/otel v0.7.0
	golang.org/x/net v0.0.0-20200520182314-0ba52f642ac2 // indirect
	golang.org/x/sys v0.0.0-20200519105757-fe76b779f299 // indirect
	google.golang.org/api v0.25.0 // indirect
	google.golang.org/genproto v0.0.0-20200521103424-e9a78aa275b7 // indirect
)
