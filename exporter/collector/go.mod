module github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector

go 1.16

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.0.0
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.40.0
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector/model v0.46.0
	go.opentelemetry.io/otel v1.4.1
	go.opentelemetry.io/otel/sdk v1.4.1
	go.opentelemetry.io/otel/trace v1.4.1
	google.golang.org/api v0.68.0
	google.golang.org/genproto v0.0.0-20220207185906-7721543eae58
	google.golang.org/grpc v1.44.0
	google.golang.org/protobuf v1.27.1
)

require (
	cloud.google.com/go/logging v1.4.2
	cloud.google.com/go/monitoring v1.2.0
	cloud.google.com/go/trace v1.0.0
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/google/go-cmp v0.5.7
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.29.0 // indirect
	go.uber.org/multierr v1.8.0
	go.uber.org/zap v1.21.0
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
)

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../trace
