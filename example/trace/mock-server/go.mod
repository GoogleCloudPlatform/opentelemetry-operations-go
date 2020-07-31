module github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/trace/mock-server

go 1.14

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../../../exporter/trace

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v0.2.1
	github.com/googleinterns/cloud-operations-api-mock v0.0.0-20200731141541-dbd9913763f3
	go.opentelemetry.io/otel v0.8.0
	google.golang.org/api v0.29.0
)
