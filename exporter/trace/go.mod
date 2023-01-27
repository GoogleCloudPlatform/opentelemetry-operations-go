module github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace

go 1.18

require (
	cloud.google.com/go/trace v1.8.0
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2
	github.com/stretchr/testify v1.8.1
	go.opentelemetry.io/otel v1.11.2
	go.opentelemetry.io/otel/sdk v1.11.2
	go.opentelemetry.io/otel/trace v1.11.2
	golang.org/x/net v0.0.0-20221017152216-f25eb7ecb193 // indirect
	golang.org/x/oauth2 v0.0.0-20221014153046-6fdb5e3db783
	golang.org/x/sys v0.1.0 // indirect
	google.golang.org/api v0.108.0
	google.golang.org/genproto v0.0.0-20230125152338-dcaf20b6aeaa
	google.golang.org/grpc v1.51.0
	google.golang.org/protobuf v1.28.1
)

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock v0.34.2
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.34.2
	github.com/googleapis/gax-go/v2 v2.7.0
	go.uber.org/multierr v1.8.0
)

require (
	cloud.google.com/go/compute v1.14.0 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	cloud.google.com/go/logging v1.6.1 // indirect
	cloud.google.com/go/longrunning v0.3.0 // indirect
	cloud.google.com/go/monitoring v1.8.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.2.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/text v0.5.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping => ../../internal/resourcemapping

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock => ../../internal/cloudmock
