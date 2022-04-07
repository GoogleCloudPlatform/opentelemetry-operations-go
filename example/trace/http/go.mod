module github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/trace/http

go 1.17

replace github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace => ../../../exporter/trace

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.3.0
	github.com/davecgh/go-spew v1.1.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.31.0
	go.opentelemetry.io/otel v1.6.1
	go.opentelemetry.io/otel/sdk v1.6.1
	go.opentelemetry.io/otel/trace v1.6.1
)

require (
	cloud.google.com/go/compute v1.2.0 // indirect
	cloud.google.com/go/trace v1.0.0 // indirect
	github.com/felixge/httpsnoop v1.0.2 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.7 // indirect
	github.com/googleapis/gax-go/v2 v2.1.1 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.opentelemetry.io/otel/metric v0.28.0 // indirect
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

retract (
	v1.0.0
	v1.0.0-RC2
	v1.0.0-RC1
)
