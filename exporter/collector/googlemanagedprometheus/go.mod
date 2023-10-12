module github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus

go 1.20

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.86.0
	github.com/stretchr/testify v1.8.4
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0015
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0015
	go.opentelemetry.io/collector/semconv v0.86.0
	google.golang.org/genproto/googleapis/api v0.0.0-20230726155614-23370e0ffb3e
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	google.golang.org/genproto v0.0.0-20230731193218-e0aa005b6bdf // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230731190214-cbb8c96f2d6d // indirect
	google.golang.org/grpc v1.58.1 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector => ../../collector
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../../trace
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping => ../../../internal/resourcemapping
)
