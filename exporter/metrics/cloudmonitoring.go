// Copyright 2018, OpenCensus Authors
//           2020, Google Cloud Operations Exporter Authors
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

package metrics

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	metadataapi "cloud.google.com/go/compute/metadata"
	monitoringapi "cloud.google.com/go/monitoring/apiv3"
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource"
	"go.opencensus.io/resource"
	"go.opencensus.io/stats/view"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

// Option is function type that is passed to the exporter initialization function.
type Option func(*options)

// options contains options for configuring the exporter.
type options struct {
	// ProjectID is the identifier of the Stackdriver
	// project the user is uploading the stats data to.
	// If not set, this will default to your "Application Default Credentials".
	// For details see: https://developers.google.com/accounts/docs/application-default-credentials.
	//
	// It will be used in the project_id label of a Stackdriver monitored
	// resource if the resource does not inherently belong to a specific
	// project, e.g. on-premise resource like k8s_container or generic_task.
	ProjectID string

	// Location is the identifier of the GCP or AWS cloud region/zone in which
	// the data for a resource is stored.
	// If not set, it will default to the location provided by the metadata server.
	//
	// It will be used in the location label of a Stackdriver monitored resource
	// if the resource does not inherently belong to a specific project, e.g.
	// on-premise resource like k8s_container or generic_task.
	Location string

	// OnError is the hook to be called when there is
	// an error uploading the stats or tracing data.
	// If no custom hook is set, errors are logged.
	// Optional.
	OnError func(err error)

	// MonitoringClientOptions are additional options to be passed
	// to the underlying Stackdriver Monitoring API client.
	// Optional.
	MonitoringClientOptions []option.ClientOption

	// Resource sets the MonitoredResource against which all views will be
	// recorded by this exporter.
	//
	// All Stackdriver metrics created by this exporter are custom metrics,
	// so only a limited number of MonitoredResource types are supported, see:
	// https://cloud.google.com/monitoring/custom-metrics/creating-metrics#which-resource
	//
	// An important consideration when setting the Resource here is that
	// Stackdriver Monitoring only allows a single writer per
	// TimeSeries, see: https://cloud.google.com/monitoring/api/v3/metrics-details#intro-time-series
	// A TimeSeries is uniquely defined by the metric type name
	// (constructed from the view name and the MetricPrefix), the Resource field,
	// and the set of label key/value pairs (in OpenCensus terminology: tag).
	//
	// If no custom Resource is set, a default MonitoredResource
	// with type global and no resource labels will be used. If you explicitly
	// set this field, you may also want to set custom DefaultMonitoringLabels.
	//
	// Deprecated: Use MonitoredResource instead.
	Resource *monitoredrespb.MonitoredResource

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
	MonitoredResource monitoredresource.Interface

	// MapResource converts a OpenTelemetry resource to a Google Cloud Monitored resource.
	//
	// If this field is unset, defaultMapResource will be used which encodes a set of default
	// conversions from auto-detected resources to well-known Google Cloud Monitoring monitored resources.
	MapResource func(*resource.Resource) *monitoredrespb.MonitoredResource

	// MetricPrefix overrides the prefix of a Stackdriver metric names.
	// Optional. If unset defaults to "custom.googleapis.com/opencensus/".
	// If GetMetricPrefix is non-nil, this option is ignored.
	MetricPrefix string

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

	// GetMetricDisplayName allows customizing the display name for the metric
	// associated with the given view. By default it will be:
	//   MetricPrefix + view.Name
	GetMetricDisplayName func(view *view.View) string

	// GetMetricType allows customizing the metric type for the given view.
	// By default, it will be:
	//   "custom.googleapis.com/opencensus/" + view.Name
	//
	// See: https://cloud.google.com/monitoring/api/ref_v3/rest/v3/projects.metricDescriptors#MetricDescriptor
	// Depreacted. Use GetMetricPrefix instead.
	GetMetricType func(view *view.View) string

	// GetMetricPrefix allows customizing the metric prefix for the given metric name.
	// If it is not set, MetricPrefix is used. If MetricPrefix is not set, it defaults to:
	//   "custom.googleapis.com/opencensus/"
	//
	// See: https://cloud.google.com/monitoring/api/ref_v3/rest/v3/projects.metricDescriptors#MetricDescriptor
	GetMetricPrefix func(name string) string

	// DefaultMonitoringLabels are labels added to every metric created by this
	// exporter in Stackdriver Monitoring.
	//
	// If unset, this defaults to a single label with key "opencensus_task" and
	// value "go-<pid>@<hostname>". This default ensures that the set of labels
	// together with the default Resource (global) are unique to this
	// process, as required by Stackdriver Monitoring.
	//
	// If you set DefaultMonitoringLabels, make sure that the Resource field
	// together with these labels is unique to the
	// current process. This is to ensure that there is only a single writer to
	// each TimeSeries in Stackdriver.
	//
	// Set this to &Labels{} (a pointer to an empty Labels) to avoid getting the
	// default "opencensus_task" label. You should only do this if you know that
	// the Resource you set uniquely identifies this Go process.
	DefaultMonitoringLabels *Labels

	// SkipCMD enforces to skip all the CreateMetricDescriptor calls.
	// These calls are important in order to configure the unit of the metrics,
	// but in some cases all the exported metrics are builtin (unit is configured)
	// or the unit is not important.
	SkipCMD bool

	// Timeout for all API calls. If not set, defaults to 5 seconds.
	Timeout time.Duration

	// ReportingInterval sets the interval between reporting metrics.
	// If it is set to zero then default value is used.
	ReportingInterval time.Duration

	// NumberOfWorkers sets the number of go rountines that send requests
	// to Stackdriver Monitoring. This is only used for Proto metrics export
	// for now. The minimum number of workers is 1.
	NumberOfWorkers int
}

