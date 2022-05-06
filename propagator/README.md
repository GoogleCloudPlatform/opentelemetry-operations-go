# OpenTelemetry Google Cloud Trace Propagators

[![Docs](https://godoc.org/github.com/GoogleCloudPlatform/opentelemetry-operations-go/propagator?status.svg)](https://pkg.go.dev/github.com/GoogleCloudPlatform/opentelemetry-operations-go/propagator)

This package contains Trace Context Propagators for use with [Google Cloud
Trace](https://cloud.google.com/trace) that make it compatible with
[OpenTelemetry](http://opentelemetry.io). 

There are two available propagators in this package:

### `CloudTraceOneWayPropagator` (Recommended)

The `CloudTraceOneWayPropagator` reads the `X-Cloud-Trace-Context` header for trace and
span IDs, but does not write the `X-Cloud-Trace-Context` header into outgoing
requests.

This is useful for ensuring spans created in your code are attached to the
traces that some Google Cloud services [automatically trace](https://cloud.google.com/trace/docs/overview#configurations_with_automatic_tracing).

#### Usage

```golang
import (
    "go.opentelemetry.io/otel/propagation"
    gcppropagator "github.com/GoogleCloudPlatform/opentelemetry-operations-go/propagator"
)

func installPropagators() {
    otel.SetTextMapPropagator(
        propagation.NewCompositeTextMapPropagator(
            // Putting the CloudTraceOneWayPropagator first means the TraceContext propagator 
            // takes precedence if both the traceparent and the XCTC headers exist.
            gcppropagator.CloudTraceOneWayPropagator{},
            propagation.TraceContext{},
            propagation.Baggage{},
        ))
}
```

### `CloudTraceFormatPropagator`

The standard propagator reads and writes the `X-Cloud-Trace-Context` header.
Note that because of differences between the meaning of the `sampled` flag
(described below), this can result in 100% tracing when the parent context
has a deferred tracing decision.

#### Usage

```golang
import (
    "go.opentelemetry.io/otel/propagation"
    gcppropagator "github.com/GoogleCloudPlatform/opentelemetry-operations-go/propagator"
)

func installPropagators() {
    otel.SetTextMapPropagator(
        propagation.NewCompositeTextMapPropagator(
            // Putting the CloudTraceFormatPropagator first means the TraceContext propagator 
            // takes precedence if both the traceparent and the XCTC headers exist.
            gcppropagator.CloudTraceFormatPropagator{},
            propagation.TraceContext{},
            propagation.Baggage{},
        ))
}
```

## Differences between Google Cloud Trace and W3C Trace Context

Google Cloud Trace encodes trace information in the `X-Cloud-Trace-Context` HTTP
header, using the format described in the [Trace documentation](https://cloud.google.com/trace/docs/setup#force-trace).

OpenTelemetry uses the newer, W3C standard
[`traceparent` header](https://www.w3.org/TR/trace-context/#traceparent-header)

There is an important semantic difference between Cloud Trace's
`TRACE_TRUE` flag, and W3C's `sampled` flag.

As outlined in the [Trace
documentation](https://cloud.google.com/trace/docs/setup#force-trace), setting
the `TRACE_TRUE` flag will cause trace information to be collected.

This differs from the W3C behavior, where the [`sampled`
flag](https://www.w3.org/TR/trace-context/#sampled-flag) indicates that the
caller *may* have recorded trace information, but does not necessarily impact
the sampling done by other services.

To preserve the Cloud-Trace behavior when using `traceparent`, you can use the
[`ParentBased`
sampler](https://pkg.go.dev/go.opentelemetry.io/otel/sdk/trace#ParentBased) like
so:

```go
import sdktrace go.opentelemetry.io/otel/sdk/trace
sampler := sdktrace.ParentBased(
    sdktrace.NeverSample(),
    sdktrace.WithRemoteParentSampled(sdktrace.AlwaysSample()))
)
```