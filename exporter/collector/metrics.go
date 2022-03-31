// Copyright 2022 Google LLC
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
	"math"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/api/distribution"
	"google.golang.org/genproto/googleapis/api/label"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/datapointstorage"
)

// self-observability reporting meters/tracers/loggers.
type selfObservability struct {
	// Logger to use for this exporter.
	log *zap.Logger
}

// MetricsExporter is the GCM exporter that uses pdata directly
type MetricsExporter struct {
	cfg    Config
	client *monitoring.MetricClient
	obs    selfObservability
	mapper metricMapper

	// tracks the currently running child tasks
	goroutines sync.WaitGroup
	// channel for signaling a graceful shutdown
	shutdownC chan struct{}

	// A channel that receives metric descriptor and sends them to GCM once
	metricDescriptorC chan *metricpb.MetricDescriptor
	// Tracks the metric descriptors that have already been sent to GCM
	mdCache map[string]*metricpb.MetricDescriptor

	// A channel that receives timeserieses and exports them to GCM in batches
	timeSeriesC chan *monitoringpb.TimeSeries
	// stores the currently pending batch of timeserieses
	pendingTimeSerieses []*monitoringpb.TimeSeries
	batchTimeoutTimer   *time.Timer
}

// metricMapper is the part that transforms metrics. Separate from MetricsExporter since it has
// all pure functions.
type metricMapper struct {
	obs      selfObservability
	cfg      Config
	sumCache datapointstorage.Cache
}

// Constants we use when translating summary metrics into GCP.
const (
	SummaryCountPrefix = "_count"
	SummarySumSuffix   = "_sum"
)

const (
	// batchTimeout is how long to wait to build a full batch before sending
	// off what we already have. We set it to 10 seconds because GCM
	// throttles us to this anyway.
	batchTimeout = 10 * time.Second

	// The number of timeserieses to send to GCM in a single request. This
	// is a hard limit in the GCM API, so we never want to exceed 200.
	sendBatchSize = 200
)

type labels map[string]string

func (me *MetricsExporter) Shutdown(ctx context.Context) error {
	// TODO: pass ctx to goroutines so that we can use its deadline
	close(me.shutdownC)
	c := make(chan struct{})
	go func() {
		// Wait until all goroutines are done
		me.goroutines.Wait()
		close(c)
	}()
	select {
	case <-ctx.Done():
		me.obs.log.Error("Error waiting for async tasks to finish.", zap.Error(ctx.Err()))
	case <-c:
	}
	return me.client.Close()
}

func NewGoogleCloudMetricsExporter(
	ctx context.Context,
	cfg Config,
	log *zap.Logger,
	version string,
	timeout time.Duration,
) (*MetricsExporter, error) {
	view.Register(MetricViews()...)
	view.Register(ocgrpc.DefaultClientViews...)
	setVersionInUserAgent(&cfg, version)
	setProjectFromADC(ctx, &cfg, monitoring.DefaultAuthScopes())

	clientOpts, err := generateClientOptions(&cfg.MetricConfig.ClientConfig, cfg.UserAgent)
	if err != nil {
		return nil, err
	}

	client, err := monitoring.NewMetricClient(ctx, clientOpts...)
	if err != nil {
		return nil, err
	}
	obs := selfObservability{log: log}
	shutdown := make(chan struct{})
	mExp := &MetricsExporter{
		cfg:    cfg,
		client: client,
		obs:    obs,
		mapper: metricMapper{
			obs,
			cfg,
			datapointstorage.NewCache(shutdown),
		},
		// We create a buffered channel for metric descriptors.
		// MetricDescritpors are asychronously sent and optimistic.
		// We only get Unit/Description/Display name from them, so it's ok
		// to drop / conserve resources for sending timeseries.
		metricDescriptorC: make(chan *metricpb.MetricDescriptor, cfg.MetricConfig.CreateMetricDescriptorBufferSize),
		mdCache:           make(map[string]*metricpb.MetricDescriptor),
		timeSeriesC:       make(chan *monitoringpb.TimeSeries),
		shutdownC:         shutdown,
	}

	// Fire up the metric descriptor exporter.
	mExp.goroutines.Add(1)
	go mExp.exportMetricDescriptorRunner()

	// Fire up the time series exporter.
	mExp.goroutines.Add(1)
	go mExp.exportTimeSeriesRunner()

	return mExp, nil
}

