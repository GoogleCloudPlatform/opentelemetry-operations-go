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
	"fmt"
	"log"
	"net/url"
	"path"
	"strings"
	"unicode"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
	"google.golang.org/genproto/googleapis/api/label"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// metricsExporter is the GCM exporter that uses pdata directly
type metricsExporter struct {
	cfg    *Config
	client *monitoring.MetricClient
	mapper metricMapper
	// A channel that receives metric descriptor and sends them to GCM once.
	mds chan *metricpb.MetricDescriptor
}

// metricMapper is the part that transforms metrics. Separate from metricsExporter since it has
// all pure functions.
type metricMapper struct {
	cfg *Config
}

// Known metric domains. Note: This is now configurable for advanced usages.
var domains = []string{"googleapis.com", "kubernetes.io", "istio.io", "knative.dev"}

// Constants we use when translating summary metrics into GCP.
const (
	SummaryCountPrefix      = "_summary_count"
	SummarySumSuffix        = "_summary_sum"
	SummaryPercentileSuffix = "_summary_percentile"
)

type labels map[string]string

func (me *metricsExporter) Shutdown(context.Context) error {
	close(me.mds)
	return me.client.Close()
}

func newGoogleCloudMetricsExporter(
	ctx context.Context,
	cfg *Config,
	set component.ExporterCreateSettings,
) (component.MetricsExporter, error) {
	setVersionInUserAgent(cfg, set.BuildInfo.Version)

	clientOpts, err := generateClientOptions(cfg)
	if err != nil {
		return nil, err
	}

	client, err := monitoring.NewMetricClient(ctx, clientOpts...)
	if err != nil {
		return nil, err
	}

	mExp := &metricsExporter{
		cfg:    cfg,
		client: client,
		mapper: metricMapper{cfg},
		// We create a buffered channel for metric descriptors.
		// MetricDescritpors are asycnhronously sent and optimistic.
		// We only get Unit/Description/Display name from them, so it's ok
		// to drop / conserve resources for sending timeseries.
		mds: make(chan *metricpb.MetricDescriptor, 10),
	}

	// Fire up the metric descriptor exporter.
	go mExp.exportMetricDescriptorRunner()

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
		monitoredResource, extraResourceLabels := me.mapper.resourceMetricsToMonitoredResource(rm.Resource())
		ilms := rm.InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)

			extraLabels := mergeLabels(instrumentationLibraryToLabels(ilm.InstrumentationLibrary()), extraResourceLabels)
			mes := ilm.Metrics()
			for k := 0; k < mes.Len(); k++ {
				metric := mes.At(k)
				// TODO - check to see if this is a service/system metric and doesn't send descriptors.
				if !me.cfg.MetricConfig.SkipCreateMetricDescriptor {
					for _, md := range me.mapper.metricDescriptor(metric) {
						if md != nil {
							select {
							case me.mds <- md:
							default:
								// Ignore drops, we'll catch descriptor next time around.
							}
						}
					}
				}
				timeSeries = append(timeSeries, me.mapper.metricToTimeSeries(monitoredResource, extraLabels, metric)...)
			}
		}
	}

	// TODO: self observability
	// TODO: Figure out how to configure service time series calls.
	if false {
		err := me.createServiceTimeSeries(ctx, timeSeries)
		recordPointCount(ctx, len(timeSeries), m.DataPointCount()-len(timeSeries), err)
		return err
	}
	err := me.createTimeSeries(ctx, timeSeries)
	recordPointCount(ctx, len(timeSeries), m.DataPointCount()-len(timeSeries), err)
	return err
}

// Reads metric descriptors from the md channel, and reports them (once) to GCM.
func (me *metricsExporter) exportMetricDescriptorRunner() {
	mdCache := make(map[string]*metricpb.MetricDescriptor)
	// We iterate over all metric descritpors until the channel is closed.
	// Note: if we get terminated, this will still attempt to export all descriptors
	// prior to shutdown.
	for md := range me.mds {
		// Not yet sent, now we sent it.
		if mdCache[md.Type] == nil {
			err := me.exportMetricDescriptor(context.TODO(), md)
			// TODO: Log-once on error, per metric descriptor?
			// TODO: Re-use passed-in logger to exporter.
			if err != nil {
				log.Printf("Unable to send metric descriptor: %s, %s", md, err)
				continue
			}
			mdCache[md.Type] = md
		}
		// TODO: We may want to compare current MD vs. previous and validate no changes.
	}
}

