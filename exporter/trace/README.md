# OpenTelemetry Google Cloud Trace Exporter

[![Docs](https://godoc.org/github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace?status.svg)](https://pkg.go.dev/github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace)
[![Apache License][license-image]][license-image]

OpenTelemetry Google Cloud Trace Exporter allow the user to send collected traces and spans to Google Cloud.

[Google Cloud Trace](https://cloud.google.com/trace) is a distributed tracing backend system. It helps developers to gather timing data needed to troubleshoot latency problemts in microservice architecture as well as monolithic architecture. It manages both the collection and lookup of gathered trace data.

## Setup

Google Cloud Trace is a managed service provided by Google Cloud Platform. The end-to-end set up guide with OpenTelemetry is available on [the office document](https://cloud.google.com/trace/docs/setup/go-ot), so this document go through the exporter set up.

## Usage

Add `github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace` to the import list and set up `go.mod` file accordingly.

```go
import texpoter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
```

Once you import the trace exporter package, then register the exporter to the application, and start tracing. If you are running in a GCP environment, the exporter will automatically authenticate using the environment's service account. If not, you will need to follow the instruction in [Authentication](#Authentication).

```go
// Create exporter.
projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
exporter, err := texporter.NewExporter(texporter.WithProjectID(projectID))
if err != nil {
    log.Fatalf("texporter.NewExporter: %v", err)
}
// Create trace provider with the exporter.
//
// By default it uses AlwaysSample() which samples all traces.
// In a production environment or high QPS setup please use
// ProbabilitySampler set at the desired probability.
// Example:
//   config := sdktrace.Config{DefaultSampler:sdktrace.ProbabilitySampler(0.0001)}
//   tp, err := sdktrace.NewProvider(sdktrace.WithConfig(config), ...)
tp, err := sdktrace.NewProvider(sdktrace.WithSyncer(exporter))
if err != nil {
        log.Fatal(err)
}
global.SetTraceProvider(tp)

// Create custom span.
tracer := global.TraceProvider().Tracer("example.com/trace")
tracer.WithSpan(context.Background(), "foo",
        func(_ context.Context) error {
                // Do some work.
                return nil
        })
```

## Authentication

The Google Cloud Trace exporter depends upon [`google.FindDefaultCredentials`](https://pkg.go.dev/golang.org/x/oauth2/google?tab=doc#FindDefaultCredentials), so the service account is automatically detected by default, but also the custom credential file (so called `service_account_key.json`) can be detected with specific conditions. Quoting from the document of `google.FindDefaultCredentials`:

* A JSON file whose path is specified by the `GOOGLE_APPLICATION_CREDENTIALS` environment variable.
* A JSON file in a location known to the gcloud command-line tool. On Windows, this is `%APPDATA%/gcloud/application_default_credentials.json`. On other systems, `$HOME/.config/gcloud/application_default_credentials.json`.

## Useful links

* For more information on OpenTelemetry, visit: https://opentelemetry.io/
* For more about OpenTelemetry Go, visit: https://github.com/open-telemetry/opentelemetry-go
* Learn more about Google Cloud Trace at https://cloud.google.com/trace