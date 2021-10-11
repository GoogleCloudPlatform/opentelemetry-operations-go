module github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector

go 1.14

require (
	contrib.go.opencensus.io/exporter/stackdriver v0.13.8
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.0.0
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.36.0
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector v0.36.0
	go.opentelemetry.io/collector/model v0.36.0
	go.opentelemetry.io/otel v1.0.1
	go.opentelemetry.io/otel/sdk v1.0.1
	go.opentelemetry.io/otel/trace v1.0.1
	google.golang.org/api v0.57.0
	google.golang.org/genproto v0.0.0-20210903162649-d08c68adba83
	google.golang.org/grpc v1.40.0
	google.golang.org/protobuf v1.27.1
)

require (
	cloud.google.com/go v0.94.1
	cloud.google.com/go/monitoring v0.1.0
	cloud.google.com/go/trace v0.1.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.6
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f
)

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../trace
