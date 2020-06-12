# Open-Telemetry Operations Exporters for Go

This repository contains the source code of 2 packages of OpenTelemetry exporters to [Google Cloud Trace](https://cloud.google.com/trace) and [Google Cloud Monitoring](https://cloud.google.com/monitoring).

## OpenTelemetry Google Cloud Trace Exporter

OpenTelemetry Google Cloud Trace Exporter allow the user to send collected traces and spans to Google Cloud.

[Google Cloud Trace](https://cloud.google.com/trace) is a distributed tracing backend system. It helps developers to gather timing data needed to troubleshoot latency problems in microservice & monolithic architectures. It manages both the collection and lookup of gathered trace data.

This exporter package assumes your application is [already instrumented](https://github.com/open-telemetry/opentelemetry-go/blob/master/example/http/client/client.go) with the OpenTelemetry SDK. Once you are ready to export OpenTelemetry data, you can add this exporter to your application. See [README.md](https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/blob/master/exporter/trace/README.go) for setup and usage information.

## OpenTelemetry Google Cloud Monitoring Exporter

OpenTelemetry Google Cloud Monitoring Exporter allows the user to send collected metrics to Google Cloud Monitoring.

See [README.md](https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/blob/master/exporter/metrics/README.go) for setup and usage information.
