# Breaking changes vs old googlecloud exporter

The new pdata based exporter has some breaking changes from the original OpenCensus stackdriver
based `googlecloud` exporter:

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
