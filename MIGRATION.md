# Migration Guide

This guide provides instructions on how to migrate from the deprecated Google Cloud Trace and Monitoring exporters in this repository to the standard OpenTelemetry OTLP exporters.

## Overview

Google Cloud supports native OTLP (OpenTelemetry Protocol) ingestion for Cloud Trace and Cloud Monitoring via the [Telemetry API](https://docs.cloud.google.com/stackdriver/docs/reference/telemetry/overview). This allows you to use the standard OpenTelemetry OTLP exporters for sending telemetry data to Google Cloud.

The legacy exporters in this repository are deprecated and will be archived after January 1st, 2027.

---

## Migrate from Google Cloud Trace Exporter to OTLP Exporter

For detailed information on Google Cloud Trace's native OTLP ingestion, see the official Google Cloud documentation on [Migrating to OTLP endpoints](https://docs.cloud.google.com/trace/docs/migrate-to-otlp-endpoints).

To migrate from `github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace` to the standard OpenTelemetry OTLP trace exporter:

### 1. Add Dependencies

Add the standard OpenTelemetry OTLP trace exporter, GCP resource detector, and gRPC OAuth credentials to your `go.mod`:

```bash
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
go get go.opentelemetry.io/contrib/detectors/gcp
go get google.golang.org/grpc/credentials/oauth
```

### 2. Update Initialization Code

Replace the `texporter.New()` initialization with `otlptracegrpc.New()` configured with Google Application Default Credentials (ADC) and GCP resource detection:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/oauth"
)

