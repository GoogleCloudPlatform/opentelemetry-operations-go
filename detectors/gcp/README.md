# GCP Resource detector

Note: This is still a work in progress. Use the detector in opentelemetry-go-contrib for now: https://github.com/open-telemetry/opentelemetry-go-contrib/tree/main/detectors/gcp.

## Usage

```golang
    ctx := context.Background()
    // detect your resources
    resource, err := resource.New(ctx,
        resource.WithDetectors(
            // Use this GCP resource detector!
            gcp.NewDetector(), 
            // Add other, default resource detectors.
            resource.WithFromEnv(), 
            resource.WithTelemetrySDK(),
        ),
        // Add your own custom attributes to identify your application
        resource.WithAttributes(
            semconv.ServiceNameKey.String("my-application"),
            semconv.ServiceNamespaceKey.String("my-company-frontend-team"),
        ),
    )
    // use the resource in your tracerprovider (or meterprovider)
	tp := trace.NewTracerProvider(
        // ... other options
		trace.WithResource(resource),
	)
```

## Setting Kubernetes attributes

Previous iterations of GCP resource detection attempted to detect
container.name, k8s.pod.name and k8s.namespace.name.  When using this detector,
you should use this in your Pod Spec to set these using
OTEL_RESOURCE_ATTRIBUTES:

```yaml
env:
- name: POD_NAME
   valueFrom:
     fieldRef:
       fieldPath: metadata.name
- name: NAMESPACE_NAME
  valueFrom:
    fieldRef:
      fieldPath: metadata.namespace
- name: CONTAINER_NAME
   value: 
- name: OTEL_RESOURCE_ATTRIBUTES
  value: k8s.pod.name=$(POD_NAME),k8s.namespace.name=$(NAMESPACE_NAME),k8s.container.name=$(CONTAINER_NAME)
```