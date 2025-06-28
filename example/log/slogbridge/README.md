# Google Cloud Logging exporter example with OpenTelemetry Collector

This example shows how to use [`go.opentelemetry.io/otel`](https://pkg.go.dev/go.opentelemetry.io/otel/) to instrument a simple Go application with OpenTelemetry logs and export the logs to [Google Cloud Logging](https://cloud.google.com/logging/) using the [Google-Built OpenTelemetry Collector](https://cloud.google.com/stackdriver/docs/instrumentation/google-built-otel).

## Setup environment

Before sending metrics to Google Cloud Logging, confirm the Cloud Logging API is enabled for the project you will use to access the API as described in [this document](https://cloud.google.com/logging/docs/api/enable-api).

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
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```

## Build and run the application

> [!IMPORTANT]
> For running this example, you need to have a locally running OpenTelemetry Collector, configured using the provided [sample config](./collector/config.yaml). 
> Instructions for running OpenTelemetry Collector on your system can be found [here](https://opentelemetry.io/docs/collector/getting-started/#local),
> but a convenience script has been provided that can run the collector using `Docker`. 

Once you ensure the API is enabled, then build the example application and run the executable.
Change the current directory to the example using `cd examples/log/slogbridge`, and then run the example using following commands:

```
// From the exmaples/log/slogbridge directory
// Start the collector with the provided config
$ ./collector/run_collector.sh ./collector/config.yaml
// From a seperate terminal window - build & run the application
$ go build -o otel_logs
$ ./otel_logs
...
```

The application emits logs to the endpoint `http://localhost:4318` and the collector is configured to listen at this port.

> [!TIP]
> The endpoint to which logs are emitted can be changed by using the environment variable `OTEL_EXPORTER_OTLP_ENDPOINT`.
> If you switch the endpoint, remember to configure the collector to listen to the newly configured endpoint.

## Visualize exported logs

Once the application has run successfully, it should have sent structured logs with OpenTelemetry attributes to Google Cloud Logging.\
You should be able to search for these logs using [Logs Explorer](https://cloud.google.com/logging/docs/view/logs-explorer-interface).
In the Logs Explorer, you can search logs by various attributes, for example, service name:
```
// Put in the following query in the logs explorer to show all logs from the current sample
labels."service.name"="example-application"
```
