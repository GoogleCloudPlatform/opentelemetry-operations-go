module github.com/googlecloudplatform/opentelemetry-operations-go/exporter/trace

go 1.13

require (
	cloud.google.com/go v0.53.0
	github.com/golang/protobuf v1.3.4
	github.com/stretchr/testify v1.4.0
	go.opentelemetry.io/otel v0.2.3
	go.opentelemetry.io/otel/exporters/trace/stackdriver v0.2.3
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	google.golang.org/api v0.20.0
	google.golang.org/genproto v0.0.0-20200303153909-beee998c1893
	google.golang.org/grpc v1.27.1
)
