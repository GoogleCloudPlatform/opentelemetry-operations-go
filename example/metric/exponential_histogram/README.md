# Google Cloud Monitoring exponential histogram example

This example shows how to use [`go.opentelemetry.io/otel`](https://pkg.go.dev/go.opentelemetry.io/otel/) to instrument a simple Go application with metrics and export the metrics to [Google Cloud Monitoring](https://cloud.google.com/monitoring/)

## Example details

This example simulates server latency by sampling a [Log-Normal Distribution](https://en.wikipedia.org/wiki/Log-normal_distribution) with the parameters $\mu =  3.5 , 5.5$ and $\sigma = .5$. We generate the following three example of server latency :

1. Latency with Log-Normal Distribution.
2. Latency with Shifted Mean Distribution.
3. Latency with Multimodal Distribution (mixture of 1. and 2.).

We explore the resulting distributions with three types of histograms : 

1. Opentelemetry Default Linear Buckets Histogram. Buckets are `0, 5, 10, 25, 50, 75, 100, 250, 500, 1000`
2. Opentelemetry Linear Buckets Histogram. Buckets are `0, 10, 20, 30, ..., 340, 350`. 
3. Opentelemetry Exponential Buckets Histogram with default parameters.

## Create dashboard

When filling in the **Find resource type and metric box**, use the metric names with the prefix "workload.googleapis.com/latency_" to observe histogram metrics (for example "workload.googleapis.com/latency_a").

If you already know how to use Cloud Monitoring and would just like to confirm the data is properly received, you can run the dashboard creation script bundled in this directory. This command requires at least the [roles/monitoring.dashboardEditor](https://cloud.google.com/monitoring/access-control#dashboard_roles_desc) permissions to create a new dashboard.

```
$ ./create_dashboard.sh
```

This script creates a dashboard titled "OpenTelemetry - Exponential Histogram example".

You should be able to view histogram charts like below once you create the dashboard.

<img width="1200" alt="2 charts in dashboard" src="../images/exponential_histogram_charts.png?raw=true"/>
