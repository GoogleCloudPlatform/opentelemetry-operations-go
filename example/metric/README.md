# Google Cloud Monitoring exporter example

This example shows how to use [`go.opentelemetry.io/otel`](https://pkg.go.dev/go.opentelemetry.io/otel/) to instrument a simple Go application for metrics and export the metrics to [Google Cloud Monitoring](https://cloud.google.com/monitoring/)

## Build and run the application

Before sending metrics to Google Cloud Monitoring, confirm your current environment with `gcloud` command.

```
$ gcloud config get-value project
Your active configuration is: [example]
foo-project-214354
```

In this case, the project ID is `foo-project-214354`. Confirm whether Cloud Monitoring API is enabled in the project. Once you ensure the API is enabled, then build the example application and run the executable.

```
$ go build -o metrics
$ ./metrics
2020/06/11 21:11:15 Most recent data: counter 110, observer 13.45
2020/06/11 21:11:15 Most recent data: counter 160, observer 16.02
2020/06/11 21:11:15 Most recent data: counter 134, observer 14.33
2020/06/11 21:11:15 Most recent data: counter 125, observer 15.12
...
```

In order to have enough amount of metrics to create charts in the dashboard for Counter and Observer values in the example, keep the application running at least for 5 min before start creating the dashboard.

## Create dashboard

https://console.cloud.google.com/monitoring/dashboards?project=<your-project-id>

Once you think you send suffient data, then create dashboard. If you are learning how to use Google Cloud Monitoring, you can follow how to use charts step by step on [this document](https://cloud.google.com/monitoring/charts).

In the document, you may be asked for the name of metrics types. They are "custom.googleapis.com/opentelemetry/counter-a" and "custom.googleapis.com/opentelemetry/observer-a" respectively.

If you already know how to use Cloud Monitoring and just would like to confirm the data are properly received, you can run the dashboard craetion script bundled in this directly.

```
$ ./create_dashboard.sh
```

This script creates a dashboard titled "OpenTelemetry exporter example/metric".

You'll find the line charts like below once you create the dashboard.

<img width="1200" alt="2 charts in dashboard" src="images/charts.png?raw=true"/>
