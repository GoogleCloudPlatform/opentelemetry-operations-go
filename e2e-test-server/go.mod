module github.com/GoogleCloudPlatform/opentelemetry-operations-go/e2e-test-server

go 1.16

require (
	cloud.google.com/go/pubsub v1.12.0
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v0.0.0
	go.opentelemetry.io/otel v1.0.0-RC1
	go.opentelemetry.io/otel/sdk v1.0.0-RC1
	go.opentelemetry.io/otel/trace v1.0.0-RC1
	google.golang.org/genproto v0.0.0-20210614143202-012ab6975634
)

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../exporter/trace
