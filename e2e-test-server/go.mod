module github.com/GoogleCloudPlatform/opentelemetry-operations-go/e2e-test-server

go 1.14

require (
	cloud.google.com/go/pubsub v1.13.0
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.0.0
	go.opentelemetry.io/otel v1.0.1
	go.opentelemetry.io/otel/sdk v1.0.1
	go.opentelemetry.io/otel/trace v1.0.1
	google.golang.org/genproto v0.0.0-20210722135532-667f2b7c528f
)

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../exporter/trace
