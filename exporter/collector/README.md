# Google Cloud Exporter

This exporter can be used to send metrics and traces to Google Cloud Monitoring and Trace (formerly known as Stackdriver) respectively.

## Getting started

In general, authenticating with the Collector exporter follows the same steps as
any other app using the steps documented for [Application Default
Credentials](https://cloud.google.com/docs/authentication/provide-credentials-adc). This
section explains the specific use cases relevant to the exporter.

### Prerequisite: Authenticating

The exporter relies on GCP client libraries to send data to Google Cloud. Use of these libraries requires the caller (the Collector) to be authenticated with a GCP account and project. This should be done using a [GCP service account](https://cloud.google.com/compute/docs/access/service-accounts) with at minimum the following IAM roles (depending on the type of data you wish to send):

*   [Metrics](https://cloud.google.com/iam/docs/understanding-roles#monitoring-roles): `roles/monitoring.metricWriter`
*   [Traces](https://cloud.google.com/iam/docs/understanding-roles#cloud-trace-roles): `roles/cloudtrace.agent`
*   [Logs](https://cloud.google.com/iam/docs/understanding-roles#logging-roles): `roles/logging.logWriter`

The [Compute Engine default service account](https://cloud.google.com/compute/docs/access/service-accounts#default_service_account) has all of these permissions by default, but if you are running on a different platform or with a different GCP service account you will need to ensure your service account has these permissions.

#### Options for different environments

Depending on the environment where your Collector is running, you can authenticate one of several ways:

**GCE instances**

On GCE it is recommended to use the [GCP service account](https://cloud.google.com/compute/docs/access/service-accounts) associated with your instance. If this is the Compute Engine default service account or another GCP service account with the sufficient IAM permissions, then there is nothing additional you need to do to authenticate the Collector process. Simply run the Collector on your instance, and it will inherit these permissions.

**GKE / Workload Identity**

On GKE clusters with Workload Identity enabled (including GKE Autopilot), follow [the steps to configure a Workload Identity ServiceAccount in your cluster](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity) (if you do not already have one). Then, deploy the Collector as you would with any other workload, setting the `serviceAccountName` field in the Collector Pod’s `.spec` to the WI-enabled ServiceAccount.

In non-WI clusters, you can use the GCP service account associated with the node the same way as in the instructions for GCE instances above.

**Non-GCP (AWS, Azure, on-prem, etc.) or alternative service accounts**

In non-GCP environments, a [service account key](https://cloud.google.com/iam/docs/keys-create-delete#iam-service-account-keys-create-console) or credentials file is required. The exporter will automatically look for this file using the `GOOGLE_APPLICATION_CREDENTIALS` environment variable or, if that is unset, one of the [other known locations](https://cloud.google.com/docs/authentication/application-default-credentials). Note that when using this approach, you may need to explicitly set the `project` option in the exporter’s config.

When running the Collector in a Docker container, a credentials file can be passed to the container via volume mounts and environment variables at runtime like so:

```
docker run \
  --volume ~/service-account-key.json:/etc/otel/key.json \
  --volume $(pwd)/config.yaml:/etc/otel/config.yaml \
  --env GOOGLE_APPLICATION_CREDENTIALS=/etc/otel/key.json \
  --expose 4317 \
  --expose 55681 \
  --rm \
  otel/opentelemetry-collector-contrib
```

**Using `gcloud auth application-default login`**

Using [`gcloud auth application-default login`](https://cloud.google.com/docs/authentication/application-default-credentials) to authenticate is not recommended for production use. Instead, it’s best to use a GCP service account through one of the methods listed above. The `gcloud auth` command can be useful for development and testing on a user account, and authenticating with it follows the same approach as the service account key method above.

### Running the Collector

These instructions are to get you up and running quickly with the GCP exporter in a local development environment. We'll also point out alternatives that may be more suitable for CI or production.

1.  **Obtain a Collector binary.** Pull a binary or Docker image for the
    OpenTelemetry contrib collector which includes the GCP exporter plugin
    through one of the following:

    *   Download a [binary or package of the OpenTelemetry
        Collector Contrib](https://github.com/open-telemetry/opentelemetry-collector-releases/releases)
        that is appropriate for your platform, and includes the Google Cloud
        exporter.
    *   Pull a Docker image with `docker pull otel/opentelemetry-collector-contrib`
    *   Create your own main package in Go, that pulls in just the plugins you need.
    *   Use the [OpenTelemetry Collector
        Builder](https://github.com/open-telemetry/opentelemetry-collector-builder)
        to generate the Go main package and `go.mod`.


2.  **Create a configuration file `config.yaml`.** The example below shows a minimal recommended configuration that receives OTLP and sends data to GCP, in addition to verbose logging to help understand what is going on. It uses application default credentials (which we will set up in the next step).

    Note that this configuration includes the recommended `memory_limiter` and `batch` plugins, which avoid high latency for reporting telemetry, and ensure that the collector itself will stay stable (not run out of memory) by dropping telemetry if needed.

    ```yaml
    receivers:
      otlp:
        protocols:
          grpc:
          http:
    exporters:
      googlecloud:
        # Google Cloud Monitoring returns an error if any of the points are invalid, but still accepts the valid points.
        # Retrying successfully sent points is guaranteed to fail because the points were already written.
        # This results in a loop of unnecessary retries.  For now, disable retry_on_failure.
        retry_on_failure:
          enabled: false
      logging:
        loglevel: debug
    processors:
      memory_limiter:
      batch:
        send_batch_max_size: 200
        send_batch_size: 200
    service:
      pipelines:
        traces:
          receivers: [otlp]
          processors: [memory_limiter, batch]
          exporters: [googlecloud, logging]
        metrics:
          receivers: [otlp]
          processors: [memory_limiter, batch]
          exporters: [googlecloud, logging]
    ```

3.  **Set up credentials.**

    1.  Enable billing in your GCP project.

    2.  Enable the Cloud Metrics and Cloud Trace APIs.

    3.  Ensure that your user GCP user has (at minimum) `roles/monitoring.metricWriter` and `roles/cloudtrace.agent`. You can learn about [metric-related](https://cloud.google.com/monitoring/access-control) and [trace-related](https://cloud.google.com/trace/docs/iam) IAM in the GCP documentation.

    4.  Obtain credentials using one of the methods in the [Authenticating
        section](#prerequisite-authenticating) above.

4.  **Run the collector.** The following  runs the collector in the foreground, so please execute it in a separate terminal.

    ```sh
    ./otelcol-contrib --config=config.yaml
    ```

    <details>
    <summary>Alternatives</summary>

    If you obtained OS-specific packages or built your own binary in step 1, you'll need to follow the appropriate conventions for running the collector.

    </details>

5.  **Gather telemetry.** Run an application that can submit OTLP-formatted metrics and traces, and configure it to send them to `127.0.0.1:4317` (for gRPC) or `127.0.0.1:55681` (for HTTP).

    <details>
      <summary>Alternatives</summary>

      *   Set up the host metrics receiver, which will gather telemetry from the host without needing an external application to submit telemetry.

      *   Set up an application-specific receiver, such as the Nginx receiver, and run the corresponding application.

      *   Set up a receiver for some other protocol (such Prometheus, StatsD, Zipkin or Jaeger), and run an application that speaks one of those protocols.
    </details>

6.  **View telemetry in GCP.** Use the GCP [metrics explorer](https://console.cloud.google.com/monitoring/metrics-explorer) and [trace overview](https://console.cloud.google.com/traces) to view your newly submitted telemetry.

## Configuration reference

The following configuration options are supported:

- `project` (optional): GCP project identifier.
- `endpoint` (optional): Endpoint where data is going to be sent to.
- `user_agent` (optional): Override the user agent string sent on requests to Cloud Monitoring (currently only applies to metrics). Specify `{{version}}` to include the application version number. Defaults to `opentelemetry-collector-contrib {{version}}`.
- `use_insecure` (optional): If true. use gRPC as their communication transport. Only has effect if Endpoint is not "".
- `timeout` (optional): Timeout for all API calls. If not set, defaults to 12 seconds.
- `compression` (optional): Enable gzip compression on gRPC calls for Metrics or Logs. Valid values: `gzip`.
- `resource_mappings` (optional): ResourceMapping defines mapping of resources from source (OpenCensus) to target (Google Cloud).
  - `label_mappings` (optional): Optional flag signals whether we can proceed with transformation if a label is missing in the resource.
- `retry_on_failure` (optional): Configuration for how to handle retries when sending data to Google Cloud fails.
  - `enabled` (default = true)
  - `initial_interval` (default = 5s): Time to wait after the first failure before retrying; ignored if `enabled` is `false`
  - `max_interval` (default = 30s): Is the upper bound on backoff; ignored if `enabled` is `false`
  - `max_elapsed_time` (default = 120s): Is the maximum amount of time spent trying to send a batch; ignored if `enabled` is `false`
- `sending_queue` (optional): Configuration for how to buffer traces before sending.
  - `enabled` (default = true)
  - `num_consumers` (default = 10): Number of consumers that dequeue batches; ignored if `enabled` is `false`
  - `queue_size` (default = 5000): Maximum number of batches kept in memory before data; ignored if `enabled` is `false`;
    User should calculate this as `num_seconds * requests_per_second` where:
    - `num_seconds` is the number of seconds to buffer in case of a backend outage
    - `requests_per_second` is the average number of requests per seconds.
- `destination_project_quota` (optional): Counts quota for traces and metrics against the project to which the data is sent (as opposed to the project associated with the Collector's service account. For example, when setting `project_id` or using [multi-project export](#multi-project-exporting). (default = false)

Note: These `retry_on_failure` and `sending_queue` are provided (and documented) by the [Exporter Helper](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/exporterhelper#configuration)

Additional configuration for the metric exporter:

- `metric.prefix` (optional): MetricPrefix overrides the prefix / namespace of the Google Cloud metric type identifier. If not set, defaults to "custom.googleapis.com/opencensus/"
- `metric.skip_create_descriptor` (optional): Whether to skip creating the
  metric descriptor.
- `metric.experimental_wal_config.directory` (optional): Path to local write-ahead-log file.
- `metric.experimental_wal_config.max_backoff` (optional): Maximum duration to retry entries from
  the WAL on network errors.

Addition configuration for the logging exporter:

- `log.default_log_name` (optional): Defines a default name for log entries. If
left unset, and a log entry does not have the `gcp.log_name` attribute set, the
exporter will return an error processing that entry.
- `log.error_reporting_type` (option, default = false): If `true`, log records
with a severity of `error` or higher will be converted to JSON payloads with the
`@type` field set for [GCP Error
Reporting](https://cloud.google.com/error-reporting/docs/formatting-error-messages#log-text).
If the body is currently a string, it will be converted to a `message` field in
the new JSON payload. If the body is already a map, the `@type` field will be
added to the map. Other body types (such as byte) are undefined for this
behavior.

Example:

```yaml
exporters:
  googlecloud:
    # Google Cloud Monitoring returns an error if any of the points are invalid, but still accepts the valid points.
    # Retrying successfully sent points is guaranteed to fail because the points were already written.
    # This results in a loop of unnecessary retries.  For now, disable retry_on_failure.
    retry_on_failure:
      enabled: false
    project: my-project
    endpoint: test-endpoint
    user_agent: my-collector {{version}}
    use_insecure: true
    timeout: 12s

    resource_mappings:
      - source_type: source.resource1
        target_type: target-resource1
        label_mappings:
          - source_key: contrib.opencensus.io/exporter/googlecloud/project_id
            target_key: project_id
            optional: true
          - source_key: source.label1
            target_key: target_label_1

    sending_queue:
      enabled: true
      num_consumers: 2
      queue_size: 50

    metric:
      prefix: prefix
      skip_create_descriptor: true
      compression: gzip

    log:
      default_log_name: my-app
```

Beyond standard YAML configuration as outlined in the sections that follow,
exporters that leverage the net/http package (all do today) also respect the
following proxy environment variables:

* HTTP_PROXY
* HTTPS_PROXY
* NO_PROXY

If set at Collector start time then exporters, regardless of protocol,
will or will not proxy traffic as defined by these environment variables.

### Preventing metric label collisions

The metrics exporter can add metric labels to timeseries, such as when setting
`metric.service_resource_labels`, `metric.instrumentation_library_labels` (both
on by default), or when using `metric.resource_filters` to convert resource
attributes to metric labels.

However, if your metrics already contain any of these labels they will fail to
export to Google Cloud with a `Duplicate label key encountered` error. Such
labels from the default features above include:

* `service_name`
* `service_namespace`
* `service_instance_id`
* `instrumentation_source`
* `instrumentation_version`

*(Note that these are the sanitized versions of OpenTelemetry attributes, with `.` replaced by `_` to be compatible with Cloud Monitoring. For example, `service_name` comes from the [`service.name` resource attribute](https://github.com/open-telemetry/opentelemetry-specification/blob/dc78006c12d9767fd2e35b691706c7572a76fd43/specification/resource/semantic_conventions/README.md#service).)*

To prevent this, it's recommended to use the [transform processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/c7bd50ce773e66be327ef7618775a884a774e5d1/processor/transformprocessor) in your collector config to rename existing metric labels to preserve them, for example:

```yaml
processors:
  transform:
    metric_statements:
    - context: datapoint
      statements:
      - set(attributes["exported_service_name"], attributes["service_name"])
      - delete_key(attributes, "service_name")
      - set(attributes["exported_service_namespace"], attributes["service_namespace"])
      - delete_key(attributes, "service_namespace")
      - set(attributes["exported_service_instance_id"], attributes["service_instance_id"])
      - delete_key(attributes, "service_instance_id")
      - set(attributes["exported_instrumentation_source"], attributes["instrumentation_source"])
      - delete_key(attributes, "instrumentation_source")
      - set(attributes["exported_instrumentation_version"], attributes["instrumentation_version"])
      - delete_key(attributes, "instrumentation_version")
```

The same method can be used for any resource attributes being filtered to metric
labels, or metric labels which might collide with the GCP monitored resource
used with resource detection.

Keep in mind that your conflicting attributes may contain dots instead of
underscores (eg, `service.name`), but these will still collide once all
attributes are normalized to metric labels. In this case you will need to update
the collector config above appropriately.

### Performance tuning

The exporter can be configured for performance on higher-throughput systems with
the `batch` processor and client connection settings in the exporter itself.

For example, see the following config snippet:

```yaml
processors:
  batch:
    send_batch_max_size: 6000
    send_batch_size: 6000
    
exporters:
  googlecloud:
    grpc_pool_size: 20
    sending_queue:
      num_consumers: 40
```

This uses the `batch` processor to set a batch size of `6000`, and in the
`googlecloud` exporter it sets `grpc_pool_size` (the number of simultaneous
connections in the GCP client) to `20` and the `sending_queue` (the number of
consumers that dequeue batches) to `40`.

There is no guaranteed one-size-fits-all combination of settings, and for the
majority of use cases the defaults will provide more than sufficient
throughput. But in certain outlier cases, these options are a good starting
point to tune the exporter for maximum efficiency depending on your needs.

### Logging Exporter

The logging exporter processes OpenTelemetry log entries and exports them to GCP Cloud Logging. Logs can be collected using one 
of the opentelemetry-collector-contrib log receivers, such as the [filelogreceiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filelogreceiver).

Log entries must contain any Cloud Logging-specific fields as a matching OpenTelemetry attribute (as shown in examples from the 
[logs data model](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/data-model.md#google-cloud-logging)). 
These attributes can be parsed using the various [log collection operators](https://github.com/open-telemetry/opentelemetry-log-collection/blob/main/docs/operators/README.md#what-operators-are-available) available upstream.

For example, the following config parses the [HTTPRequest field](https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#HttpRequest) from Apache log entries saved in `/var/log/apache.log`. 
It also parses out the `timestamp` and inserts a non-default `log_name` attribute and GCP [MonitoredResource](https://cloud.google.com/logging/docs/reference/v2/rest/v2/MonitoredResource) attribute.

```yaml
receivers:
  filelog:
    include: [ /var/log/apache.log ]
    start_at: beginning
    operators:
      - id: http_request_parser
        type: regex_parser
        regex: '(?m)^(?P<remoteIp>[^ ]*) (?P<host>[^ ]*) (?P<user>[^ ]*) \[(?P<time>[^\]]*)\] "(?P<requestMethod>\S+)(?: +(?P<requestUrl>[^\"]*?)(?: +(?P<protocol>\S+))?)?" (?P<status>[^ ]*) (?P<responseSize>[^ ]*)(?: "(?P<referer>[^\"]*)" "(?P<userAgent>[^\"]*)")?$'
        parse_to: attributes["gcp.http_request"]
        timestamp:
          parse_from: attributes["gcp.http_request"].time
          layout_type: strptime
          layout: '%d/%b/%Y:%H:%M:%S %z'
    converter:
      max_flush_count: 100
      flush_interval: 100ms

exporters:
  googlecloud:
    project: my-gcp-project
    log:
      default_log_name: opentelemetry.io/collector-exported-log

processors:
  memory_limiter:
      check_interval: 1s
      limit_mib: 4000
      spike_limit_mib: 800
  resourcedetection:
    detectors: [gce, gke]
    timeout: 10s
  attributes:
    # Override the default log name.  `gcp.log_name` takes precedence
    # over the `default_log_name` specified in the exporter.
    actions:
      - key: gcp.log_name
        action: insert
        value: apache-access-log

service:
    logs:
      receivers: [filelog]
      processors: [memory_limiter, resourcedetection, attributes]
      exporters: [googlecloud]

```

This would parse logs of the following example structure:

```
127.0.0.1 - - [26/Apr/2022:22:53:36 +0800] "GET / HTTP/1.1" 200 1247
```

To the following GCP entry structure:

```
        {
          "logName": "projects/my-gcp-project/logs/apache-access-log",
          "resource": {
            "type": "gce_instance",
            "labels": {
              "instance_id": "",
              "zone": ""
            }
          },
          "textPayload": "127.0.0.1 - - [26/Apr/2022:22:53:36 +0800] \"GET / HTTP/1.1\" 200 1247",
          "timestamp": "2022-05-02T12:16:14.574548493Z",
          "httpRequest": {
            "requestMethod": "GET",
            "requestUrl": "/",
            "status": 200,
            "responseSize": "1247",
            "remoteIp": "127.0.0.1",
            "protocol": "HTTP/1.1"
          }
        }
```

The logging exporter also supports the full range of [GCP log severity levels](https://cloud.google.com/logging/docs/reference/v2/rpc/google.logging.type#google.logging.type.LogSeverity), 
which differ from the available [OpenTelemetry log severity levels](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/data-model.md#severity-fields). 
To accommodate this, the following mapping is used to equate an incoming OpenTelemetry [`SeverityNumber`](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/data-model.md#field-severitynumber) 
to a matching GCP log severity:

|OTel `SeverityNumber`/Name|GCP severity level|
|---|---|
|Undefined|Default|
|1-4 / Trace|Debug|
|5-8 / Debug|Debug|
|9-10 / Info|Info|
|11-12 / Info|Notice|
|13-16 / Warn|Warning|
|17-20 / Error|Error|
|21-22 / Fatal|Critical|
|23 / Fatal|Alert|
|24 / Fatal|Emergency|

The upstream [severity parser](https://github.com/open-telemetry/opentelemetry-log-collection/blob/main/docs/types/severity.md) (along 
with the [regex parser](https://github.com/open-telemetry/opentelemetry-log-collection/blob/main/docs/operators/regex_parser.md)) allows for 
additional flexibility in parsing log severity from incoming entries.

## Multi-Project exporting

By default, the exporter sends telemetry to the project specified by `project` in the configuration. This can be overridden on a per-metrics basis using the `gcp.project.id` resource attribute. For example, if a metric has a label `project`, you could use the `groupbyattrs` processor to promote it to a resource label, and the `resource` processor to rename the attribute from `project` to `gcp.project.id`.

### Multi-Project quota usage

The `gcp.project.id` label can be combined with the `destination_project_quota` option to attribute quota usage to the project parsed by the label. This feature is available
for traces, metrics, and logs. The Collector's default service account will need `roles/serviceusage.serviceUsageConsumer` IAM permissions in the destination quota project.

Note that this option will not work  if a quota project is already defined in your Collector's GCP credentials. In this case, the telemetry will fail to export with a "project not found" error.
This can be done by manually editing your [ADC file](https://cloud.google.com/docs/authentication/application-default-credentials#personal) (if it exists) to remove the `quota_project_id` entry line.

## Recommendations

It is recommended to always run a [batch processor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor)
and [memory limiter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/memorylimiterprocessor) for tracing pipelines to ensure
optimal network usage and avoiding memory overruns.  You may also want to run an additional
[sampler](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/probabilisticsamplerprocessor), depending on your needs.

## Deprecations

The previous trace configuration (v0.21.0) has been deprecated in favor of the common configuration options available in OpenTelemetry. These will cause a failure to start
and should be migrated:

- `trace.bundle_delay_threshold` (optional): Use `batch` processor instead ([docs](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor)).
- `trace.bundle_count_threshold` (optional): Use `batch` processor instead ([docs](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/batchprocessor)).
- `trace.bundle_byte_threshold` (optional): Use `memorylimiter` processor instead ([docs](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/memorylimiterprocessor))
- `trace.bundle_byte_limit` (optional): Use `memorylimiter` processor instead ([docs](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/memorylimiterprocessor))
- `trace.buffer_max_bytes` (optional): Use `memorylimiter` processor instead ([docs](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/memorylimiterprocessor))