func (me *metricsExporter) projectName() string {
	// TODO set Name field with project ID from config or ADC
	return fmt.Sprintf("projects/%s", me.cfg.ProjectID)
}

// Helper method to send metric descriptors to GCM.
func (me *metricsExporter) exportMetricDescriptor(ctx context.Context, md *metricpb.MetricDescriptor) error {
	// export
	req := &monitoringpb.CreateMetricDescriptorRequest{
		Name:             me.projectName(),
		MetricDescriptor: md,
	}
	_, err := me.client.CreateMetricDescriptor(ctx, req)
	return err
}

// Sends a user-custom-metric timeseries.
func (me *metricsExporter) createTimeSeries(ctx context.Context, ts []*monitoringpb.TimeSeries) error {
	return me.client.CreateTimeSeries(
		ctx,
		&monitoringpb.CreateTimeSeriesRequest{
			Name:       me.projectName(),
			TimeSeries: ts,
		},
	)
}

// Sends a service timeseries.
func (me *metricsExporter) createServiceTimeSeries(ctx context.Context, ts []*monitoringpb.TimeSeries) error {
	return me.client.CreateServiceTimeSeries(
		ctx,
		&monitoringpb.CreateTimeSeriesRequest{
			Name:       me.projectName(),
			TimeSeries: ts,
		},
	)
}

// Transforms pdata Resource to a GCM Monitored Resource. Any resource attributes not accounted
// for in the monitored resource which should be merged into metric labels are also returned.
func (m *metricMapper) resourceMetricsToMonitoredResource(
	resource pdata.Resource,
) (*monitoredrespb.MonitoredResource, labels) {
	// TODO
	return nil, nil
}

func instrumentationLibraryToLabels(il pdata.InstrumentationLibrary) labels {
	// TODO
	return nil
}

func (m *metricMapper) metricToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pdata.Metric,
) []*monitoringpb.TimeSeries {
	timeSeries := []*monitoringpb.TimeSeries{}

	switch metric.DataType() {
	case pdata.MetricDataTypeSum:
		sum := metric.Sum()
		points := sum.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.sumPointToTimeSeries(resource, extraLabels, metric, sum, points.At(i))
			timeSeries = append(timeSeries, ts)
		}
	case pdata.MetricDataTypeGauge:
		gauge := metric.Gauge()
		points := gauge.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.gaugePointToTimeSeries(resource, extraLabels, metric, gauge, points.At(i))
			timeSeries = append(timeSeries, ts)
		}
	// TODO: add cases for other metric data types
	default:
		// TODO: log unsupported metric
	}

	return timeSeries
}

func (m *metricMapper) sumPointToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pdata.Metric,
	sum pdata.Sum,
	point pdata.NumberDataPoint,
) *monitoringpb.TimeSeries {
	metricKind := metricpb.MetricDescriptor_CUMULATIVE
	startTime := timestamppb.New(point.StartTimestamp().AsTime())
	if !sum.IsMonotonic() {
		metricKind = metricpb.MetricDescriptor_GAUGE
		startTime = nil
	}
	value, valueType := numberDataPointToValue(point)

	return &monitoringpb.TimeSeries{
		Resource:   resource,
		Unit:       metric.Unit(),
		MetricKind: metricKind,
		ValueType:  valueType,
		Points: []*monitoringpb.Point{{
			Interval: &monitoringpb.TimeInterval{
				StartTime: startTime,
				EndTime:   timestamppb.New(point.Timestamp().AsTime()),
			},
			Value: value,
		}},
		Metric: &metricpb.Metric{
			Type: m.metricNameToType(metric.Name()),
			Labels: mergeLabels(
				attributesToLabels(point.Attributes()),
				extraLabels,
			),
		},
	}
}

func (m *metricMapper) gaugePointToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pdata.Metric,
	gauge pdata.Gauge,
	point pdata.NumberDataPoint,
) *monitoringpb.TimeSeries {
	metricKind := metricpb.MetricDescriptor_GAUGE
	value, valueType := numberDataPointToValue(point)

	return &monitoringpb.TimeSeries{
		Resource:   resource,
		Unit:       metric.Unit(),
		MetricKind: metricKind,
		ValueType:  valueType,
		Points: []*monitoringpb.Point{{
			Interval: &monitoringpb.TimeInterval{
				EndTime: timestamppb.New(point.Timestamp().AsTime()),
			},
			Value: value,
		}},
		Metric: &metricpb.Metric{
			Type: m.metricNameToType(metric.Name()),
			Labels: mergeLabels(
				attributesToLabels(point.Attributes()),
				extraLabels,
			),
		},
	}
}

