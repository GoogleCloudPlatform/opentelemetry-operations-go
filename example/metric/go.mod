module github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/metric

go 1.17

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric => ../../exporter/metric

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.24.0
	go.opentelemetry.io/otel v1.6.1
	go.opentelemetry.io/otel/metric v0.28.0
	go.opentelemetry.io/otel/sdk v1.6.1
	go.opentelemetry.io/otel/sdk/metric v0.28.0
)

require (
	cloud.google.com/go/compute v1.2.0 // indirect
	cloud.google.com/go/monitoring v1.2.0 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.7 // indirect
	github.com/googleapis/gax-go/v2 v2.1.1 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.opentelemetry.io/otel/trace v1.6.1 // indirect
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd // indirect
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8 // indirect
	golang.org/x/sys v0.0.0-20220207234003-57398862261d // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/api v0.68.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20220405205423-9d709892a2bf // indirect
	google.golang.org/grpc v1.45.0 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
)
