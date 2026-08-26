# Open-Telemetry Operations Exporters for Go

[![Build Status][circleci-image]][circleci-url]

> [!WARNING]
> The OpenTelemetry Google Cloud Trace and Monitoring exporters in this repository are deprecated and will be archived after January 1st, 2027.
> Please migrate to the standard OpenTelemetry OTLP exporters. For migration instructions, see the [Migration Guide](MIGRATION.md).

This repository contains the source code of 2 packages of OpenTelemetry exporters to [Google Cloud Trace](https://cloud.google.com/trace) and [Google Cloud Monitoring](https://cloud.google.com/monitoring).

To get started with instrumentation in Google Cloud, see [Generate traces and metrics with
Go](https://cloud.google.com/stackdriver/docs/instrumentation/setup/go).

To learn more about instrumentation and observability, including opinionated recommendations
for Google Cloud Observability, visit [Instrumentation and
observability](https://cloud.google.com/stackdriver/docs/instrumentation/overview).

## OpenTelemetry Google Cloud Trace Exporter

> [!WARNING]
> Deprecated: Please migrate to the standard OpenTelemetry OTLP trace exporter. See [MIGRATION.md](MIGRATION.md) for details.

OpenTelemetry Google Cloud Trace Exporter allows the user to send collected traces and spans to Google Cloud.

See [README.md](https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/blob/main/exporter/trace/README.md) for setup and usage information.

## OpenTelemetry Google Cloud Monitoring Exporter

> [!WARNING]
> Deprecated: Please migrate to the standard OpenTelemetry OTLP metric exporter. See [MIGRATION.md](MIGRATION.md) for details.

OpenTelemetry Google Cloud Monitoring Exporter allows the user to send collected metrics to Google Cloud Monitoring.

See [README.md](https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/blob/main/exporter/metric/README.md) for setup and usage information.

[circleci-image]: https://circleci.com/gh/GoogleCloudPlatform/opentelemetry-operations-go.svg?style=shield 
[circleci-url]: https://circleci.com/gh/GoogleCloudPlatform/opentelemetry-operations-go
