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
- [**scopes**](https://datatracker.ietf.org/doc/html/rfc6749#section-3.3) - The oauth 2.0 scopes requested by the extension.
- [**quota_project**](https://cloud.google.com/apis/docs/system-parameters) - The project for quota and billing purposes. The caller must have serviceusage.services.use permission on the project.
- **token_type** - The type of generated token. Default: `access_token`
  - `access_token`: [OAuth 2.0 access token](https://cloud.google.com/docs/authentication/token-types#access) will be generated.
  - `id_token`: Google-signed [ID token](https://cloud.google.com/docs/authentication/token-types#id) will be generated.
- **audience** - The audience claim used for generating ID token
