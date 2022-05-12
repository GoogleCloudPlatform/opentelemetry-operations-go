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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
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
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/normalization"
)

// self-observability reporting meters/tracers/loggers.
type selfObservability struct {
	// Logger to use for this exporter.
	log *zap.Logger
}

// MetricsExporter is the GCM exporter that uses pdata directly
type MetricsExporter struct {
	mapper metricMapper
	// A channel that receives metric descriptor and sends them to GCM once
	metricDescriptorC chan *metricpb.MetricDescriptor
	client            *monitoring.MetricClient
	obs               selfObservability
	// shutdownC is a channel for signaling a graceful shutdown
	shutdownC chan struct{}
	// mdCache tracks the metric descriptors that have already been sent to GCM
	mdCache map[string]*metricpb.MetricDescriptor
	cfg     Config
	// goroutines tracks the currently running child tasks
	goroutines sync.WaitGroup
	timeout    time.Duration
}

// metricMapper is the part that transforms metrics. Separate from MetricsExporter since it has
// all pure functions.
type metricMapper struct {
	normalizer normalization.Normalizer
	obs        selfObservability
	cfg        Config
}

// Constants we use when translating summary metrics into GCP.
const (
	SummaryCountPrefix = "_count"
	SummarySumSuffix   = "_sum"
)

const (
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
	normalizer := normalization.NewDisabledNormalizer()
	if cfg.MetricConfig.CumulativeNormalization {
		normalizer = normalization.NewStandardNormalizer(shutdown, log)
	}
	mExp := &MetricsExporter{
		cfg:    cfg,
		client: client,
		obs:    obs,
		mapper: metricMapper{
			obs:        obs,
			cfg:        cfg,
			normalizer: normalizer,
		},
		// We create a buffered channel for metric descriptors.
		// MetricDescritpors are asychronously sent and optimistic.
		// We only get Unit/Description/Display name from them, so it's ok
		// to drop / conserve resources for sending timeseries.
		metricDescriptorC: make(chan *metricpb.MetricDescriptor, cfg.MetricConfig.CreateMetricDescriptorBufferSize),
		mdCache:           make(map[string]*metricpb.MetricDescriptor),
		shutdownC:         shutdown,
		timeout:           timeout,
	}

	// Fire up the metric descriptor exporter.
	mExp.goroutines.Add(1)
	go mExp.exportMetricDescriptorRunner()

	return mExp, nil
}

// PushMetrics calls pushes pdata metrics to GCM, creating metric descriptors if necessary
func (me *MetricsExporter) PushMetrics(ctx context.Context, m pmetric.Metrics) error {
	pendingTimeSeries := []*monitoringpb.TimeSeries{}
	rms := m.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		monitoredResource := me.cfg.MetricConfig.MapMonitoredResource(rm.Resource())
		extraResourceLabels := me.mapper.resourceToMetricLabels(rm.Resource())
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)

			instrumentationScopeLabels := me.mapper.instrumentationScopeToLabels(sm.Scope())
			metricLabels := mergeLabels(nil, instrumentationScopeLabels, extraResourceLabels)

			mes := sm.Metrics()
			for k := 0; k < mes.Len(); k++ {
				metric := mes.At(k)
				pendingTimeSeries = append(pendingTimeSeries, me.mapper.metricToTimeSeries(monitoredResource, metricLabels, metric)...)

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
	// Batch and export
	for len(pendingTimeSeries) > 0 {
		var sendSize int
		if len(pendingTimeSeries) < sendBatchSize {
			sendSize = len(pendingTimeSeries)
		} else {
			sendSize = sendBatchSize
		}

		var ts []*monitoringpb.TimeSeries
		ts, pendingTimeSeries = pendingTimeSeries[:sendSize], pendingTimeSeries[sendSize:]

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
			return fmt.Errorf("failed to export time series to GCM: %v", err)
		}
	}
	return nil
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
	ctx, cancel := context.WithTimeout(context.Background(), me.timeout)
	defer cancel()
	_, err := me.client.CreateMetricDescriptor(ctx, req)
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
	ctx, cancel := context.WithTimeout(ctx, me.timeout)
	defer cancel()
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
	ctx, cancel := context.WithTimeout(ctx, me.timeout)
	defer cancel()
	return me.client.CreateServiceTimeSeries(
		ctx,
		&monitoringpb.CreateTimeSeriesRequest{
			Name:       me.projectName(),
			TimeSeries: ts,
		},
	)
}

