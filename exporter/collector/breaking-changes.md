# Breaking changes vs old googlecloud exporter

The new pdata based exporter has some breaking changes from the original OpenCensus stackdriver
based `googlecloud` exporter:

## Metric Names and Descriptors

The previous collector exporter would default to sending metrics with the type:
`custom.googleapis.com/OpenCensus/{metric_name}`.  This has been changed to
`workload.googleapis.com/{metric_name}`.

Additionally, the previous exporter had a hardcoded list of known metric domains
where this "prefix" would not be used. The new exporter allows full configuration
of this list via the `metric.known_domains` property.

Additionally, the DisplayName for a metric used to be exactly the
`{metric_name}`. Now, the metric name is chosen as the full-path after the
domain name of the metric type.  E.g. if a metric called
`workload.googleapis.com/nginx/latency` is created, the display name will
be `nginx/latency` instead of `workload.googleapis.com/nginx/latency`.

## Labels

Original label key mapping code is
[here](https://github.com/census-ecosystem/opencensus-go-exporter-stackdriver/blob/42e7e58efdb937e8477f827d3fba022212335dbc/sanitize.go#L26).
The new code does not:

- truncate label keys longer than 100 characters.
- prepend `key` when the first character is `_`.

## OTLP Sum

In the old exporter, delta sums were converted into GAUGE points ([see test
fixture](https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/blob/9bc1f49ebe000b0b3b1aa5b7f201e7996effdcd8/exporter/collector/testdata/fixtures/delta_counter_metrics_expect.json#L15)).
The new pdata exporter sends these as CUMULATIVE points with the same delta time window
(reseting at each point) aka pseudo-cumulatives.

## OTLP Summary

The old exporter relied on upstream conversion of OTLP Summary into Gauge and
Cumulative points.  The new exporter performas this conversion itself, which
means summary metric descriptors will include label description for `percentile`
labels.