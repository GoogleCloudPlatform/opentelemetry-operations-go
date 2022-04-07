module github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector

go 1.17

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.3.0
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.48.0
	github.com/stretchr/testify v1.7.1
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector/model v0.48.0
	go.opentelemetry.io/otel v1.6.1
	go.opentelemetry.io/otel/sdk v1.6.1
	go.opentelemetry.io/otel/trace v1.6.1
	google.golang.org/api v0.68.0
	google.golang.org/genproto v0.0.0-20220405205423-9d709892a2bf
	google.golang.org/grpc v1.45.0
	google.golang.org/protobuf v1.28.0
)

require (
	cloud.google.com/go/monitoring v1.2.0
	cloud.google.com/go/trace v1.0.0
	github.com/google/go-cmp v0.5.7
	go.uber.org/zap v1.21.0
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
)

require (
	cloud.google.com/go/compute v1.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/googleapis/gax-go/v2 v2.1.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.48.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/otel/metric v0.28.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd // indirect
	golang.org/x/sys v0.0.0-20220207234003-57398862261d // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../trace