func (m *metricMapper) instrumentationScopeToLabels(is pcommon.InstrumentationScope) labels {
	if !m.cfg.MetricConfig.InstrumentationLibraryLabels {
		return labels{}
	}
	return labels{
		"instrumentation_source":  sanitizeUTF8(is.Name()),
		"instrumentation_version": sanitizeUTF8(is.Version()),
	}
}

func (m *metricMapper) metricToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pmetric.Metric,
) []*monitoringpb.TimeSeries {
	timeSeries := []*monitoringpb.TimeSeries{}

	switch metric.DataType() {
	case pmetric.MetricDataTypeSum:
		sum := metric.Sum()
		points := sum.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.sumPointToTimeSeries(resource, extraLabels, metric, sum, points.At(i))
			timeSeries = append(timeSeries, ts...)
		}
	case pmetric.MetricDataTypeGauge:
		gauge := metric.Gauge()
		points := gauge.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.gaugePointToTimeSeries(resource, extraLabels, metric, gauge, points.At(i))
			timeSeries = append(timeSeries, ts...)
		}
	case pmetric.MetricDataTypeSummary:
		summary := metric.Summary()
		points := summary.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.summaryPointToTimeSeries(resource, extraLabels, metric, summary, points.At(i))
			timeSeries = append(timeSeries, ts...)
		}
	case pmetric.MetricDataTypeHistogram:
		hist := metric.Histogram()
		points := hist.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.histogramToTimeSeries(resource, extraLabels, metric, hist, points.At(i))
			timeSeries = append(timeSeries, ts...)
		}
	case pmetric.MetricDataTypeExponentialHistogram:
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
	metric pmetric.Metric,
	sum pmetric.Summary,
	point pmetric.SummaryDataPoint,
) []*monitoringpb.TimeSeries {
	if point.Flags().HasFlag(pmetric.MetricDataPointFlagNoRecordedValue) {
		// Drop points without a value.
		return nil
	}
	// Normalize the summary point.
	metricIdentifier := datapointstorage.Identifier(resource, extraLabels, metric, point.Attributes())
	normalizedPoint := m.normalizer.NormalizeSummaryDataPoint(point, metricIdentifier)
	if normalizedPoint == nil {
		return nil
	}
	point = *normalizedPoint
	sumType, countType, quantileType, err := m.summaryMetricTypes(metric)
	if err != nil {
		m.obs.log.Debug("Failed to get metric type (i.e. name) for summary metric. Dropping the metric.", zap.Error(err), zap.Any("metric", metric))
		return nil
	}
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
				Type: sumType,
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
				Type: countType,
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
				Type: quantileType,
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

func (m *metricMapper) exemplar(ex pmetric.Exemplar) *distribution.Distribution_Exemplar {
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

func (m *metricMapper) exemplars(exs pmetric.ExemplarSlice) []*distribution.Distribution_Exemplar {
	exemplars := make([]*distribution.Distribution_Exemplar, exs.Len())
	for i := 0; i < exs.Len(); i++ {
		exemplars[i] = m.exemplar(exs.At(i))
	}
	return exemplars
}

// histogramPoint maps a histogram data point into a GCM point.
func (m *metricMapper) histogramPoint(point pmetric.HistogramDataPoint) *monitoringpb.TypedValue {
	counts := make([]int64, len(point.MBucketCounts()))
	var mean, deviation, prevBound float64

	for i, v := range point.MBucketCounts() {
		counts[i] = int64(v)
	}

	if !math.IsNaN(point.Sum()) && point.Count() > 0 { // Avoid divide-by-zero
		mean = float64(point.Sum() / float64(point.Count()))
	}

	bounds := point.MExplicitBounds()
	if m.cfg.MetricConfig.EnableSumOfSquaredDeviation {
		// Calculate the sum of squared deviation.
		for i, bound := range bounds {
			// Assume all points in the bucket occur at the middle of the bucket range
			middleOfBucket := (prevBound + bound) / 2
			deviation += float64(counts[i]) * (middleOfBucket - mean) * (middleOfBucket - mean)
			prevBound = bound
		}
		// The infinity bucket is an implicit +Inf bound after the list of explicit bounds.
		// Assume points in the infinity bucket are at the top of the previous bucket
		middleOfInfBucket := prevBound
		deviation += float64(counts[len(counts)-1]) * (middleOfInfBucket - mean) * (middleOfInfBucket - mean)
	}

	return &monitoringpb.TypedValue{
		Value: &monitoringpb.TypedValue_DistributionValue{
			DistributionValue: &distribution.Distribution{
				Count:                 int64(point.Count()),
				Mean:                  mean,
				BucketCounts:          counts,
				SumOfSquaredDeviation: deviation,
				BucketOptions: &distribution.Distribution_BucketOptions{
					Options: &distribution.Distribution_BucketOptions_ExplicitBuckets{
						ExplicitBuckets: &distribution.Distribution_BucketOptions_Explicit{
							Bounds: bounds,
						},
					},
				},
				Exemplars: m.exemplars(point.Exemplars()),
			},
		},
	}
}

// Maps an exponential distribution into a GCM point.
func (m *metricMapper) exponentialHistogramPoint(point pmetric.ExponentialHistogramDataPoint) *monitoringpb.TypedValue {
	// First calculate underflow bucket with all negatives + zeros.
	underflow := point.ZeroCount()
	for _, v := range point.Negative().MBucketCounts() {
		underflow += v
	}
	// Next, pull in remaining buckets.
	counts := make([]int64, len(point.Positive().MBucketCounts())+2)
	bucketOptions := &distribution.Distribution_BucketOptions{}
	counts[0] = int64(underflow)
	for i, v := range point.Positive().MBucketCounts() {
		counts[i+1] = int64(v)
	}
	// Overflow bucket is always empty
	counts[len(counts)-1] = 0

	if len(point.Positive().MBucketCounts()) == 0 {
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
	metric pmetric.Metric,
	hist pmetric.Histogram,
	point pmetric.HistogramDataPoint,
) []*monitoringpb.TimeSeries {
	if point.Flags().HasFlag(pmetric.MetricDataPointFlagNoRecordedValue) || !point.HasSum() {
		// Drop points without a value or without a sum
		return nil
	}
	t, err := m.metricNameToType(metric.Name(), metric)
	if err != nil {
		m.obs.log.Debug("Failed to get metric type (i.e. name) for histogram metric. Dropping the metric.", zap.Error(err), zap.Any("metric", metric))
		return nil
	}
	if hist.AggregationTemporality() == pmetric.MetricAggregationTemporalityCumulative {
		// Normalize cumulative histogram points.
		metricIdentifier := datapointstorage.Identifier(resource, extraLabels, metric, point.Attributes())
		normalizedPoint := m.normalizer.NormalizeHistogramDataPoint(point, metricIdentifier)
		if normalizedPoint == nil {
			return nil
		}
		point = *normalizedPoint
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
			Type: t,
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
	metric pmetric.Metric,
	exponentialHist pmetric.ExponentialHistogram,
	point pmetric.ExponentialHistogramDataPoint,
) []*monitoringpb.TimeSeries {
	if point.Flags().HasFlag(pmetric.MetricDataPointFlagNoRecordedValue) {
		// Drop points without a value.
		return nil
	}
	t, err := m.metricNameToType(metric.Name(), metric)
	if err != nil {
		m.obs.log.Debug("Failed to get metric type (i.e. name) for exponential histogram metric. Dropping the metric.", zap.Error(err), zap.Any("metric", metric))
		return nil
	}
	if exponentialHist.AggregationTemporality() == pmetric.MetricAggregationTemporalityCumulative {
		// Normalize the histogram point.
		metricIdentifier := datapointstorage.Identifier(resource, extraLabels, metric, point.Attributes())
		normalizedPoint := m.normalizer.NormalizeExponentialHistogramDataPoint(point, metricIdentifier)
		if normalizedPoint == nil {
			return nil
		}
		point = *normalizedPoint
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
			Type: t,
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
	metric pmetric.Metric,
	sum pmetric.Sum,
	point pmetric.NumberDataPoint,
) []*monitoringpb.TimeSeries {
	metricKind := metricpb.MetricDescriptor_CUMULATIVE
	var startTime *timestamppb.Timestamp
	if point.Flags().HasFlag(pmetric.MetricDataPointFlagNoRecordedValue) {
		// Drop points without a value.  This may be a staleness marker from
		// prometheus.
		return nil
	}
	t, err := m.metricNameToType(metric.Name(), metric)
	if err != nil {
		m.obs.log.Debug("Failed to get metric type (i.e. name) for sum metric. Dropping the metric.", zap.Error(err), zap.Any("metric", metric))
		return nil
	}
	if sum.IsMonotonic() {
		if sum.AggregationTemporality() == pmetric.MetricAggregationTemporalityCumulative {
			metricIdentifier := datapointstorage.Identifier(resource, extraLabels, metric, point.Attributes())
			normalizedPoint := m.normalizer.NormalizeNumberDataPoint(point, metricIdentifier)
			if normalizedPoint == nil {
				return nil
			}
			point = *normalizedPoint
		}
		startTime = timestamppb.New(point.StartTimestamp().AsTime())
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
			Type: t,
			Labels: mergeLabels(
				attributesToLabels(point.Attributes()),
				extraLabels,
			),
		},
	}}
}

func (m *metricMapper) gaugePointToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pmetric.Metric,
	gauge pmetric.Gauge,
	point pmetric.NumberDataPoint,
) []*monitoringpb.TimeSeries {
	if point.Flags().HasFlag(pmetric.MetricDataPointFlagNoRecordedValue) {
		// Drop points without a value.
		return nil
	}
	t, err := m.metricNameToType(metric.Name(), metric)
	if err != nil {
		m.obs.log.Debug("Unable to get metric type (i.e. name) for gauge metric.", zap.Error(err), zap.Any("metric", metric))
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
			Type: t,
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
func (m *metricMapper) metricNameToType(name string, metric pmetric.Metric) (string, error) {
	metricName, err := m.cfg.MetricConfig.GetMetricName(name, metric)
	if err != nil {
		return "", err
	}
	return path.Join(m.getMetricNamePrefix(name), metricName), nil
}

// defaultGetMetricName does not (further) customize the baseName
func defaultGetMetricName(baseName string, _ pmetric.Metric) (string, error) {
	return baseName, nil
}

func numberDataPointToValue(
	point pmetric.NumberDataPoint,
) (*monitoringpb.TypedValue, metricpb.MetricDescriptor_ValueType) {
	if point.ValueType() == pmetric.NumberDataPointValueTypeInt {
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

func attributesToLabels(attrs pcommon.Map) labels {
	ls := make(labels, attrs.Len())
	attrs.Range(func(k string, v pcommon.Value) bool {
		ls[sanitizeKey(k)] = sanitizeUTF8(v.AsString())
		return true
	})
	return ls
}

func sanitizeUTF8(s string) string {
	return strings.ToValidUTF8(s, "�")
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
	pm pmetric.Metric,
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
	addAttributes := func(attr pcommon.Map) {
		attr.Range(func(key string, _ pcommon.Value) bool {
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
	case pmetric.MetricDataTypeGauge:
		points := pm.Gauge().DataPoints()
		for i := 0; i < points.Len(); i++ {
			addAttributes(points.At(i).Attributes())
		}
	case pmetric.MetricDataTypeSum:
		points := pm.Sum().DataPoints()
		for i := 0; i < points.Len(); i++ {
			addAttributes(points.At(i).Attributes())
		}
	case pmetric.MetricDataTypeSummary:
		points := pm.Summary().DataPoints()
		for i := 0; i < points.Len(); i++ {
			addAttributes(points.At(i).Attributes())
		}
	case pmetric.MetricDataTypeHistogram:
		points := pm.Histogram().DataPoints()
		for i := 0; i < points.Len(); i++ {
			addAttributes(points.At(i).Attributes())
		}
	case pmetric.MetricDataTypeExponentialHistogram:
		points := pm.ExponentialHistogram().DataPoints()
		for i := 0; i < points.Len(); i++ {
			addAttributes(points.At(i).Attributes())
		}
	}
	return result
}

// Returns (sum, count, quantile) metric types (i.e. names) for a summary metric.
func (m *metricMapper) summaryMetricTypes(pm pmetric.Metric) (string, string, string, error) {
	sumType, err := m.metricNameToType(pm.Name()+SummarySumSuffix, pm)
	if err != nil {
		return "", "", "", err
	}
	countType, err := m.metricNameToType(pm.Name()+SummaryCountPrefix, pm)
	if err != nil {
		return "", "", "", err
	}
	quantileType, err := m.metricNameToType(pm.Name(), pm)
	if err != nil {
		return "", "", "", err
	}
	return sumType, countType, quantileType, nil
}

func (m *metricMapper) summaryMetricDescriptors(
	pm pmetric.Metric,
	extraLabels labels,
) []*metricpb.MetricDescriptor {
	sumType, countType, quantileType, err := m.summaryMetricTypes(pm)
	if err != nil {
		m.obs.log.Debug("Failed to get metric types (i.e. names) for summary metric. Dropping the metric.", zap.Error(err), zap.Any("metric", pm))
		return nil
	}
	labels := m.labelDescriptors(pm, extraLabels)
	return []*metricpb.MetricDescriptor{
		{
			Type:        sumType,
			Labels:      labels,
			MetricKind:  metricpb.MetricDescriptor_CUMULATIVE,
			ValueType:   metricpb.MetricDescriptor_DOUBLE,
			Unit:        pm.Unit(),
			Description: pm.Description(),
			DisplayName: pm.Name() + SummarySumSuffix,
		},
		{
			Type:        countType,
			Labels:      labels,
			MetricKind:  metricpb.MetricDescriptor_CUMULATIVE,
			ValueType:   metricpb.MetricDescriptor_DOUBLE,
			Unit:        pm.Unit(),
			Description: pm.Description(),
			DisplayName: pm.Name() + SummaryCountPrefix,
		},
		{
			Type: quantileType,
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
	pm pmetric.Metric,
	extraLabels labels,
) []*metricpb.MetricDescriptor {
	if pm.DataType() == pmetric.MetricDataTypeSummary {
		return m.summaryMetricDescriptors(pm, extraLabels)
	}
	kind, typ := mapMetricPointKind(pm)
	metricType, err := m.metricNameToType(pm.Name(), pm)
	if err != nil {
		m.obs.log.Debug("Failed to get metric type (i.e. name) for metric descriptor. Dropping the metric descriptor.", zap.Error(err), zap.Any("metric", pm))
		return nil
	}
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

func metricPointValueType(pt pmetric.NumberDataPointValueType) metricpb.MetricDescriptor_ValueType {
	switch pt {
	case pmetric.NumberDataPointValueTypeInt:
		return metricpb.MetricDescriptor_INT64
	case pmetric.NumberDataPointValueTypeDouble:
		return metricpb.MetricDescriptor_DOUBLE
	default:
		return metricpb.MetricDescriptor_VALUE_TYPE_UNSPECIFIED
	}
}

func mapMetricPointKind(m pmetric.Metric) (metricpb.MetricDescriptor_MetricKind, metricpb.MetricDescriptor_ValueType) {
	var kind metricpb.MetricDescriptor_MetricKind
	var typ metricpb.MetricDescriptor_ValueType
	switch m.DataType() {
	case pmetric.MetricDataTypeGauge:
		kind = metricpb.MetricDescriptor_GAUGE
		if m.Gauge().DataPoints().Len() > 0 {
			typ = metricPointValueType(m.Gauge().DataPoints().At(0).ValueType())
		}
	case pmetric.MetricDataTypeSum:
		if !m.Sum().IsMonotonic() {
			kind = metricpb.MetricDescriptor_GAUGE
		} else if m.Sum().AggregationTemporality() == pmetric.MetricAggregationTemporalityDelta {
			// We report fake-deltas for now.
			kind = metricpb.MetricDescriptor_CUMULATIVE
		} else {
			kind = metricpb.MetricDescriptor_CUMULATIVE
		}
		if m.Sum().DataPoints().Len() > 0 {
			typ = metricPointValueType(m.Sum().DataPoints().At(0).ValueType())
		}
	case pmetric.MetricDataTypeSummary:
		kind = metricpb.MetricDescriptor_GAUGE
	case pmetric.MetricDataTypeHistogram:
		typ = metricpb.MetricDescriptor_DISTRIBUTION
		if m.Histogram().AggregationTemporality() == pmetric.MetricAggregationTemporalityDelta {
			// We report fake-deltas for now.
			kind = metricpb.MetricDescriptor_CUMULATIVE
		} else {
			kind = metricpb.MetricDescriptor_CUMULATIVE
		}
	case pmetric.MetricDataTypeExponentialHistogram:
		typ = metricpb.MetricDescriptor_DISTRIBUTION
		if m.ExponentialHistogram().AggregationTemporality() == pmetric.MetricAggregationTemporalityDelta {
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

func (me *MetricsExporter) processItem(ts *monitoringpb.TimeSeries) {
}
