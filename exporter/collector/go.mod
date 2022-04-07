module github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector

go 1.16

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.3.0
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.48.0
	github.com/stretchr/testify v1.7.1
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector/model v0.48.0
	go.opentelemetry.io/otel v1.6.2
	go.opentelemetry.io/otel/sdk v1.6.2
	go.opentelemetry.io/otel/trace v1.6.2
	google.golang.org/api v0.74.0
	google.golang.org/genproto v0.0.0-20220407144326-9054f6ed7bac
	google.golang.org/grpc v1.45.0
	google.golang.org/protobuf v1.28.0
)

require (
	cloud.google.com/go/logging v1.4.2
	cloud.google.com/go/monitoring v1.4.0
	cloud.google.com/go/trace v1.2.0
	github.com/google/go-cmp v0.5.7
	go.uber.org/multierr v1.8.0
	go.uber.org/zap v1.21.0
	golang.org/x/net v0.0.0-20220403103023-749bd193bc2b // indirect
	golang.org/x/oauth2 v0.0.0-20220309155454-6242fa91716a
	golang.org/x/sys v0.0.0-20220406163625-3f8b81556e12 // indirect
)

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../trace
