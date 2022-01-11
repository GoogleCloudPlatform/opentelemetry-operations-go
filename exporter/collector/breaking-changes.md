# Breaking changes vs old googlecloud exporter

The new pdata based exporter has some breaking changes from the original OpenCensus (OC)
stackdriver based `googlecloud` exporter:

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

## Monitored Resources

Mapping from OTel Resource to GCM monitored resource has been completely changed. The OC based
exporter worked by converting the OTel resource into an OC resource which the exporter
recognized. The `resource_mappings` config option allowed customizing this conversion so the OC
exporter would correctly convert to a GCM monitored resource.

Then new pdata based exporter works by interpreting the [OTel Resource semantic
conventions](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/resource/semantic_conventions/README.md)
as follows to determine the monitored resource type:

- Switch on the `cloud.platform` Resource attribute and if:
  - `gcp_compute_engine`, send a `gce_instance` monitored resource.
  - `gcp_kubernetes_engine`, send the most specific possible k8s monitored resource depending
  on which resource keys are present and non-empty. In order, try for `k8s_container`,
  `k8s_pod`, `k8s_node`, `k8s_cluster`.
  - `aws_ec2`, send a `aws_ec2_instance` monitored resource.
- Otherwise, fallback to:
  - `generic_task` if the `service.name` and `service.instance_id` resource attributes are
  present and non-empty.
  - `generic_node`

Once the type is determine, the monitored resource labels are populated from the mappings
defined in [`monitoredresource.go`](monitoredresource.go#L51). The new behavior will never send the
`global` monitored resource.

For now, it is not possible to customizate the mapping algorithm, beyond using the
[`resourceprocessor`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourceprocessor)
in the collector pipeline before the exporter. If you have a use case for customizing the
behavior, please open an issue.

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
Cumulative points.  The new exporter performs this conversion itself, which
means summary metric descriptors will include label description for `percentile`
labels.

## Self Observability Metrics

For each OTLP Summary metric point, the old exporter would add 1 to the
`googlecloudmonitoring/point_count` self-observability counter. For a Summary point with N
percentile values, the new exporter will add `N + 2` (one for each percentile timeseries, one
for count, and one for sum) to the counter.