// PushMetrics calls pushes pdata metrics to GCM, creating metric descriptors if necessary
func (me *MetricsExporter) PushMetrics(ctx context.Context, m pdata.Metrics) error {
	rms := m.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		monitoredResource, extraResourceLabels := me.mapper.resourceToMonitoredResource(rm.Resource())
		ilms := rm.InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)

			instrumentationLibraryLabels := me.mapper.instrumentationLibraryToLabels(ilm.InstrumentationLibrary())
			metricLabels := mergeLabels(nil, instrumentationLibraryLabels, extraResourceLabels)

			mes := ilm.Metrics()
			for k := 0; k < mes.Len(); k++ {
				metric := mes.At(k)
				for _, ts := range me.mapper.metricToTimeSeries(monitoredResource, metricLabels, metric) {
					me.timeSeriesC <- ts
				}

				// We only send metric descriptors if we're configured *and* we're not sending service timeseries.
				if me.cfg.MetricConfig.SkipCreateMetricDescriptor || me.cfg.MetricConfig.CreateServiceTimeSeries {
					continue
				}

				for _, md := range me.mapper.metricDescriptor(metric, metricLabels) {
					if md == nil {
						continue
					}
					select {
					case me.metricDescriptorC <- md:
					default:
						// Ignore drops, we'll catch descriptor next time around.
					}
				}
			}
		}
	}
	return nil
}

func (me *MetricsExporter) exportPendingTimeSerieses() {
	ctx := context.Background()

	var sendSize int
	if len(me.pendingTimeSerieses) < sendBatchSize {
		sendSize = len(me.pendingTimeSerieses)
	} else {
		sendSize = sendBatchSize
	}

	var ts []*monitoringpb.TimeSeries
	ts, me.pendingTimeSerieses = me.pendingTimeSerieses, me.pendingTimeSerieses[sendSize:]

	var err error
	if me.cfg.MetricConfig.CreateServiceTimeSeries {
		err = me.createServiceTimeSeries(ctx, ts)
	} else {
		err = me.createTimeSeries(ctx, ts)
	}

	var st string
	s, _ := status.FromError(err)
	st = statusCodeToString(s)

	recordPointCountDataPoint(ctx, len(ts), st)
	if err != nil {
		me.obs.log.Error("could not export time series to GCM", zap.Error(err))
	}
}

// Reads metric descriptors from the md channel, and reports them (once) to GCM.
func (me *MetricsExporter) exportMetricDescriptorRunner() {
	defer me.goroutines.Done()

	// We iterate over all metric descritpors until the channel is closed.
	// Note: if we get terminated, this will still attempt to export all descriptors
	// prior to shutdown.
	for {
		select {
		case <-me.shutdownC:
			for {
				// We are shutting down. Publish all the pending
				// items on the channel before we stop.
				select {
				case md := <-me.metricDescriptorC:
					me.exportMetricDescriptor(md)
				default:
					// Return and continue graceful shutdown.
					return
				}
			}

		case md := <-me.metricDescriptorC:
			me.exportMetricDescriptor(md)
		}
	}
}

func (me *MetricsExporter) projectName() string {
	return fmt.Sprintf("projects/%s", me.cfg.ProjectID)
}

// Helper method to send metric descriptors to GCM.
func (me *MetricsExporter) exportMetricDescriptor(md *metricpb.MetricDescriptor) {
	if _, exists := me.mdCache[md.Type]; exists {
		return
	}

	req := &monitoringpb.CreateMetricDescriptorRequest{
		Name:             me.projectName(),
		MetricDescriptor: md,
	}
	_, err := me.client.CreateMetricDescriptor(context.Background(), req)
	if err != nil {
		// TODO: Log-once on error, per metric descriptor?
		me.obs.log.Error("Unable to send metric descriptor.", zap.Error(err), zap.Any("metric_descriptor", md))
		return
	}

	// only cache if we are successful. We want to retry if there is an error
	me.mdCache[md.Type] = md
}

