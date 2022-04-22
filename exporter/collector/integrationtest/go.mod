module github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/integrationtest

go 1.16

require (
	contrib.go.opencensus.io/exporter/stackdriver v0.13.11
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector v0.28.0
	github.com/aws/aws-sdk-go v1.42.49 // indirect
	github.com/google/go-cmp v0.5.7
	github.com/stretchr/testify v1.7.1
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector v0.49.0
	go.opentelemetry.io/collector/model v0.49.0
	go.opentelemetry.io/collector/pdata v0.49.0
	go.uber.org/zap v1.21.0
	google.golang.org/api v0.74.0
	google.golang.org/genproto v0.0.0-20220405205423-9d709892a2bf
	google.golang.org/grpc v1.45.0
	google.golang.org/protobuf v1.28.0
)

replace (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector => ../../collector
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../../trace
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping => ../../../internal/resourcemapping
)
