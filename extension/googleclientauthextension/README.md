# Authenticator - Google Client Credentials

| Status        |           |
| ------------- |-----------|
| Stability     | [alpha]  |

[alpha]: https://github.com/open-telemetry/opentelemetry-collector#alpha

This extension provides Google OAuth2 Client Credentials and Metadata for gRPC and http based exporters.

The authenticator type has to be set to `googleclientauth`.

## Configuration

```yaml
extensions:
  googleclientauth:
    
receivers:
  otlp:
    protocols:
      grpc:

exporters:      
  otlp/withauth:
    endpoint: 0.0.0.0:5000
    ca_file: /tmp/certs/ca.pem
    auth:
      authenticator: googleclientauth

service:
  extensions: [googleclientauth]
  pipelines:
    metrics:
      receivers: [otlp]
      processors: []
      exporters: [otlp/withauth]
```

Following are the configuration fields:
- **project** - The Google Cloud Project telemetry is sent to if the gcp.project.id resource attribute is not set. If unspecified, this is determined using application default credentials.
- [**scopes**](https://datatracker.ietf.org/doc/html/rfc6749#section-3.3) - The resource server's token endpoint URLs.
- [**quota_project**](https://cloud.google.com/apis/docs/system-parameters) - The project for quota and billing purposes. The caller must have serviceusage.services.use permission on the project.

## Building into a Collector

This extension is not included in any collector distributions today. To build a collector with this component, add the following to your collector builder configuration:

```yaml
extensions:
  - import: github.com/GoogleCloudPlatform/opentelemetry-operations-go/extension/googleclientauthextension
    gomod: github.com/GoogleCloudPlatform/opentelemetry-operations-go/extension/googleclientauthextension v0.43.0
```