// Sends a user-custom-metric timeseries.
func (me *MetricsExporter) createTimeSeries(ctx context.Context, ts []*monitoringpb.TimeSeries) error {
	return me.client.CreateTimeSeries(
		ctx,
		&monitoringpb.CreateTimeSeriesRequest{
			Name:       me.projectName(),
			TimeSeries: ts,
		},
	)
}

// Sends a service timeseries.
func (me *MetricsExporter) createServiceTimeSeries(ctx context.Context, ts []*monitoringpb.TimeSeries) error {
	return me.client.CreateServiceTimeSeries(
		ctx,
		&monitoringpb.CreateTimeSeriesRequest{
			Name:       me.projectName(),
			TimeSeries: ts,
		},
	)
}

func (m *metricMapper) instrumentationLibraryToLabels(il pdata.InstrumentationLibrary) labels {
	if !m.cfg.MetricConfig.InstrumentationLibraryLabels {
		return labels{}
	}
	return labels{
		"instrumentation_source":  il.Name(),
		"instrumentation_version": il.Version(),
	}
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
			timeSeries = append(timeSeries, ts...)
		}
	case pdata.MetricDataTypeGauge:
		gauge := metric.Gauge()
		points := gauge.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.gaugePointToTimeSeries(resource, extraLabels, metric, gauge, points.At(i))
			timeSeries = append(timeSeries, ts...)
		}
	case pdata.MetricDataTypeSummary:
		summary := metric.Summary()
		points := summary.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.summaryPointToTimeSeries(resource, extraLabels, metric, summary, points.At(i))
			timeSeries = append(timeSeries, ts...)
		}
	case pdata.MetricDataTypeHistogram:
		hist := metric.Histogram()
		points := hist.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.histogramToTimeSeries(resource, extraLabels, metric, hist, points.At(i))
			timeSeries = append(timeSeries, ts...)
		}
	case pdata.MetricDataTypeExponentialHistogram:
		eh := metric.ExponentialHistogram()
		points := eh.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.exponentialHistogramToTimeSeries(resource, extraLabels, metric, eh, points.At(i))
			timeSeries = append(timeSeries, ts...)
		}
	default:
		m.obs.log.Error("Unsupported metric data type", zap.Any("data_type", metric.DataType()))
	}

	return timeSeries
}

func (m *metricMapper) summaryPointToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pdata.Metric,
	sum pdata.Summary,
	point pdata.SummaryDataPoint,
) []*monitoringpb.TimeSeries {
	if point.Flags().HasFlag(pdata.MetricDataPointFlagNoRecordedValue) {
		// Drop points without a value.
		return nil
	}
	sumName, countName := summaryMetricNames(metric.Name())
	startTime := timestamppb.New(point.StartTimestamp().AsTime())
	endTime := timestamppb.New(point.Timestamp().AsTime())
	result := []*monitoringpb.TimeSeries{
		{
			Resource:   resource,
			Unit:       metric.Unit(),
			MetricKind: metricpb.MetricDescriptor_CUMULATIVE,
			ValueType:  metricpb.MetricDescriptor_DOUBLE,
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					StartTime: startTime,
					EndTime:   endTime,
				},
				Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{
					DoubleValue: point.Sum(),
				}},
			}},
			Metric: &metricpb.Metric{
				Type: m.metricNameToType(sumName),
				Labels: mergeLabels(
					attributesToLabels(point.Attributes()),
					extraLabels,
				),
			},
		},
		{
			Resource:   resource,
			Unit:       metric.Unit(),
			MetricKind: metricpb.MetricDescriptor_CUMULATIVE,
			ValueType:  metricpb.MetricDescriptor_DOUBLE,
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					StartTime: startTime,
					EndTime:   endTime,
				},
				Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{
					DoubleValue: float64(point.Count()),
				}},
			}},
			Metric: &metricpb.Metric{
				Type: m.metricNameToType(countName),
				Labels: mergeLabels(
					attributesToLabels(point.Attributes()),
					extraLabels,
				),
			},
		},
	}
	quantiles := point.QuantileValues()
	for i := 0; i < quantiles.Len(); i++ {
		quantile := quantiles.At(i)
		pLabel := labels{
			"quantile": strconv.FormatFloat(quantile.Quantile(), 'f', -1, 64),
		}
		result = append(result, &monitoringpb.TimeSeries{
			Resource:   resource,
			Unit:       metric.Unit(),
			MetricKind: metricpb.MetricDescriptor_GAUGE,
			ValueType:  metricpb.MetricDescriptor_DOUBLE,
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					EndTime: endTime,
				},
				Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{
					DoubleValue: quantile.Value(),
				}},
			}},
			Metric: &metricpb.Metric{
				Type: m.metricNameToType(metric.Name()),
				Labels: mergeLabels(
					attributesToLabels(point.Attributes()),
					extraLabels,
					pLabel,
				),
			},
		})
	}
	return result
}