// WithTimeout sets the timeout for metric exporter to call backend API.
func WithTimeout(t time.Duration) func(o *options) {
	return func(o *options) {
		o.Timeout = t
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

func (o *options) handleError(err error) {
	if o.OnError != nil {
		o.OnError(err)
		return
	}
	log.Printf("Failed to export to Google Cloud Monitoring: %v", err)
}

const (
	// defaultTimeout is used as default when timeout is not set in newContextWithTimout.
	defaultTimeout = 5 * time.Second

	// defaultReportingInterval defaults to 60 seconds.
	// https://cloud.google.com/monitoring/custom-metrics/creating-metrics#monitoring_write_timeseries-go
	defaultReportingInterval = 60 * time.Second
)

// Exporter is a metrics exporter that uploads data to Google Cloud Monitoring.
type Exporter struct {
	metricsExporter *metricsExporter
}

// NewRawExporter creates a new Exporter thats implements trace.Exporter.
func NewRawExporter(opts ...Option) (*Exporter, error) {
	o := options{Context: context.Background()}
	for _, opt := range opts {
		opt(&o)
	}
	if o.ProjectID == "" {
		creds, err := google.FindDefaultCredentials(o.Context, monitoringapi.DefaultAuthScopes()...)
		if err != nil {
			return nil, fmt.Errorf("Google Cloud Monitoring: %v", err)
		}
		if creds.ProjectID == "" {
			return nil, errors.New("Google Cloud Monitoring: no project found with application default credentials")
		}
		o.ProjectID = creds.ProjectID
	}
	if o.Location == "" {
		if metadataapi.OnGCE() {
			zone, err := metadataapi.Zone()
			if err != nil {
				err = fmt.Errorf("setting Cloud Monitoring default location failed: %v", err)
				if o.OnError != nil {
					o.OnError(err)
				} else {
					log.Print(err)
				}
			} else {
				o.Location = zone
			}
		}
	}
	if o.MonitoredResource != nil {
		o.Resource = convertMonitoredResourceToPB(o.MonitoredResource)
	}
	if o.MapResource == nil {
		o.MapResource = defaultMapResource
	}
	if o.Timeout == 0 {
		o.Timeout = defaultTimeout
	}
	if o.ReportingInterval == 0 {
		o.ReportingInterval = defaultReportingInterval
	}
	me, err := newMetricsExporter(&o)
	if err != nil {
		return nil, err
	}
	return &Exporter{
		metricsExporter: me,
	}, nil
}

// convertMonitoredResourceToPB converts MonitoredResource data in to
// protocol buffer.
func convertMonitoredResourceToPB(mr monitoredresource.Interface) *monitoredrespb.MonitoredResource {
	mrpb := new(monitoredrespb.MonitoredResource)
	var labels map[string]string
	mrpb.Type, labels = mr.MonitoredResource()
	mrpb.Labels = make(map[string]string)
	for k, v := range labels {
		mrpb.Labels[k] = v
	}
	return mrpb
}

// Export exports the provide metric record to Google Cloud Monitoring.
func (e *Exporter) Export(_ context.Context, checkpointSet export.CheckpointSet) error {
	// TODO(ymotongpoo): implement here
	return nil
}
