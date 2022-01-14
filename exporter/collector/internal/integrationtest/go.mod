module github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/integrationtest

go 1.16

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector v0.0.0-20220111155622-771af0772772
	github.com/google/go-cmp v0.5.6
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector v0.40.0
	go.opentelemetry.io/collector/model v0.40.0
	go.uber.org/zap v1.19.1
	google.golang.org/genproto v0.0.0-20211021150943-2b146023228c
	google.golang.org/grpc v1.42.0
	google.golang.org/protobuf v1.27.1
)

replace (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector => ../../../collector
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../../../trace
)
