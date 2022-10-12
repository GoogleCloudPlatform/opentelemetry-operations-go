module github.com/GoogleCloudPlatform/opentelemetry-operations-go

go 1.18

require (
	github.com/google/go-cmp v0.5.8
	go.opentelemetry.io/otel v1.11.0
	go.opentelemetry.io/otel/trace v1.11.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

retract (
	v1.8.0
	v1.5.2
	v1.5.1
	v1.5.0
	v1.4.0
	v1.3.0
	v1.0.0
	v1.0.0-RC2
	v1.0.0-RC1
)