func (m *metricMapper) exemplar(ex pdata.Exemplar) *distribution.Distribution_Exemplar {
	ctx := context.TODO()
	attachments := []*anypb.Any{}
	// TODO: Look into still sending exemplars with no span.
	if !ex.TraceID().IsEmpty() && !ex.SpanID().IsEmpty() {
		sctx, err := anypb.New(&monitoringpb.SpanContext{
			// TODO - make sure project id is correct.
			SpanName: fmt.Sprintf("projects/%s/traces/%s/spans/%s", m.cfg.ProjectID, ex.TraceID().HexString(), ex.SpanID().HexString()),
		})
		if err == nil {
			attachments = append(attachments, sctx)
		} else {
			// This happens in the event of logic error (e.g. missing required fields).
			// As such we complaining loudly to fail our unit tests.
			recordExemplarFailure(ctx, 1)
		}
	}
	if ex.FilteredAttributes().Len() > 0 {
		attr, err := anypb.New(&monitoringpb.DroppedLabels{
			Label: attributesToLabels(ex.FilteredAttributes()),
		})
		if err == nil {
			attachments = append(attachments, attr)
		} else {
			// This happens in the event of logic error (e.g. missing required fields).
			// As such we complaining loudly to fail our unit tests.
			recordExemplarFailure(ctx, 1)
		}
	}
	return &distribution.Distribution_Exemplar{
		Value:       ex.DoubleVal(),
		Timestamp:   timestamppb.New(ex.Timestamp().AsTime()),
		Attachments: attachments,
	}
}

func (m *metricMapper) exemplars(exs pdata.ExemplarSlice) []*distribution.Distribution_Exemplar {
	exemplars := make([]*distribution.Distribution_Exemplar, exs.Len())
	for i := 0; i < exs.Len(); i++ {
		exemplars[i] = m.exemplar(exs.At(i))
	}
	return exemplars
}

// histogramPoint maps a histogram data point into a GCM point.
func (m *metricMapper) histogramPoint(point pdata.HistogramDataPoint) *monitoringpb.TypedValue {
	counts := make([]int64, len(point.BucketCounts()))
	for i, v := range point.BucketCounts() {
		counts[i] = int64(v)
	}

	mean := float64(0)
	if !math.IsNaN(point.Sum()) && point.Count() > 0 { // Avoid divide-by-zero
		mean = float64(point.Sum() / float64(point.Count()))
	}

	return &monitoringpb.TypedValue{
		Value: &monitoringpb.TypedValue_DistributionValue{
			DistributionValue: &distribution.Distribution{
				Count:        int64(point.Count()),
				Mean:         mean,
				BucketCounts: counts,
				BucketOptions: &distribution.Distribution_BucketOptions{
					Options: &distribution.Distribution_BucketOptions_ExplicitBuckets{
						ExplicitBuckets: &distribution.Distribution_BucketOptions_Explicit{
							Bounds: point.ExplicitBounds(),
						},
					},
				},
				Exemplars: m.exemplars(point.Exemplars()),
			},
		},
	}
}