func initTracer(ctx context.Context) (func(), error) {
	// Configure gRPC client with Google Application Default Credentials
	creds, err := oauth.NewApplicationDefault(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load application default credentials: %w", err)
	}

	res, err := resource.New(
		ctx,
		// Detect GCP platform information
		resource.WithDetectors(gcp.NewDetector()),
		resource.WithTelemetrySDK(),
		resource.WithFromEnv(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("my-service"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Configure exporter to use Application Default Credentials.
	// Set endpoint via OTEL_EXPORTER_OTLP_ENDPOINT=https://telemetry.googleapis.com
	// or programmatically via otlptracegrpc.WithEndpoint("telemetry.googleapis.com:443")
	exporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithEndpoint("telemetry.googleapis.com:443"),
		otlptracegrpc.WithDialOption(grpc.WithPerRPCCredentials(creds)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(tp)

	return func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("error shutting down trace provider: %v", err)
		}
	}, nil
}
```

### 3. Environment Variables

You can configure the exporter via standard OpenTelemetry environment variables:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=https://telemetry.googleapis.com
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
export OTEL_RESOURCE_ATTRIBUTES="gcp.project_id=<PROJECT_ID>"
```

> [!NOTE]
> Setting environment variables alone does not configure authentication credentials. When exporting directly to `telemetry.googleapis.com`, you must still configure Google Application Default Credentials (ADC) in your code using `grpc.WithPerRPCCredentials(creds)` as shown in the initialization example above.

### Configuration Mapping

The following table maps the configurations available in `exporter/trace` to their OTLP equivalents:

| `exporter/trace` Option | OTLP Equivalent Option / Env Var | Notes |
| :--- | :--- | :--- |
| `WithProjectID(id)` | Resource attribute `gcp.project_id` / `OTEL_RESOURCE_ATTRIBUTES` | Set resource attribute `gcp.project_id=<PROJECT_ID>`. |
| `WithDestinationProjectQuota()` | Header `x-goog-user-project` / `OTEL_EXPORTER_OTLP_HEADERS` | Set header `x-goog-user-project=<PROJECT_ID>`. |
| `WithTimeout(d)` | `otlptracegrpc.WithTimeout(d)` / `OTEL_EXPORTER_OTLP_TIMEOUT` | Exporter request timeout. |
| `WithTraceClientOptions(opts...)` | `otlptracegrpc.WithDialOption(...)` | Pass gRPC dial options directly to the OTLP exporter. |
| `WithErrorHandler(h)` | `otel.SetErrorHandler(h)` | Use standard OpenTelemetry error handler. |
| `WithAttributeMapping(m)` | N/A | Standard OpenTelemetry semantic conventions should be used directly. |
| `WithContext(ctx)` | `otlptracegrpc.New(ctx, ...)` | Pass context directly to `otlptracegrpc.New(ctx, ...)` and trace provider operations (`Shutdown(ctx)`). |

### Complete Sample

For a complete runnable sample, see the [Trace OTLP sample in opentelemetry-samples](https://github.com/GoogleCloudPlatform/opentelemetry-samples/tree/main/golang/trace).

---

## Migrate from Google Cloud Monitoring Exporter to OTLP Exporter

> [!WARNING]
> **Breaking Change Warning:** Migrating from the legacy Google Cloud Monitoring exporter to the standard OTLP exporter introduces breaking changes to your metric names.
>
> * **Legacy Exporter:** Ingests metrics under the `workload.googleapis.com/` domain (unless a custom prefix was configured).
> * **OTLP Exporter:** Ingests metrics under the `prometheus.googleapis.com/` domain by default via Google Managed Prometheus (GMP).
>
> Because of this domain change, your metric names in Cloud Monitoring will change. **This will affect existing dashboards and alerting policies** that query `workload.googleapis.com/` metric types.

### Why Migrate?

* **Standardization:** Aligns your application with the industry-standard OpenTelemetry Protocol (OTLP), ensuring vendor neutrality and compatibility with the broader OpenTelemetry ecosystem.
* **Google Managed Prometheus (GMP):** Standard OTLP metrics are ingested into Google Managed Prometheus, offering a scalable, cost-effective monitoring solution (~20x cheaper ingestion than the Cloud Monitoring custom metrics API).
* **Future-proofing:** The legacy exporter is deprecated and will be archived after January 1st, 2027.

---

### Migration Strategies

We recommend three paths for migration:

1. **Direct Migration (Recommended):** Migrate to the OTLP metric exporter and update dashboards/alerts to use `prometheus.googleapis.com/` metric names.
2. **Transition via Double-Writing (Alternative):** Register both the legacy exporter and the OTLP exporter on the `MeterProvider` to send metrics to both pipelines simultaneously during the transition period.
3. **Metric Views / Renaming (Alternative):** Use OpenTelemetry Views (`sdkmetric.WithView`) to customize metric names or attributes before export.

---

### Strategy 1: Direct Migration (Recommended)

#### 1. Update Dependencies

Add the standard OpenTelemetry OTLP metric exporter and the GCP resource detector:

```bash
go get go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc
go get go.opentelemetry.io/contrib/detectors/gcp
go get google.golang.org/grpc/credentials/oauth
```

#### 2. Configure the SDK

Initialize the `MeterProvider` with the OTLP exporter and GCP resource detector:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/oauth"
)

func initMeter(ctx context.Context) (func(), error) {
	creds, err := oauth.NewApplicationDefault(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load application default credentials: %w", err)
	}

	res, err := resource.New(
		ctx,
		// Detect GCP platform information
		resource.WithDetectors(gcp.NewDetector()),
		resource.WithTelemetrySDK(),
		resource.WithFromEnv(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("my-service"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	exporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint("telemetry.googleapis.com:443"),
		otlpmetricgrpc.WithDialOption(grpc.WithPerRPCCredentials(creds)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
	)
	otel.SetMeterProvider(meterProvider)

	return func() {
		if err := meterProvider.Shutdown(ctx); err != nil {
			log.Printf("error shutting down meter provider: %v", err)
		}
	}, nil
}
```

---

### Strategy 2: Transition via Double-Writing

To prevent gaps in monitoring dashboards while migrating, you can export metrics to both Google Cloud Monitoring (legacy) and the Telemetry API (OTLP) simultaneously:

```go
func initDoubleWritingMeter(ctx context.Context) (func(), error) {
	// Detect GCP platform resources as shown in Strategy #1
	res, err := resource.New(
		ctx,
		resource.WithDetectors(gcp.NewDetector()),
		resource.WithTelemetrySDK(),
		resource.WithFromEnv(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("my-service"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create legacy exporter (pass mexporter.WithProjectID if ADC lacks an embedded project ID)
	legacyExporter, err := mexporter.New()
	if err != nil {
		return nil, err
	}

	// Create OTLP exporter
	creds, err := oauth.NewApplicationDefault(ctx)
	if err != nil {
		return nil, err
	}
	otlpExporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint("telemetry.googleapis.com:443"),
		otlpmetricgrpc.WithDialOption(grpc.WithPerRPCCredentials(creds)),
	)
	if err != nil {
		return nil, err
	}

	// Register both readers on the same MeterProvider
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(legacyExporter)),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(otlpExporter)),
	)
	otel.SetMeterProvider(meterProvider)

	return func() {
		_ = meterProvider.Shutdown(ctx)
	}, nil
}
```

---

### Strategy 3: Custom Metric Prefixing / Metric Views

If you want to preserve legacy metric prefixes (such as `workload.googleapis.com/` or `custom.googleapis.com/`) during migration, you can use OpenTelemetry Views (`sdkmetric.WithView`) to prepend the prefix to metric names before export. Google Cloud's Telemetry API recognizes `workload.googleapis.com/` and `custom.googleapis.com/` prefixes in OTLP metric names and routes them directly to Cloud Monitoring custom metrics, preserving existing dashboards and alerts without converting them to `prometheus.googleapis.com/` metrics.

> [!NOTE]
> Metrics ingested under `workload.googleapis.com/` or `custom.googleapis.com/` are billed at Cloud Monitoring Custom Metric rates rather than Google Managed Prometheus rates.

```go
func initPrefixedMeter(ctx context.Context) (func(), error) {
	creds, err := oauth.NewApplicationDefault(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load application default credentials: %w", err)
	}

	res, err := resource.New(
		ctx,
		// Detect GCP platform information
		resource.WithDetectors(gcp.NewDetector()),
		resource.WithTelemetrySDK(),
		resource.WithFromEnv(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("my-service"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	exporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint("telemetry.googleapis.com:443"),
		otlpmetricgrpc.WithDialOption(grpc.WithPerRPCCredentials(creds)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	// Custom View to prepend "workload.googleapis.com/" to all metric instrument names
	prefixView := func(i sdkmetric.Instrument) (sdkmetric.Stream, bool) {
		return sdkmetric.Stream{
			Name:        "workload.googleapis.com/" + i.Name,
			Description: i.Description,
			Unit:        i.Unit,
		}, true
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithView(prefixView),
	)
	otel.SetMeterProvider(meterProvider)

	return func() {
		if err := meterProvider.Shutdown(ctx); err != nil {
			log.Printf("error shutting down meter provider: %v", err)
		}
	}, nil
}
```

---

### Mapping and Limitations

#### Configuration Mapping

The following table maps configurations available in `exporter/metric` to their OTLP equivalents:

| `exporter/metric` Option | OTLP Equivalent Option / Env Var | Notes |
| :--- | :--- | :--- |
| `WithProjectID(id)` | Resource attribute `gcp.project_id` | Set via `resource.WithAttributes` or `OTEL_RESOURCE_ATTRIBUTES`. |
| `WithDestinationProjectQuota()` | Header `x-goog-user-project` | Set via `otlpmetricgrpc.WithHeaders` or `OTEL_EXPORTER_OTLP_HEADERS`. |
| `WithMonitoringClient(cl)` | N/A | Pre-configured `MetricClient` cannot be passed directly to `otlpmetricgrpc.New`. Use `otlpmetricgrpc.WithDialOption` for custom gRPC client configuration. |
| `WithMonitoringClientOptions(opts...)` | `otlpmetricgrpc.WithDialOption(...)` | Pass gRPC dial options directly to the OTLP exporter. |
| `WithCompression("gzip")` | `otlpmetricgrpc.WithCompressor("gzip")` / `OTEL_EXPORTER_OTLP_COMPRESSION` | Configure compression on the exporter. |
| `WithTimeout(t)` | `otlpmetricgrpc.WithTimeout(t)` / `OTEL_EXPORTER_OTLP_TIMEOUT` | Exporter request timeout. |
| `WithFilteredResourceAttributes(f)` | OpenTelemetry Views / Resource configuration | Filter resource attributes using custom views or resource options. |
| `WithMetricDescriptorTypeFormatter(f)` | OpenTelemetry Views (`sdkmetric.WithView`) / N/A | Telemetry API handles metric naming automatically under `prometheus.googleapis.com/`. Use Views if custom prefixing is required. |
| `WithDisableCreateMetricDescriptors()` | N/A | Telemetry API creates descriptors dynamically as needed. |
| `WithSumOfSquaredDeviation()` | N/A | OpenTelemetry histograms do not calculate or export estimated sum of squared deviation; Telemetry API handles distributions natively. |
| `WithCreateServiceTimeSeries()` | N/A | Specific to Google-internal usage of the exporter. |
| `WithMonitoredResourceDescription(...)` | N/A | OTel uses standard OTel resource attributes mapped automatically by GCP. |
| `WithContext(ctx)` | `otlpmetricgrpc.New(ctx, ...)` | Pass context directly to `otlpmetricgrpc.New(ctx, ...)` and meter provider operations (`Shutdown(ctx)`). |

#### Complete Sample

For a complete runnable sample, see the [Metric OTLP sample in opentelemetry-samples](https://github.com/GoogleCloudPlatform/opentelemetry-samples/tree/main/golang/metric).
