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
	"google.golang.org/api/option"
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
	MonitoringClientOptions []option.ClientOption

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
}

// WithInterval sets the interval for metric exporter to send data to Cloud Monitoring.
// t should not be less than 10 seconds.
// c.f. https://cloud.google.com/monitoring/docs/release-notes#March_30_2020
func WithInterval(t time.Duration) func(o *options) {
	return func(o *options) {
		o.ReportingInterval = t
	}
}
