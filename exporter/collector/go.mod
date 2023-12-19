module github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector

go 1.20

require (
	cloud.google.com/go/logging v1.7.0
	cloud.google.com/go/monitoring v1.15.1
	cloud.google.com/go/trace v1.10.1
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.21.0
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.45.0
	github.com/census-instrumentation/opencensus-proto v0.4.1
	github.com/fsnotify/fsnotify v1.6.0
	github.com/google/go-cmp v0.5.9
	github.com/googleapis/gax-go/v2 v2.11.0
	github.com/stretchr/testify v1.8.4
	github.com/tidwall/wal v1.1.7
	go.opencensus.io v0.24.0
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0015
	go.opentelemetry.io/collector/semconv v0.86.0
	go.opentelemetry.io/otel v1.19.0
	go.opentelemetry.io/otel/sdk v1.19.0
	go.opentelemetry.io/otel/trace v1.19.0
	go.uber.org/atomic v1.10.0
	go.uber.org/zap v1.23.0
	golang.org/x/oauth2 v0.10.0
	google.golang.org/api v0.126.0
	google.golang.org/genproto v0.0.0-20230731193218-e0aa005b6bdf
	google.golang.org/genproto/googleapis/api v0.0.0-20230726155614-23370e0ffb3e
	google.golang.org/grpc v1.58.3
	google.golang.org/protobuf v1.31.0
)

require (
	cloud.google.com/go v0.110.6 // indirect
	cloud.google.com/go/compute v1.23.0 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	cloud.google.com/go/longrunning v0.5.1 // indirect
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/s2a-go v0.1.4 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.2.3 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/tidwall/gjson v1.10.2 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tidwall/tinylru v1.1.0 // indirect
	go.opentelemetry.io/otel/metric v1.19.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.17.0 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sync v0.3.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230731190214-cbb8c96f2d6d // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../trace

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping => ../../internal/resourcemapping

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock => ../../internal/cloudmock

retract v0.39.1
