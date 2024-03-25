# Google Cloud Monitoring exporter example

This example shows how to use [`go.opentelemetry.io/otel`](https://pkg.go.dev/go.opentelemetry.io/otel/) to instrument a simple Go application with metrics and export the metrics to [Google Cloud Monitoring](https://cloud.google.com/monitoring/)

## Create dashboard

When filling in the **Find resource type and metric box**, use the metric names "custom.googleapis.com/opentelemetry/counter-a" and "custom.googleapis.com/opentelemetry/observer-a".

If you already know how to use Cloud Monitoring and would just like to confirm the data is properly received, you can run the dashboard creation script bundled in this directory. This command requires at least the [roles/monitoring.dashboardEditor](https://cloud.google.com/monitoring/access-control#dashboard_roles_desc) permissions to create a new dashboard.
```
$ ./create_dashboard.sh
```

This script creates a dashboard titled "OpenTelemetry exporter example/metric".

You should be able to view line charts like below once you create the dashboard.

*Note: This script is configured to create dashboard which displays the metrics generated via the `sdk` example.*

<img width="1200" alt="2 charts in dashboard" src="../images/sdk_charts.png?raw=true"/>