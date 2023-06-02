# Collector Exporter Integration Tests

The `googlecloud` exporter has fixture based integration tests which can verify that telemetry
can be written to the real GCP APIs. Separately, the tests also mock the real APIs to capture
and compare requests against recorded expectation fixtures. You can write tests for metrics,
traces, and logs.

<a id="testcasefiles"></a>The fixtures are located in the
[`integrationtest/testdata/fixtures`](integrationtest/testdata/fixtures) directories and
registered in signal dependent test case files:

- [`integrationtest/testcases/testcases_metrics.go`](integrationtest/testcases/testcases_metrics.go).
- [`integrationtest/testcases/testcases_traces.go`](integrationtest/testcases/testcases_traces.go).
- [`integrationtest/testcases/testcases_logs.go`](integrationtest/testcases/testcases_logs.go).

## Running Tests

The actual Go test files are in the `integrationtest/` directory, e.g. for metrics
[`metrics_test.go`](integrationtest/metrics_test.go) (use a mocked server to compare the GCP
requests against recorded fixtures) and
[`metrics_integration_test.go`](integrationtest/metrics_integration_test.go) (send the actual
fixtures to GCP and expect OK responses).

### Expectation Only

Nothing special here, these run along with all the other go tests:

```sh
# from repo root
make test

# OR from this directory
go test
```

### Sending Requests to GCP

For one-off contributions, it's easiest to create a PR and see the integration test results
there.

Since these require a GCP project and credentials, they do not run by default. You need to set
the build tag `integrationtest` to enable building them. The tests will set the exporter's
project ID from the `PROJECT_ID` environment variable, or use [application default
credentials](https://cloud.google.com/docs/authentication/production#automatically).

```sh
# Only needed if you want to target a specific project instead of ADC
export PROJECT_ID="foo"

# Set SECOND_PROJECT_ID to test multi-project export.
export SECOND_PROJECT_ID="bar"

# from repo root
make integrationtest

# OR from this directory
go test -tags=integrationtest

# OR optionally, run just integration tests
go test -tags=integrationtest -run=TestIntegration
```

## Adding New Tests

To add a new test, using metrics as an example:

1. Create an OTLP fixture. The test fixtures should be JSON encoded OTLP messages of type:
    - [`ExportMetricsServiceRequest`](https://github.com/open-telemetry/opentelemetry-proto/blob/v0.19.0/opentelemetry/proto/collector/metrics/v1/metrics_service.proto#L36) for metrics
    - [`ExportTraceServiceRequest`](https://github.com/open-telemetry/opentelemetry-proto/blob/v0.19.0/opentelemetry/proto/collector/trace/v1/trace_service.proto#L36) for traces.
    - [`ExportLogsServiceRequest`](https://github.com/open-telemetry/opentelemetry-proto/blob/v0.19.0/opentelemetry/proto/collector/logs/v1/logs_service.proto#L36) for logs.
    
    Put the fixture in the signal specific
    [`testdata/fixtures`](integrationtest/testdata/fixtures) directory. As an example, see
    [`testdata/fixtures/basic_counter_metrics.json`](integrationtest/testdata/fixtures/metrics/basic_counter_metrics.json).

    One easy way to generate these fixtures from a live system is by using the collector's
    `file` exporter in a collector pipeline to dump OTLP. For example, update the collector
    config to:
  
    ```diff
    exporters:
      googlecloud:
    + file:
    +   path: ./otlp-output.json
    # ...
    service:
      pipelines:
        metrics/agent:
          exporters:
          - googlecloud
    +     - file
    ```

    This is how the Ops Agent fixtures were generated.

1. Add an entry in the corresponding [testcase file](#testcasefiles). For metrics, that is [`integrationtest/testcases/testcases_metrics.go`][testcases].
1. Run the script to record the expectation fixture. This will contain the expected requests
    that the exporter makes to GCP services:

    ```sh
    cd integrationtest
    go run cmd/recordfixtures/main.go
    ```
    
    or from the repo root:
    ```sh
    make fixtures
    ```

    The generated file is a JSON encoded
    [`MetricExpectFixture`](integrationtest/protos/fixtures.proto#L23) protobuf message.

See [#229](https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/pull/229) for an
example PR.

## Continuous Integration

The integration tests are run on Cloud Build in a real GCP project. You will see a CI job on
PRs called `ops-go-integration-tests (opentelemetry-ops-e2e)`. The Cloud Build config is in
[`cloudbuild-integration-tests.yaml`](/cloudbuild-integration-tests.yaml).

Unfortunately, Cloud Build only allows people with access to our GCP project to see build logs.
If you need to see the build logs because tests are failing, just ask someone to share them
with you.

[fixtures]: testdata/fixtures
[testcases]: integrationtest/testcases.go
