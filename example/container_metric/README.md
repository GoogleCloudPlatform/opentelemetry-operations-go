# GKE Container metric example

This example is to test writing to the k8s_container resource.  This is
normally accomplished using the [gke resource detector](https://github.com/open-telemetry/opentelemetry-go-contrib/blob/main/detectors/gcp/gke.go),
but this example allows writing to container metrics from a local development
setup.

You should not set resource attributes this way in non-dev setups.
