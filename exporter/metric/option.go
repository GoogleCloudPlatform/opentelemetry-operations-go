// Copyright 2020, Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metric

import (
	"context"
	"fmt"
	"time"

	apimetric "go.opentelemetry.io/otel/api/metric"
	otel "go.opentelemetry.io/otel/sdk"

	apioption "google.golang.org/api/option"
)

var userAgent = fmt.Sprintf("opentelemetry-go %s; metric-exporter %s", otel.Version(), Version())

// Option is function type that is passed to the exporter initialization function.
type Option func(*options)

// options is the struct to hold options for metricExporter and its client instance.
type options struct {
	// ProjectID is the identifier of the Cloud Monitoring
	// project the user is uploading the stats data to.
	// If not set, this will default to your "Application Default Credentials".
	// For details see: https://developers.google.com/accounts/docs/application-default-credentials.
	//
	// It will be used in the project_id label of a Google Cloud Monitoring monitored
	// resource if the resource does not inherently belong to a specific
	// project, e.g. on-premise resource like k8s_container or generic_task.
	ProjectID string

	// MonitoringClientOptions are additional options to be passed
	// to the underlying Stackdriver Monitoring API client.
	// Optional.
	MonitoringClientOptions []apioption.ClientOption

	// Context allows you to provide a custom context for API calls.
	//
	// This context will be used several times: first, to create Cloud Monitoring
	// clients, and then every time a new batch of metrics needs to be uploaded.
	//
	// If unset, context.Background() will be used.
	Context context.Context

	// ReportingInterval sets the interval between reporting metrics.
	// If it is set to zero then default value is used.
	ReportingInterval time.Duration

	// MetricDescriptorTypeFormatter is the custom formtter for the MetricDescriptor.Type.
	// By default, the format string is "custom.googleapis.com/opentelemetry/[metric name]".
	MetricDescriptorTypeFormatter func(*apimetric.Descriptor) string

	// onError is the hook to be called when there is an error uploading the metric data.
	// If no custom hook is set, errors are logged. Optional.
	//
	// TODO: This option should be replaced with OTel defining error handler.
	// c.f. https://pkg.go.dev/go.opentelemetry.io/otel@v0.6.0/sdk/metric/controller/push?tab=doc#Config
	onError func(error)
}

// WithProjectID sets Google Cloud Platform project as projectID.
// Without using this option, it automatically detects the project ID
// from the default credential detection process.
// Please find the detailed order of the default credentail detection proecess on the doc:
// https://godoc.org/golang.org/x/oauth2/google#FindDefaultCredentials
func WithProjectID(id string) func(o *options) {
	return func(o *options) {
		o.ProjectID = id
	}
}

// WithMonitoringClientOptions add the options for Cloud Monitoring client instance.
// Available options are defined in
func WithMonitoringClientOptions(opts ...apioption.ClientOption) func(o *options) {
	return func(o *options) {
		o.MonitoringClientOptions = append(o.MonitoringClientOptions, opts...)
	}
}

// WithInterval sets the interval for metric exporter to send data to Cloud Monitoring.
// t should not be less than 10 seconds.
// c.f. https://cloud.google.com/monitoring/docs/release-notes#March_30_2020
func WithInterval(t time.Duration) func(o *options) {
	return func(o *options) {
		o.ReportingInterval = t
	}
}

// WithMetricDescriptorTypeFormatter sets the custom formatter for MetricDescriptor.
// Note that the format has to follow the convention defined in the official document.
// The default is "custom.googleapis.com/[metric name]".
// ref. https://cloud.google.com/monitoring/custom-metrics/creating-metrics#custom_metric_names
func WithMetricDescriptorTypeFormatter(f func(*apimetric.Descriptor) string) func(o *options) {
	return func(o *options) {
		o.MetricDescriptorTypeFormatter = f
	}
}

// WithOnError sets the custom error handler to be called on errors.
func WithOnError(f func(error)) func(o *options) {
	return func(o *options) {
		o.onError = f
	}
}
