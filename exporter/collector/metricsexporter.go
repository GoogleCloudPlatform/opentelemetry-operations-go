// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file contains the rewritten googlecloud metrics exporter which no longer takes
// dependency on the OpenCensus stackdriver exporter.

package collector

import (
	"context"
	"strings"
	"unicode"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
	"google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

// metricsExporter is the GCM exporter that uses pdata directly
type metricsExporter struct {
	cfg    *Config
	client *monitoring.MetricClient
}

type labels map[string]string

func (me *metricsExporter) Shutdown(context.Context) error {
	return me.client.Close()
}

func newGoogleCloudMetricsExporter(
	ctx context.Context,
	cfg *Config,
	set component.ExporterCreateSettings,
) (component.MetricsExporter, error) {
	setVersionInUserAgent(cfg, set.BuildInfo.Version)

	// TODO: map cfg options into metric service client configuration with
	// generateClientOptions()
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		return nil, err
	}

	mExp := &metricsExporter{
		cfg:    cfg,
		client: client,
	}

	return exporterhelper.NewMetricsExporter(
		cfg,
		set,
		mExp.pushMetrics,
		exporterhelper.WithShutdown(mExp.Shutdown),
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: defaultTimeout}),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings))
}

// pushMetrics calls pushes pdata metrics to GCM, creating metric descriptors if necessary
func (me *metricsExporter) pushMetrics(ctx context.Context, m pdata.Metrics) error {
	timeSeries := make([]*monitoringpb.TimeSeries, 0, m.DataPointCount())

	rms := m.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		monitoredResource, extraResourceLabels := resourceMetricsToMonitoredResource(rm.Resource())
		ilms := rm.InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)

			extraLabels := mergeLabels(instrumentationLibraryToLabels(ilm.InstrumentationLibrary()), extraResourceLabels)
			mes := ilm.Metrics()
			for k := 0; k < mes.Len(); k++ {
				me := mes.At(k)
				timeSeries = append(timeSeries, metricToTimeSeries(monitoredResource, extraLabels, ilm, me)...)
			}
		}
	}

	// TODO: self observability
	err := me.createTimeSeries(ctx, timeSeries)
	recordPointCount(ctx, len(timeSeries), m.DataPointCount()-len(timeSeries), err)
	return err
}

func (me *metricsExporter) createTimeSeries(ctx context.Context, ts []*monitoringpb.TimeSeries) error {
	return me.client.CreateServiceTimeSeries(
		ctx,
		&monitoringpb.CreateTimeSeriesRequest{
			TimeSeries: ts,
			// TODO set Name field with project ID from config or ADC
		},
	)
}

// Transforms pdata Resource to a GCM Monitored Resource. Any resource attributes not accounted
// for in the monitored resource which should be merged into metric labels are also returned.
func resourceMetricsToMonitoredResource(
	resource pdata.Resource,
) (*monitoredrespb.MonitoredResource, labels) {
	// TODO
	return nil, nil
}

func instrumentationLibraryToLabels(il pdata.InstrumentationLibrary) labels {
	// TODO
	return nil
}

func metricToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	ilm pdata.InstrumentationLibraryMetrics,
	me pdata.Metric,
) []*monitoringpb.TimeSeries {
	timeSeries := []*monitoringpb.TimeSeries{}

	switch me.DataType() {
	case pdata.MetricDataTypeSum:
		sum := me.Sum()
		points := sum.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := sumPointToTimeSeries(resource, extraLabels, ilm, me, sum, points.At(i))
			timeSeries = append(timeSeries, ts)
		}
	// TODO: add cases for other metric data types
	default:
		// TODO: log unsupported metric
	}

	return timeSeries
}

func sumPointToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	ilm pdata.InstrumentationLibraryMetrics,
	me pdata.Metric,
	sum pdata.Sum,
	point pdata.NumberDataPoint,
) *monitoringpb.TimeSeries {
	// TODO
	return &monitoringpb.TimeSeries{
		Resource: resource,
		Unit:     me.Unit(),
		Metric: &metric.Metric{
			Labels: mergeLabels(
				attributesToLabels(point.Attributes()),
				extraLabels,
				// ... instrumentation library
			),
			// ...
		},
		// ...
	}
}

func attributesToLabels(
	attrs pdata.AttributeMap,
) labels {
	// TODO
	ls := make(labels, attrs.Len())
	attrs.Range(func(k string, v pdata.AttributeValue) bool {
		ls[sanitizeKey(k)] = v.AsString()
		return true
	})
	return ls
}

// Replaces non-alphanumeric characters to underscores. Note, this does not truncate label keys
// longer than 100 characters or prepend "key" when the first character is "_" like OpenCensus
// did.
func sanitizeKey(s string) string {
	if len(s) == 0 {
		return s
	}
	s = strings.Map(sanitizeRune, s)
	if unicode.IsDigit(rune(s[0])) {
		s = "key_" + s
	}
	return s
}

// converts anything that is not a letter or digit to an underscore
func sanitizeRune(r rune) rune {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return r
	}
	// Everything else turns into an underscore
	return '_'
}

func mergeLabels(mergeInto labels, others ...labels) labels {
	if mergeInto == nil {
		mergeInto = labels{}
	}
	for _, ls := range others {
		for k, v := range ls {
			mergeInto[k] = v
		}
	}

	return mergeInto
}
