module github.com/GoogleCloudPlatform/opentelemetry-operations-go/e2e-test-server

go 1.16

require (
	cloud.google.com/go/iam v0.1.1 // indirect
	cloud.google.com/go/pubsub v1.17.1
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.0.0
	go.opentelemetry.io/otel v1.3.0
	go.opentelemetry.io/otel/sdk v1.3.0
	go.opentelemetry.io/otel/trace v1.3.0
	google.golang.org/genproto v0.0.0-20220207185906-7721543eae58
)

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../exporter/trace

retract v1.0.0-RC1
