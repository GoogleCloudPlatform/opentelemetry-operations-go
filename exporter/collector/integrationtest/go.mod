module github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/integrationtest

go 1.22.0

require (
	cloud.google.com/go/logging v1.13.0
	cloud.google.com/go/monitoring v1.22.1
	cloud.google.com/go/trace v1.11.3
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector v0.49.0
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus v0.49.0
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.49.0
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock v0.49.0
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.49.0
	github.com/google/go-cmp v0.6.0
	github.com/google/uuid v1.6.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.118.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v0.118.0
	go.opentelemetry.io/collector/component/componenttest v0.118.0
	go.opentelemetry.io/collector/exporter v0.118.0
	go.opentelemetry.io/collector/featuregate v1.24.0
	go.opentelemetry.io/collector/otelcol/otelcoltest v0.118.0
	go.opentelemetry.io/collector/pdata v1.24.0
	go.opentelemetry.io/otel v1.32.0
	go.opentelemetry.io/otel/metric v1.32.0
	go.opentelemetry.io/otel/sdk v1.32.0
	go.opentelemetry.io/otel/sdk/metric v1.32.0
	go.uber.org/zap v1.27.0
	google.golang.org/api v0.216.0
	google.golang.org/genproto/googleapis/api v0.0.0-20250115164207-1a7da9e5054f
	google.golang.org/grpc v1.69.4
	google.golang.org/protobuf v1.36.3
)

require (
	cloud.google.com/go v0.118.0 // indirect
	cloud.google.com/go/auth v0.14.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.7 // indirect
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	cloud.google.com/go/longrunning v0.6.4 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.25.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/ebitengine/purego v0.8.2 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.4 // indirect
	github.com/googleapis/gax-go/v2 v2.14.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/lufia/plan9stats v0.0.0-20240909124753-873cd0166683 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_golang v1.20.5 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.61.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/shirou/gopsutil/v4 v4.24.12 // indirect
	github.com/spf13/cobra v1.8.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/tinylru v1.2.1 // indirect
	github.com/tidwall/wal v1.1.8 // indirect
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	github.com/tklauser/numcpus v0.9.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.118.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.118.0 // indirect
	go.opentelemetry.io/collector/confmap v1.24.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.24.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.24.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/httpprovider v1.24.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.24.0 // indirect
	go.opentelemetry.io/collector/connector v0.118.0 // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.118.0 // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.118.0 // indirect
	go.opentelemetry.io/collector/consumer v1.24.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.118.0 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.118.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.118.0 // indirect
	go.opentelemetry.io/collector/exporter/exportertest v0.118.0 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.118.0 // indirect
	go.opentelemetry.io/collector/extension v0.118.0 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.118.0 // indirect
	go.opentelemetry.io/collector/extension/extensiontest v0.118.0 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.118.0 // indirect
	go.opentelemetry.io/collector/otelcol v0.118.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.118.0 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.118.0 // indirect
	go.opentelemetry.io/collector/pipeline v0.118.0 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.118.0 // indirect
	go.opentelemetry.io/collector/processor v0.118.0 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.118.0 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.118.0 // indirect
	go.opentelemetry.io/collector/receiver v0.118.0 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.118.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.118.0 // indirect
	go.opentelemetry.io/collector/semconv v0.118.0 // indirect
	go.opentelemetry.io/collector/service v0.118.0 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.6.0 // indirect
	go.opentelemetry.io/contrib/config v0.10.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.54.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.56.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.31.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.7.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.32.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.32.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.31.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.31.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.31.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.54.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.7.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.32.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.31.0 // indirect
	go.opentelemetry.io/otel/log v0.8.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.7.0 // indirect
	go.opentelemetry.io/otel/trace v1.32.0 // indirect
	go.opentelemetry.io/proto/otlp v1.5.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.32.0 // indirect
	golang.org/x/exp v0.0.0-20250106191152-7588d65b2ba8 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/oauth2 v0.25.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	gonum.org/v1/gonum v0.15.1 // indirect
	google.golang.org/genproto v0.0.0-20250106144421-5f5ef82da422 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector => ../../collector
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus => ../../collector/googlemanagedprometheus
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric => ../../metric
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../../trace
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock => ../../../internal/cloudmock
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping => ../../../internal/resourcemapping
)
