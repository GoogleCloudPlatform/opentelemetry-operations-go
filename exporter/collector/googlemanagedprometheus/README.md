# Google Managed Service for Prometheus Collector Exporter

## Building a container image with the googlemanagedprometheus exporter

In your own fork of [open-telemetry/opentelemetry-collector-releases](https://github.com/open-telemetry/opentelemetry-collector-releases), add your own "distribution" directory within the distributions directory, based on either the otelcol or otelcol-contrib distributions. In the `exporters` list in `manifest.yaml`, add:
```yaml
exporters:
  - gomod: "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus v0.29.0"
```

The syntax of `manifest.yaml` is described in the [Collector Builder documentation](https://github.com/open-telemetry/opentelemetry-collector/blob/54f271b7d473f36b4ecbc21994d59359dbd263f6/cmd/builder/README.md#opentelemetry-collector-builder).

In the `configs` directory, add your collector configuration yaml file, which should look something like:

```yaml
receivers:
    prometheus:
        config:
          scrape_configs:
            # TODO: Add your prometheus scrape configuration here.
            # Using kubernetes_sd_configs with namespaced resources
            # ensures the namespace is set on your metrics.
processors:
    batch:
        # batch metrics before sending to reduce API usage
        send_batch_max_size: 200
        send_batch_size: 200
        timeout: 5s
    memory_limiter:
        # drop metrics if memory usage gets too high
        check_interval: 1s
        limit_percentage: 65
        spike_limit_percentage: 20
    resourcedetection:
        # detect cluster name and location
        detectors: [gce, gke]
        timeout: 10s
exporters:
    googlemanagedprometheus:
```

Change the Dockerfile in your directory within `distributions` to point to your collector config [here](https://github.com/open-telemetry/opentelemetry-collector-releases/blob/main/distributions/otelcol-contrib/Dockerfile#L17).

Finally, build the image: 

```sh
DISTRIBUTIONS=my-distribution make build
```

## Additional Options

The [filterprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/filterprocessor) can filter out metrics. The [metricstransformprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/metricstransformprocessor) can manipulate metrics in a variety of ways, including synthesizing new metrics from other metrics, adding or removing labels, renaming metrics, and scaling metrics. `metric_relabl_configs` within the prometheus receiver configuration can also be used to manipulate metrics.
