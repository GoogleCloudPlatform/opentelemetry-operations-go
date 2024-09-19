module github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus

go 1.22.0

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.109.0
	github.com/prometheus/common v0.53.0
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/collector/featuregate v1.15.0
	go.opentelemetry.io/collector/pdata v1.15.0
	go.opentelemetry.io/collector/semconv v0.109.0
	google.golang.org/genproto/googleapis/api v0.0.0-20240604185151-ef581f913117
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.6.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240604185151-ef581f913117 // indirect
	google.golang.org/grpc v1.66.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector => ../../collector
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../../trace
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping => ../../../internal/resourcemapping
)