// Maps an exponential distribution into a GCM point.
func (m *metricMapper) exponentialHistogramPoint(point pdata.ExponentialHistogramDataPoint) *monitoringpb.TypedValue {
	// First calculate underflow bucket with all negatives + zeros.
	underflow := point.ZeroCount()
	for _, v := range point.Negative().BucketCounts() {
		underflow += v
	}
	// Next, pull in remaining buckets.
	counts := make([]int64, len(point.Positive().BucketCounts())+2)
	bucketOptions := &distribution.Distribution_BucketOptions{}
	counts[0] = int64(underflow)
	for i, v := range point.Positive().BucketCounts() {
		counts[i+1] = int64(v)
	}
	// Overflow bucket is always empty
	counts[len(counts)-1] = 0

	if len(point.Positive().BucketCounts()) == 0 {
		// We cannot send exponential distributions with no positive buckets,
		// instead we send a simple overflow/underflow histogram.
		bucketOptions.Options = &distribution.Distribution_BucketOptions_ExplicitBuckets{
			ExplicitBuckets: &distribution.Distribution_BucketOptions_Explicit{
				Bounds: []float64{0},
			},
		}
	} else {
		// Exponential histogram
		growth := math.Exp2(math.Exp2(-float64(point.Scale())))
		scale := math.Pow(growth, float64(point.Positive().Offset()))
		bucketOptions.Options = &distribution.Distribution_BucketOptions_ExponentialBuckets{
			ExponentialBuckets: &distribution.Distribution_BucketOptions_Exponential{
				GrowthFactor:     growth,
				Scale:            scale,
				NumFiniteBuckets: int32(len(counts) - 2),
			},
		}
	}

	mean := float64(0)
	if !math.IsNaN(point.Sum()) && point.Count() > 0 { // Avoid divide-by-zero
		mean = float64(point.Sum() / float64(point.Count()))
	}

	return &monitoringpb.TypedValue{
		Value: &monitoringpb.TypedValue_DistributionValue{
			DistributionValue: &distribution.Distribution{
				Count:         int64(point.Count()),
				Mean:          mean,
				BucketCounts:  counts,
				BucketOptions: bucketOptions,
				Exemplars:     m.exemplars(point.Exemplars()),
			},
		},
	}
}

