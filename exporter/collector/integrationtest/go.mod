module github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/integrationtest

go 1.16

require (
	contrib.go.opencensus.io/exporter/stackdriver v0.13.10
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector v0.0.0-20220208161221-df653dc6cb10
	github.com/aws/aws-sdk-go v1.42.49 // indirect
	github.com/google/go-cmp v0.5.7
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector v0.46.0
	go.opentelemetry.io/collector/model v0.46.0
	go.uber.org/zap v1.21.0
	google.golang.org/api v0.68.0
	google.golang.org/genproto v0.0.0-20220207185906-7721543eae58
	google.golang.org/grpc v1.44.0
	google.golang.org/protobuf v1.27.1
)

replace (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector => ../../collector
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../../trace
)
