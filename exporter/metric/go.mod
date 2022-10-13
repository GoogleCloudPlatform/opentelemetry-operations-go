module github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric

go 1.18

require (
	cloud.google.com/go/monitoring v1.4.0
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	go.opentelemetry.io/otel v1.11.0
	go.opentelemetry.io/otel/metric v0.32.3 // indirect
	go.opentelemetry.io/otel/sdk v1.11.0
	go.opentelemetry.io/otel/sdk/metric v0.32.3
	golang.org/x/net v0.0.0-20220325170049-de3da57026de // indirect
	golang.org/x/oauth2 v0.0.0-20220309155454-6242fa91716a
	golang.org/x/sys v0.0.0-20220919091848-fb04ddd9f9c8 // indirect
	google.golang.org/api v0.74.0
	google.golang.org/genproto v0.0.0-20220829175752-36a9c930ecbf
	google.golang.org/grpc v1.49.0
	google.golang.org/protobuf v1.28.1
)

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock v0.34.1
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.34.1
	github.com/stretchr/testify v1.7.1
	go.uber.org/multierr v1.8.0
)

require (
	cloud.google.com/go/compute v1.5.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.8 // indirect
	github.com/googleapis/gax-go/v2 v2.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.opentelemetry.io/otel/trace v1.11.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock => ../../internal/cloudmock

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping => ../../internal/resourcemapping

retract v1.0.0-RC1
