package exporter
import (
	"context"
	"time"

	sdk "go.opentelemetry.io/otel/sdk/metric"
	apimetric "go.opentelemetry.io/otel/api/metric"
	texport "go.opentelemetry.io/otel/sdk/export/trace"	
	cloudtrace "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	cloudmetric "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric"
	controllerTime "go.opentelemetry.io/otel/sdk/metric/controller/time"

	"google.golang.org/api/option"
	"go.opentelemetry.io/otel/sdk/metric/controller/push"
)

var (
	monitoredResLabelMap = make(map[string]string)
)

// Options contains options for configuring the exporter.
type Options struct {
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
	MonitoringClientOptions option.ClientOption

	// TraceClientOptions are additional options to be passed
	// to the underlying Stackdriver Trace API client.
	// Optional.
	TraceClientOptions []option.ClientOption

	// TraceSpansBufferMaxBytes is the maximum size (in bytes) of spans that
	// will be buffered in memory before being dropped.
	//
	// If unset, a default of 8MB will be used.
	// TraceSpansBufferMaxBytes int

	// DefaultTraceAttributes will be appended to every span that is exported to
	// Stackdriver Trace.
	DefaultTraceAttributes map[string]interface{}

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

	// MetricDescriptorTypeFormatter is the custom formtter for the MetricDescriptor.Type.
	// By default, the format string is "custom.googleapis.com/opentelemetry/[metric name]".
	MetricDescriptorTypeFormatter func(*apimetric.Descriptor) string
}


// Exporter is a trace and metric exporter
type Exporter struct {
	traceExporter *cloudtrace.Exporter
	metricPusher *push.Controller
}

// extractResourceLabels extracts resources from the construction option
func extractResourceLabels(popts ...push.Option) {
	for _, popt := range popts {
		var config push.Config
		popt.Apply(&config)
		if config.Resource.Len() > 0 {
			for _, ele := range config.Resource.Attributes() {
				monitoredResLabelMap[string(ele.Key)] = ele.Value.AsString()
			}
		}
	}
}

// NewExporter creates a new Exporter that implements both trace.Exporter
// and metric.Exporter
func NewExporter(o Options, popts... push.Option) (*Exporter, error) {

	te, err := cloudtrace.NewExporter(cloudtrace.WithProjectID(o.ProjectID), cloudtrace.WithContext(o.Context),
		cloudtrace.WithTraceClientOptions(o.TraceClientOptions), cloudtrace.WithTimeout(o.Timeout), 
		cloudtrace.WithOnError(o.OnError),
	)
	if err != nil {
		return nil, err
	} 

	extractResourceLabels(popts...)

	pusher, err := cloudmetric.InstallNewPipeline(
		[]cloudmetric.Option{		
			cloudmetric.WithProjectID(o.ProjectID), 
			cloudmetric.WithInterval(o.Timeout), cloudmetric.WithOnError(o.OnError),		
			cloudmetric.WithMetricDescriptorTypeFormatter(o.MetricDescriptorTypeFormatter),
			// cloudmetric.WithMonitoringClientOptions(o.MonitoringClientOptions), 
		}, popts...,
	)
	if err != nil {
		return nil, err
	} 


	return &Exporter{
		traceExporter: te,
		metricPusher: pusher,
	}, nil
}


// ExportSpan exports a SpanData to Stackdriver Trace.
func (e *Exporter) ExportSpan(ctx context.Context, sd *texport.SpanData) {
	e.traceExporter.ExportSpan(ctx, sd)
}

// ExportSpans exports a slice of SpanData to Stackdriver Trace in batch
func (e *Exporter) ExportSpans(ctx context.Context, sds []*texport.SpanData) {
	e.traceExporter.ExportSpans(ctx, sds)
}

// GetTraceExporter returns the traceExporter
func (e *Exporter) GetTraceExporter() *cloudtrace.Exporter {
	return e.traceExporter;
}

// GetMetricPusher returns the metricPusher
func (e *Exporter) GetMetricPusher() *push.Controller {
	return e.metricPusher;
}

// SetClock supports setting a mock clock for testing.  This must be
// called before Start().
func (e *Exporter) SetClock(clock controllerTime.Clock) {
	e.metricPusher.SetClock(clock)
}

// SetErrorHandler sets the handler for errors.  If none has been set, the
// SDK default error handler is used.
func (e *Exporter) SetErrorHandler(errorHandler sdk.ErrorHandler) {
	e.metricPusher.SetErrorHandler(errorHandler)
}

// Provider returns a metric.Provider instance for this controller.
func (e *Exporter) Provider() apimetric.Provider {
	return e.metricPusher.Provider()
}

// Start begins a ticker that periodically collects and exports
// metrics with the configured interval.
func (e *Exporter) Start() {
	e.metricPusher.Start()
}

// Stop waits for the background goroutine to return and then collects
// and exports metrics one last time before returning.
func (e *Exporter) Stop() {
	e.metricPusher.Stop()
}
