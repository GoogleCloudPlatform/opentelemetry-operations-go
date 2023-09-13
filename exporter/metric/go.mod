module github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric

go 1.20

require (
	cloud.google.com/go/monitoring v1.15.1
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock v0.43.1
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.43.1
	github.com/googleapis/gax-go/v2 v2.11.0
	github.com/stretchr/testify v1.8.4
	go.opentelemetry.io/otel v1.18.0
	go.opentelemetry.io/otel/metric v1.18.0
	go.opentelemetry.io/otel/sdk v1.18.0
	go.opentelemetry.io/otel/sdk/metric v0.41.0
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/oauth2 v0.8.0
	golang.org/x/sys v0.12.0 // indirect
	google.golang.org/api v0.126.0
	google.golang.org/genproto v0.0.0-20230731193218-e0aa005b6bdf // indirect
	google.golang.org/grpc v1.57.0
	google.golang.org/protobuf v1.31.0
)

require google.golang.org/genproto/googleapis/api v0.0.0-20230726155614-23370e0ffb3e

require (
	cloud.google.com/go/compute v1.23.0 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	cloud.google.com/go/logging v1.7.0 // indirect
	cloud.google.com/go/longrunning v0.5.1 // indirect
	cloud.google.com/go/trace v1.10.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/s2a-go v0.1.4 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.2.3 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/otel/trace v1.18.0 // indirect
	golang.org/x/crypto v0.9.0 // indirect
	golang.org/x/sync v0.2.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230731190214-cbb8c96f2d6d // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock => ../../internal/cloudmock

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping => ../../internal/resourcemapping

retract v1.0.0-RC1