func (m *metricMapper) histogramToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pdata.Metric,
	_ pdata.Histogram,
	point pdata.HistogramDataPoint,
) []*monitoringpb.TimeSeries {
	if point.Flags().HasFlag(pdata.MetricDataPointFlagNoRecordedValue) {
		// Drop points without a value.
		return nil
	}
	// We treat deltas as cumulatives w/ resets.
	metricKind := metricpb.MetricDescriptor_CUMULATIVE
	startTime := timestamppb.New(point.StartTimestamp().AsTime())
	endTime := timestamppb.New(point.Timestamp().AsTime())
	value := m.histogramPoint(point)
	return []*monitoringpb.TimeSeries{{
		Resource:   resource,
		Unit:       metric.Unit(),
		MetricKind: metricKind,
		ValueType:  metricpb.MetricDescriptor_DISTRIBUTION,
		Points: []*monitoringpb.Point{{
			Interval: &monitoringpb.TimeInterval{
				StartTime: startTime,
				EndTime:   endTime,
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
	}}
}

func (m *metricMapper) exponentialHistogramToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pdata.Metric,
	_ pdata.ExponentialHistogram,
	point pdata.ExponentialHistogramDataPoint,
) []*monitoringpb.TimeSeries {
	if point.Flags().HasFlag(pdata.MetricDataPointFlagNoRecordedValue) {
		// Drop points without a value.
		return nil
	}
	// We treat deltas as cumulatives w/ resets.
	metricKind := metricpb.MetricDescriptor_CUMULATIVE
	startTime := timestamppb.New(point.StartTimestamp().AsTime())
	endTime := timestamppb.New(point.Timestamp().AsTime())
	value := m.exponentialHistogramPoint(point)
	return []*monitoringpb.TimeSeries{{
		Resource:   resource,
		Unit:       metric.Unit(),
		MetricKind: metricKind,
		ValueType:  metricpb.MetricDescriptor_DISTRIBUTION,
		Points: []*monitoringpb.Point{{
			Interval: &monitoringpb.TimeInterval{
				StartTime: startTime,
				EndTime:   endTime,
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
	}}
}

func (m *metricMapper) sumPointToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pdata.Metric,
	sum pdata.Sum,
	point pdata.NumberDataPoint,
) []*monitoringpb.TimeSeries {
	metricKind := metricpb.MetricDescriptor_CUMULATIVE
	var startTime *timestamppb.Timestamp
	if point.Flags().HasFlag(pdata.MetricDataPointFlagNoRecordedValue) {
		// Drop points without a value.  This may be a staleness marker from
		// prometheus.
		return nil
	}
	if sum.IsMonotonic() {
		metricIdentifier := datapointstorage.Identifier(resource, extraLabels, metric, point.Attributes())
		normalizedPoint := m.normalizeNumberDataPoint(point, metricIdentifier)
		if normalizedPoint == nil {
			return nil
		}
		point = *normalizedPoint
		startTime = timestamppb.New(normalizedPoint.StartTimestamp().AsTime())
	} else {
		metricKind = metricpb.MetricDescriptor_GAUGE
		startTime = nil
	}
	value, valueType := numberDataPointToValue(point)

	return []*monitoringpb.TimeSeries{{
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
	}}
}

// normalizeNumberDataPoint returns a point which has been normalized against an initial
// start point, or nil if the point should be dropped.
func (m *metricMapper) normalizeNumberDataPoint(point pdata.NumberDataPoint, identifier string) *pdata.NumberDataPoint {
	// if the point doesn't need to be normalized, use original point
	normalizedPoint := &point
	start, ok := m.sumCache.Get(identifier)
	if ok {
		if !start.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
			// We found a cached start timestamp that wouldn't produce a valid point.
			// Drop it and log.
			m.obs.log.Info(
				"data point being processed older than last recorded reset, will not be emitted",
				zap.String("lastRecordedReset", start.Timestamp().String()),
				zap.String("dataPoint", point.Timestamp().String()),
			)
			return nil
		}
		// Make a copy so we don't mutate underlying data
		newPoint := pdata.NewNumberDataPoint()
		point.CopyTo(newPoint)
		// Use the start timestamp from the normalization point
		newPoint.SetStartTimestamp(start.Timestamp())
		// Adjust the value based on the start point's value
		switch newPoint.ValueType() {
		case pdata.MetricValueTypeInt:
			newPoint.SetIntVal(point.IntVal() - start.IntVal())
		case pdata.MetricValueTypeDouble:
			newPoint.SetDoubleVal(point.DoubleVal() - start.DoubleVal())
		}
		normalizedPoint = &newPoint
	}
	if (!ok && point.StartTimestamp() == 0) || !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
		// This is the first time we've seen this metric, or we received
		// an explicit reset point as described in
		// https://github.com/open-telemetry/opentelemetry-specification/blob/9555f9594c7ffe5dc333b53da5e0f880026cead1/specification/metrics/datamodel.md#resets-and-gaps
		// Record it in history and drop the point.
		m.sumCache.Set(identifier, &point)
		return nil
	}
	return normalizedPoint
}

func (m *metricMapper) gaugePointToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pdata.Metric,
	gauge pdata.Gauge,
	point pdata.NumberDataPoint,
) []*monitoringpb.TimeSeries {
	if point.Flags().HasFlag(pdata.MetricDataPointFlagNoRecordedValue) {
		// Drop points without a value.
		return nil
	}
	metricKind := metricpb.MetricDescriptor_GAUGE
	value, valueType := numberDataPointToValue(point)

	return []*monitoringpb.TimeSeries{{
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
	}}
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
	if point.ValueType() == pdata.MetricValueTypeInt {
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
func (m *metricMapper) labelDescriptors(
	pm pdata.Metric,
	extraLabels labels,
) []*label.LabelDescriptor {
	// TODO - allow customization of label descriptions.
	result := []*label.LabelDescriptor{}
	for key := range extraLabels {
		result = append(result, &label.LabelDescriptor{
			Key: sanitizeKey(key),
		})
	}

	seenKeys := map[string]struct{}{}
	addAttributes := func(attr pdata.AttributeMap) {
		attr.Range(func(key string, _ pdata.AttributeValue) bool {
			// Skip keys that have already been set
			if _, ok := seenKeys[sanitizeKey(key)]; ok {
				return true
			}
			result = append(result, &label.LabelDescriptor{
				Key: sanitizeKey(key),
			})
			seenKeys[sanitizeKey(key)] = struct{}{}
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

// Returns (sum, count) metric names for a summary metric.
func summaryMetricNames(name string) (string, string) {
	sumName := fmt.Sprintf("%s%s", name, SummarySumSuffix)
	countName := fmt.Sprintf("%s%s", name, SummaryCountPrefix)
	return sumName, countName
}

func (m *metricMapper) summaryMetricDescriptors(
	pm pdata.Metric,
	extraLabels labels,
) []*metricpb.MetricDescriptor {
	sumName, countName := summaryMetricNames(pm.Name())
	labels := m.labelDescriptors(pm, extraLabels)
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
			ValueType:   metricpb.MetricDescriptor_DOUBLE,
			Unit:        pm.Unit(),
			Description: pm.Description(),
			DisplayName: countName,
		},
		{
			Type: m.metricNameToType(pm.Name()),
			Labels: append(
				labels,
				&label.LabelDescriptor{
					Key:         "quantile",
					Description: "the value at a given quantile of a distribution",
				}),
			MetricKind:  metricpb.MetricDescriptor_GAUGE,
			ValueType:   metricpb.MetricDescriptor_DOUBLE,
			Unit:        pm.Unit(),
			Description: pm.Description(),
			DisplayName: pm.Name(),
		},
	}
}

// Extract the metric descriptor from a metric data point.
func (m *metricMapper) metricDescriptor(
	pm pdata.Metric,
	extraLabels labels,
) []*metricpb.MetricDescriptor {
	if pm.DataType() == pdata.MetricDataTypeSummary {
		return m.summaryMetricDescriptors(pm, extraLabels)
	}
	kind, typ := mapMetricPointKind(pm)
	metricType := m.metricNameToType(pm.Name())
	labels := m.labelDescriptors(pm, extraLabels)
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
			typ = metricPointValueType(m.Gauge().DataPoints().At(0).ValueType())
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
			typ = metricPointValueType(m.Sum().DataPoints().At(0).ValueType())
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

func (me *MetricsExporter) exportTimeSeriesRunner() {
	defer me.goroutines.Done()
	me.batchTimeoutTimer = time.NewTimer(batchTimeout)
	for {
		select {
		case <-me.shutdownC:
			for {
				// We are shutting down. Publish all the pending
				// items on the channel before we stop.
				select {
				case ts := <-me.timeSeriesC:
					me.processItem(ts)
				default:
					goto DONE
				}
			}
		DONE:
			for len(me.pendingTimeSerieses) > 0 {
				me.exportPendingTimeSerieses()
			}
			// Return and continue graceful shutdown.
			return
		case ts := <-me.timeSeriesC:
			me.processItem(ts)
		case <-me.batchTimeoutTimer.C:
			me.batchTimeoutTimer.Reset(batchTimeout)
			for len(me.pendingTimeSerieses) > 0 {
				me.exportPendingTimeSerieses()
			}
		}
	}
}

func (me *MetricsExporter) processItem(ts *monitoringpb.TimeSeries) {
	me.pendingTimeSerieses = append(me.pendingTimeSerieses, ts)
	if len(me.pendingTimeSerieses) >= sendBatchSize {
		if !me.batchTimeoutTimer.Stop() {
			<-me.batchTimeoutTimer.C
		}
		me.batchTimeoutTimer.Reset(batchTimeout)
		me.exportPendingTimeSerieses()
	}
}
