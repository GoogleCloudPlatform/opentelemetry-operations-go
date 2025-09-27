# Export OTLP Traces to Google Cloud with OpenTelemetry Collector

This example shows how to use [`OpenTelemetry`](https://pkg.go.dev/go.opentelemetry.io/otel/) to instrument a simple Go application to generate
and export the OTLP traces to [Google Cloud Trace](https://cloud.google.com/trace/) using the
[Google-Built OpenTelemetry Collector](https://cloud.google.com/stackdriver/docs/instrumentation/google-built-otel).

This sample leverages Google Cloud's [Telemetry API](https://cloud.google.com/stackdriver/docs/reference/telemetry/overview).

## Setup environment

Get Google credentials on your machine:

```shell
gcloud auth application-default login
```
Executing this command will save your application credentials to default path which will depend on the type of machine -
- Linux, macOS: `$HOME/.config/gcloud/application_default_credentials.json`
- Windows: `%APPDATA%\gcloud\application_default_credentials.json`

**NOTE: This method of authentication is not recommended for production environments.**

Next, export the credentials to `GOOGLE_APPLICATION_CREDENTIALS` environment variable - 

For Linux & MacOS:
```shell
export GOOGLE_APPLICATION_CREDENTIALS=$HOME/.config/gcloud/application_default_credentials.json
```

Before sending metrics to Google Cloud Trace via the Telemetry API, confirm the
Telemetry API & Google Cloud Trace API is enabled for the project you will use
to access the API as described in [this document](https://cloud.google.com/trace/docs/migrate-to-otlp-endpoints#enable-apis).

You can find your current project ID using the command:

```
$ gcloud config get-value project
Your active configuration is: [example]
foo-project-214354
```

In this case, the project ID is `foo-project-214354`.

Next, set this project ID to the environment variable `GOOGLE_CLOUD_PROJECT` using the following command:

```
export GOOGLE_CLOUD_PROJECT=$(gcloud config get-value project)
```

## Build and run the application

> [!IMPORTANT]
> For running this example, you need to have a locally running OpenTelemetry Collector, configured using the provided [sample config](./collector/config.yaml). 
> Instructions for running OpenTelemetry Collector on your system can be found [here](https://opentelemetry.io/docs/collector/getting-started/#local),
> but a convenience script has been provided that can run the collector using `Docker`. 

Once you ensure the API is enabled, then build the example application and run the executable.
Change the current directory to the example using `cd examples/trace/collector`, and then run the example using following commands:

```
// From the exmaples/log/slogbridge directory
// Start the collector with the provided config
$ ./deployment/run_collector.sh ./deployment/config.yaml
// From a separate terminal window - build & run the application
$ go build -o otlp_trace_collector
$ ./otlp_trace_collector
...
```

The application emits traces to the endpoint `http://0.0.0.0:4317` and the collector is configured to listen at this port.

> [!TIP]
> The endpoint to which traces are emitted can be changed by using the environment variable `OTEL_EXPORTER_OTLP_ENDPOINT`.
> If you switch the endpoint, remember to configure the collector to listen to the newly configured endpoint.

## Visualize exported traces

Once the application has run successfully, it should have sent OTLP traces with OpenTelemetry attributes to Google Cloud Trace.\
You should be able to search for these traces using [Trace Explorer](https://cloud.google.com/trace/docs/finding-traces) page.
In the Trace Explorer, you can filter traces by various attributes, for example, "Service Name" & "Span Name".\
The traces produced by this sample would have a span name of `test span` and a service name `collector-trace-example`.