// Returns any configured prefix to add to unknown metric name.
func (m *metricMapper) getMetricNamePrefix(name string) string {
	for _, domain := range m.cfg.MetricConfig.KnownDomains {
		if strings.Contains(name, domain) {
			return ""
		}
	}
	return m.cfg.MetricConfig.Prefix
}

// metricNameToType maps OTLP metric name to GCM metric type (aka name)
func (m *metricMapper) metricNameToType(name string) string {
	return path.Join(m.getMetricNamePrefix(name), name)
}

func numberDataPointToValue(
	point pdata.NumberDataPoint,
) (*monitoringpb.TypedValue, metricpb.MetricDescriptor_ValueType) {
	if point.Type() == pdata.MetricValueTypeInt {
		return &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_Int64Value{
				Int64Value: point.IntVal(),
			}},
			metricpb.MetricDescriptor_INT64
	}
	return &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{
			DoubleValue: point.DoubleVal(),
		}},
		metricpb.MetricDescriptor_DOUBLE
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

// Takes a GCM metric type, like (workload.googleapis.com/MyCoolMetric) and returns the display name.
func (m *metricMapper) metricTypeToDisplayName(mURL string) string {
	// TODO - user configuration around display name?
	// Default: strip domain, keep path after domain.
	u, err := url.Parse(fmt.Sprintf("metrics://%s", mURL))
	if err != nil {
		return mURL
	}
	return strings.TrimLeft(u.Path, "/")
}

// Returns label descriptors for a metric.
func (m *metricMapper) labelDescriptors(pm pdata.Metric) []*label.LabelDescriptor {
	// TODO - allow customization of label descriptions.
	result := []*label.LabelDescriptor{}
	addAttributes := func(attr pdata.AttributeMap) {
		attr.Range(func(key string, _ pdata.AttributeValue) bool {
			result = append(result, &label.LabelDescriptor{
				Key: key,
			})
			return true
		})
	}
	switch pm.DataType() {
	case pdata.MetricDataTypeGauge:
		points := pm.Gauge().DataPoints()
		for i := 0; i < points.Len(); i++ {
			addAttributes(points.At(i).Attributes())
		}
	case pdata.MetricDataTypeSum:
		points := pm.Sum().DataPoints()
		for i := 0; i < points.Len(); i++ {
			addAttributes(points.At(i).Attributes())
		}
	case pdata.MetricDataTypeSummary:
		points := pm.Summary().DataPoints()
		for i := 0; i < points.Len(); i++ {
			addAttributes(points.At(i).Attributes())
		}
	case pdata.MetricDataTypeHistogram:
		points := pm.Histogram().DataPoints()
		for i := 0; i < points.Len(); i++ {
			addAttributes(points.At(i).Attributes())
		}
	case pdata.MetricDataTypeExponentialHistogram:
		points := pm.ExponentialHistogram().DataPoints()
		for i := 0; i < points.Len(); i++ {
			addAttributes(points.At(i).Attributes())
		}
	}
	return result
}

func (m *metricMapper) summaryMetricDescriptors(pm pdata.Metric) []*metricpb.MetricDescriptor {
	sumName := fmt.Sprintf("%s%s", pm.Name(), SummarySumSuffix)
	countName := fmt.Sprintf("%s%s", pm.Name(), SummaryCountPrefix)
	percentileName := fmt.Sprintf("%s%s", pm.Name(), SummaryPercentileSuffix)
	labels := m.labelDescriptors(pm)
	return []*metricpb.MetricDescriptor{
		{
			Type:        m.metricNameToType(sumName),
			Labels:      labels,
			MetricKind:  metricpb.MetricDescriptor_CUMULATIVE,
			ValueType:   metricpb.MetricDescriptor_DOUBLE,
			Unit:        pm.Unit(),
			Description: pm.Description(),
			DisplayName: sumName,
		},
		{
			Type:        m.metricNameToType(countName),
			Labels:      labels,
			MetricKind:  metricpb.MetricDescriptor_CUMULATIVE,
			ValueType:   metricpb.MetricDescriptor_INT64,
			Unit:        pm.Unit(),
			Description: pm.Description(),
			DisplayName: countName,
		},
		{
			Type: m.metricNameToType(percentileName),
			Labels: append(
				labels,
				&label.LabelDescriptor{
					Key:         "percentile",
					Description: "the value at a given percentile of a distribution",
				}),
			MetricKind:  metricpb.MetricDescriptor_GAUGE,
			ValueType:   metricpb.MetricDescriptor_DOUBLE,
			Unit:        pm.Unit(),
			Description: pm.Description(),
			DisplayName: percentileName,
		},
	}
}

