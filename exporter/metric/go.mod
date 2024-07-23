module github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric

go 1.21

toolchain go1.22.0

require (
	cloud.google.com/go/monitoring v1.20.2
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock v0.48.1
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.48.1
	github.com/googleapis/gax-go/v2 v2.12.5
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/otel v1.28.0
	go.opentelemetry.io/otel/metric v1.28.0
	go.opentelemetry.io/otel/sdk v1.28.0
	go.opentelemetry.io/otel/sdk/metric v1.28.0
	golang.org/x/net v0.27.0 // indirect
	golang.org/x/oauth2 v0.21.0
	golang.org/x/sys v0.22.0 // indirect
	google.golang.org/api v0.188.0
	google.golang.org/genproto v0.0.0-20240708141625-4ad9e859172b // indirect
	google.golang.org/grpc v1.64.1
	google.golang.org/protobuf v1.34.2
)

require (
	go.opentelemetry.io/otel/trace v1.28.0
	google.golang.org/genproto/googleapis/api v0.0.0-20240701130421-f6361c86f094
)

require (
	cloud.google.com/go/auth v0.7.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.2 // indirect
	cloud.google.com/go/compute/metadata v0.5.0 // indirect
	cloud.google.com/go/logging v1.10.0 // indirect
	cloud.google.com/go/longrunning v0.5.9 // indirect
	cloud.google.com/go/trace v1.10.9 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.49.0 // indirect
	golang.org/x/crypto v0.25.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240708141625-4ad9e859172b // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock => ../../internal/cloudmock

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping => ../../internal/resourcemapping

retract v1.0.0-RC1
