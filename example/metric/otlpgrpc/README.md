# OTLP Metric with Google Auth Example

Run this sample to connect to an endpoint that is protected by GCP authentication.

#### Prerequisites

Get Google credentials on your machine:

```sh
gcloud auth application-default login
```

#### Run the Sample

```sh
# export necessary OTEL environment variables
export PROJECT_ID=<project-id>
export OTEL_EXPORTER_OTLP_ENDPOINT=<endpoint>
export OTEL_RESOURCE_ATTRIBUTES="gcp.project_id=$PROJECT_ID,service.name=otlp-sample,service.instance.id=1"
export OTEL_EXPORTER_OTLP_HEADERS=X-Goog-User-Project=$PROJECT_ID

# from the samples/otlpmetric repository
cd example/metric/otlpgrpc && go run .
```
