module github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/trace/http

go 1.13

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../../../exporter/trace

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v0.11.0
	go.opentelemetry.io/contrib/instrumentation/net/http v0.11.0
	go.opentelemetry.io/otel v0.11.0
	go.opentelemetry.io/otel/sdk v0.11.0
	golang.org/x/sys v0.0.0-20200615200032-f1bc736245b1 // indirect
)
