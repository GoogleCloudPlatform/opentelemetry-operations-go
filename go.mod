module github.com/GoogleCloudPlatform/opentelemetry-operations-go

go 1.17

require (
	github.com/google/go-cmp v0.5.7
	go.opentelemetry.io/otel v1.6.2
	go.opentelemetry.io/otel/trace v1.6.2
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

retract (
	v1.5.1
	v1.5.0
	v1.4.0
	v1.3.0
	v1.0.0
	v1.0.0-RC2
	v1.0.0-RC1
)
