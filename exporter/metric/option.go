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
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

var (
	userAgent = fmt.Sprintf("opentelemetry-go %s; metric-exporter %s", otel.Version(), version)
)

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
	// This context will be used several times: first, to create Stackdriver
	// trace and metric clients, and then every time a new batch of traces or
	// stats needs to be uploaded.
	//
	// Do not set a timeout on this context. Instead, set the Timeout option.
	//
	// If unset, context.Background() will be used.
	Context context.Context

	// ReportingInterval sets the interval between reporting metrics.
	// If it is set to zero then default value is used.
	ReportingInterval time.Duration

	// MetricDescriptorTypeFormatter is the custom formtter for the MetricDescriptor.Type.
	// By default, the format string is "custom.googleapis.com/opentelemetry/[metric name]".
	MetricDescriptorTypeFormatter func(*apimetric.Descriptor) string

	// MonitoredResource sets the MonitoredResource against which all views will be
	// recorded by this exporter.
	//
	// All Stackdriver metrics created by this exporter are custom metrics,
	// so only a limited number of MonitoredResource types are supported, see:
	// https://cloud.google.com/monitoring/custom-metrics/creating-metrics#which-resource
	//
	// An important consideration when setting the MonitoredResource here is that
	// Stackdriver Monitoring only allows a single writer per
	// TimeSeries, see: https://cloud.google.com/monitoring/api/v3/metrics-details#intro-time-series
	// A TimeSeries is uniquely defined by the metric type name
	// (constructed from the view name and the MetricPrefix), the MonitoredResource field,
	// and the set of label key/value pairs (in OpenCensus terminology: tag).
	//
	// If no custom MonitoredResource is set AND if Resource is also not set then
	// a default MonitoredResource with type global and no resource labels will be used.
	// If you explicitly set this field, you may also want to set custom DefaultMonitoringLabels.
	//
	// This field replaces Resource field. If this is set then it will override the
	// Resource field.
	// Optional, but encouraged.
	//
	// NOTE: [ymotongpoo] Leave this field with primitive type, but this should have
	// a similar abstract interface as OpenCensus does.
	// c.f. https://github.com/census-ecosystem/opencensus-go-exporter-stackdriver/blob/e191b7c50f/monitoredresource
	MonitoredResource *monitoredrespb.MonitoredResource
}

// WithMoniroingClientOptions add the options for Cloud Monitoring client instance.
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
// The default is "custom.googleapi.com/[metric name]".
// ref. https://cloud.google.com/monitoring/custom-metrics/creating-metrics#custom_metric_names
func WithMetricDescriptorTypeFormatter(f func(*apimetric.Descriptor) string) func(o *options) {
	return func(o *options) {
		o.MetricDescriptorTypeFormatter = f
	}
}

// WithMoniWithMonitoredResource sets the custom MonitoredResoruce.
// Note that the resource name should follow the convention defined in the official document.
// The default is "projects/[PROJECT_ID]/metridDescriptors/[METRIC_TYPE]".
// ref. https://cloud.google.com/monitoring/custom-metrics/creating-metrics#custom_metric_names
func WithMonitoredResource(mr *monitoredrespb.MonitoredResource) func(o *options) {
	return func(o *options) {
		o.MonitoredResource = mr
	}
}