// Extract the metric descriptor from a metric data point.
func (m *metricMapper) metricDescriptor(pm pdata.Metric) []*metricpb.MetricDescriptor {
	if pm.DataType() == pdata.MetricDataTypeSummary {
		return m.summaryMetricDescriptors(pm)
	}
	kind, typ := mapMetricPointKind(pm)
	metricType := m.metricNameToType(pm.Name())
	labels := m.labelDescriptors(pm)
	// Return nil for unsupported types.
	if kind == metricpb.MetricDescriptor_METRIC_KIND_UNSPECIFIED {
		return nil
	}
	return []*metricpb.MetricDescriptor{
		{
			Name:        pm.Name(),
			DisplayName: m.metricTypeToDisplayName(metricType),
			Type:        metricType,
			MetricKind:  kind,
			ValueType:   typ,
			Unit:        pm.Unit(),
			Description: pm.Description(),
			Labels:      labels,
		},
	}
}

func metricPointValueType(pt pdata.MetricValueType) metricpb.MetricDescriptor_ValueType {
	switch pt {
	case pdata.MetricValueTypeInt:
		return metricpb.MetricDescriptor_INT64
	case pdata.MetricValueTypeDouble:
		return metricpb.MetricDescriptor_DOUBLE
	default:
		return metricpb.MetricDescriptor_VALUE_TYPE_UNSPECIFIED
	}
}

func mapMetricPointKind(m pdata.Metric) (metricpb.MetricDescriptor_MetricKind, metricpb.MetricDescriptor_ValueType) {
	var kind metricpb.MetricDescriptor_MetricKind
	var typ metricpb.MetricDescriptor_ValueType
	switch m.DataType() {
	case pdata.MetricDataTypeGauge:
		kind = metricpb.MetricDescriptor_GAUGE
		if m.Gauge().DataPoints().Len() > 0 {
			typ = metricPointValueType(m.Gauge().DataPoints().At(0).Type())
		}
	case pdata.MetricDataTypeSum:
		if !m.Sum().IsMonotonic() {
			kind = metricpb.MetricDescriptor_GAUGE
		} else if m.Sum().AggregationTemporality() == pdata.MetricAggregationTemporalityDelta {
			// We report fake-deltas for now.
			kind = metricpb.MetricDescriptor_CUMULATIVE
		} else {
			kind = metricpb.MetricDescriptor_CUMULATIVE
		}
		if m.Sum().DataPoints().Len() > 0 {
			typ = metricPointValueType(m.Sum().DataPoints().At(0).Type())
		}
	case pdata.MetricDataTypeSummary:
		kind = metricpb.MetricDescriptor_GAUGE
	case pdata.MetricDataTypeHistogram:
		typ = metricpb.MetricDescriptor_DISTRIBUTION
		if m.Histogram().AggregationTemporality() == pdata.MetricAggregationTemporalityDelta {
			// We report fake-deltas for now.
			kind = metricpb.MetricDescriptor_CUMULATIVE
		} else {
			kind = metricpb.MetricDescriptor_CUMULATIVE
		}
	case pdata.MetricDataTypeExponentialHistogram:
		typ = metricpb.MetricDescriptor_DISTRIBUTION
		if m.ExponentialHistogram().AggregationTemporality() == pdata.MetricAggregationTemporalityDelta {
			// We report fake-deltas for now.
			kind = metricpb.MetricDescriptor_CUMULATIVE
		} else {
			kind = metricpb.MetricDescriptor_CUMULATIVE
		}
	default:
		kind = metricpb.MetricDescriptor_METRIC_KIND_UNSPECIFIED
		typ = metricpb.MetricDescriptor_VALUE_TYPE_UNSPECIFIED
	}
	return kind, typ
}
