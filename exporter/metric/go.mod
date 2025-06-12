module github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric

go 1.23.0

require (
	cloud.google.com/go/monitoring v1.24.2
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.52.0
	github.com/googleapis/gax-go/v2 v2.14.2
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/otel v1.36.0
	go.opentelemetry.io/otel/metric v1.36.0
	go.opentelemetry.io/otel/sdk v1.36.0
	go.opentelemetry.io/otel/sdk/metric v1.36.0
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/oauth2 v0.30.0
	golang.org/x/sys v0.33.0 // indirect
	google.golang.org/api v0.234.0
	google.golang.org/genproto v0.0.0-20250505200425-f936aa4a68b2 // indirect
	google.golang.org/grpc v1.72.2
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock v0.52.0
	go.opentelemetry.io/otel/trace v1.36.0
	google.golang.org/genproto/googleapis/api v0.0.0-20250505200425-f936aa4a68b2
)

require (
	cloud.google.com/go/auth v0.16.1 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.7.0 // indirect
	cloud.google.com/go/logging v1.13.0 // indirect
	cloud.google.com/go/longrunning v0.6.7 // indirect
	cloud.google.com/go/trace v1.11.6 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	golang.org/x/crypto v0.38.0 // indirect
	golang.org/x/sync v0.14.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250519155744-55703ea1f237 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock => ../../internal/cloudmock

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping => ../../internal/resourcemapping

retract v1.0.0-RC1
