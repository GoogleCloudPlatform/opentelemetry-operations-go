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

package collector

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/api/label"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/datapointstorage"
)

var (
	start = time.Unix(1000, 1000)
)

func newTestMetricMapper() (metricMapper, func()) {
	obs := selfObservability{log: zap.NewNop()}
	s := make(chan struct{})
	return metricMapper{
		obs,
		DefaultConfig(),
		datapointstorage.NewCache(s),
	}, func() { close(s) }
}

func TestMetricToTimeSeries(t *testing.T) {
	mr := &monitoredrespb.MonitoredResource{}

	t.Run("Sum", func(t *testing.T) {
		mapper, shutdown := newTestMetricMapper()
		defer shutdown()
		metric := pdata.NewMetric()
		metric.SetDataType(pmetric.MetricDataTypeSum)
		sum := metric.Sum()
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
		startTs := pdata.NewTimestampFromTime(start)
		endTs := pdata.NewTimestampFromTime(start.Add(time.Hour))
		// Add three points
		point := sum.DataPoints().AppendEmpty()
		point.SetDoubleVal(10)
		point.SetStartTimestamp(startTs)
		point.SetTimestamp(endTs)
		point = sum.DataPoints().AppendEmpty()
		point.SetDoubleVal(15)
		point.SetStartTimestamp(startTs)
		point.SetTimestamp(endTs)
		point = sum.DataPoints().AppendEmpty()
		point.SetDoubleVal(16)
		point.SetStartTimestamp(startTs)
		point.SetTimestamp(endTs)

		ts := mapper.metricToTimeSeries(
			mr,
			labels{},
			metric,
		)
		require.Len(t, ts, 3, "Should create one timeseries for each sum point")
		require.Same(t, ts[0].Resource, mr, "Should assign the passed in monitored resource")
	})

	t.Run("Sum without timestamps", func(t *testing.T) {
		mapper, shutdown := newTestMetricMapper()
		defer shutdown()
		metric := pdata.NewMetric()
		metric.SetDataType(pmetric.MetricDataTypeSum)
		sum := metric.Sum()
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
		zeroTs := pdata.Timestamp(0)
		startTs := pdata.NewTimestampFromTime(start)
		endTs := pdata.NewTimestampFromTime(start.Add(time.Hour))
		// Add three points without start timestamps.
		// The first one should be dropped to set the start timestamp for the rest
		point := sum.DataPoints().AppendEmpty()
		point.SetDoubleVal(10)
		point.SetStartTimestamp(zeroTs)
		point.SetTimestamp(startTs)
		point = sum.DataPoints().AppendEmpty()
		point.SetDoubleVal(15)
		point.SetStartTimestamp(zeroTs)
		point.SetTimestamp(endTs)
		point = sum.DataPoints().AppendEmpty()
		point.SetDoubleVal(16)
		point.SetStartTimestamp(zeroTs)
		point.SetTimestamp(endTs)

		ts := mapper.metricToTimeSeries(
			mr,
			labels{},
			metric,
		)
		require.Len(t, ts, 2, "Should create one timeseries for each sum point")
		require.Same(t, ts[0].Resource, mr, "Should assign the passed in monitored resource")
		require.Equal(t, ts[0].Points[0].Value.GetDoubleValue(), 5.0, "Should normalize the resulting sum")
		require.Equal(t, ts[0].Points[0].Interval.StartTime, timestamppb.New(start), "Should use the first timestamp as the start time for the rest of the points")
	})

	t.Run("Sum with reset timestamp", func(t *testing.T) {
		mapper, shutdown := newTestMetricMapper()
		defer shutdown()
		metric := pdata.NewMetric()
		metric.SetDataType(pmetric.MetricDataTypeSum)
		sum := metric.Sum()
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
		startTs := pdata.NewTimestampFromTime(start)
		middleTs := pdata.NewTimestampFromTime(start.Add(30 * time.Minute))
		endTs := pdata.NewTimestampFromTime(start.Add(time.Hour))
		// Add three points
		point := sum.DataPoints().AppendEmpty()
		point.SetDoubleVal(10)
		point.SetStartTimestamp(startTs)
		point.SetTimestamp(middleTs)
		point = sum.DataPoints().AppendEmpty()
		point.SetDoubleVal(15)
		// identical start and end indicates a reset point
		// this point is expected to be dropped, and to normalize subsequent points
		point.SetStartTimestamp(middleTs)
		point.SetTimestamp(middleTs)
		point = sum.DataPoints().AppendEmpty()
		point.SetDoubleVal(16)
		point.SetStartTimestamp(middleTs)
		point.SetTimestamp(endTs)

		ts := mapper.metricToTimeSeries(
			mr,
			labels{},
			metric,
		)
		require.Len(t, ts, 2, "Should create one timeseries for each sum point")
		require.Same(t, ts[0].Resource, mr, "Should assign the passed in monitored resource")
		require.Equal(t, ts[1].Points[0].Value.GetDoubleValue(), 1.0, "Should normalize the point after the reset")
	})

	t.Run("Sum with no value", func(t *testing.T) {
		mapper, shutdown := newTestMetricMapper()
		defer shutdown()
		metric := pdata.NewMetric()
		metric.SetDataType(pmetric.MetricDataTypeSum)
		sum := metric.Sum()
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
		startTs := pdata.NewTimestampFromTime(start)
		endTs := pdata.NewTimestampFromTime(start.Add(time.Hour))
		// Add three points
		point := sum.DataPoints().AppendEmpty()
		point.SetDoubleVal(10)
		point.SetStartTimestamp(startTs)
		point.SetTimestamp(endTs)
		point = sum.DataPoints().AppendEmpty()
		point.SetDoubleVal(15)
		point.SetStartTimestamp(startTs)
		point.SetTimestamp(endTs)
		// The last point has no value
		point = sum.DataPoints().AppendEmpty()
		point.SetFlags(pmetric.MetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue))

		ts := mapper.metricToTimeSeries(
			mr,
			labels{},
			metric,
		)
		require.Len(t, ts, 2, "Should create one timeseries for each sum point, but omit the stale point")
		require.Same(t, ts[0].Resource, mr, "Should assign the passed in monitored resource")
	})

	t.Run("Gauge", func(t *testing.T) {
		mapper, shutdown := newTestMetricMapper()
		defer shutdown()
		metric := pdata.NewMetric()
		metric.SetDataType(pmetric.MetricDataTypeGauge)
		gauge := metric.Gauge()
		// Add three points
		gauge.DataPoints().AppendEmpty().SetIntVal(10)
		gauge.DataPoints().AppendEmpty().SetIntVal(15)
		gauge.DataPoints().AppendEmpty().SetIntVal(16)

		ts := mapper.metricToTimeSeries(
			mr,
			labels{},
			metric,
		)
		require.Len(t, ts, 3, "Should create one timeseries for each gauge point")
		require.Same(t, ts[0].Resource, mr, "Should assign the passed in monitored resource")
	})

	t.Run("Gauge with no value", func(t *testing.T) {
		mapper, shutdown := newTestMetricMapper()
		defer shutdown()
		metric := pdata.NewMetric()
		metric.SetDataType(pmetric.MetricDataTypeGauge)
		gauge := metric.Gauge()
		// Add three points
		gauge.DataPoints().AppendEmpty().SetIntVal(10)
		gauge.DataPoints().AppendEmpty().SetIntVal(15)
		// The last point has no value
		gauge.DataPoints().AppendEmpty().SetFlags(pmetric.MetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue))

		ts := mapper.metricToTimeSeries(
			mr,
			labels{},
			metric,
		)
		require.Len(t, ts, 2, "Should create one timeseries for each gauge point, except the point without a value")
		require.Same(t, ts[0].Resource, mr, "Should assign the passed in monitored resource")
	})
}

func TestMergeLabels(t *testing.T) {
	assert.NotNil(t, mergeLabels(nil, labels{}), "Should create labels if mergeInto is nil")

	assert.Equal(
		t,
		mergeLabels(labels{"foo": "foo"}, labels{"bar": "bar"}),
		labels{"foo": "foo", "bar": "bar"},
		"Should merge disjoint labels",
	)

	assert.Equal(
		t,
		mergeLabels(labels{"foo": "foo", "baz": "baz"}, labels{"foo": "bar"}),
		labels{"foo": "bar", "baz": "baz"},
		"Should merge intersecting labels, keeping the last value",
	)
}

func TestHistogramPointToTimeSeries(t *testing.T) {
	mapper, shutdown := newTestMetricMapper()
	defer shutdown()
	mapper.cfg.ProjectID = "myproject"
	mr := &monitoredrespb.MonitoredResource{}
	metric := pdata.NewMetric()
	metric.SetName("myhist")
	metric.SetDataType(pmetric.MetricDataTypeHistogram)
	unit := "1"
	metric.SetUnit(unit)
	hist := metric.Histogram()
	point := hist.DataPoints().AppendEmpty()
	end := start.Add(time.Hour)
	point.SetStartTimestamp(pdata.NewTimestampFromTime(start))
	point.SetTimestamp(pdata.NewTimestampFromTime(end))
	point.SetBucketCounts([]uint64{1, 2, 3, 4, 5})
	point.SetCount(15)
	point.SetSum(42)
	point.SetExplicitBounds([]float64{10, 20, 30, 40})
	exemplar := point.Exemplars().AppendEmpty()
	exemplar.SetDoubleVal(2)
	exemplar.SetTimestamp(pdata.NewTimestampFromTime(end))
	exemplar.SetTraceID(pdata.NewTraceID([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 1, 2, 3, 4, 5, 6}))
	exemplar.SetSpanID(pdata.NewSpanID([8]byte{0, 1, 2, 3, 4, 5, 6, 7}))
	exemplar.FilteredAttributes().InsertString("test", "extra")

	tsl := mapper.histogramToTimeSeries(mr, labels{}, metric, hist, point)
	assert.Len(t, tsl, 1)
	ts := tsl[0]
	// Verify aspects
	assert.Equal(t, metricpb.MetricDescriptor_CUMULATIVE, ts.MetricKind)
	assert.Equal(t, metricpb.MetricDescriptor_DISTRIBUTION, ts.ValueType)
	assert.Equal(t, unit, ts.Unit)
	assert.Same(t, mr, ts.Resource)

	assert.Equal(t, "workload.googleapis.com/myhist", ts.Metric.Type)
	assert.Equal(t, map[string]string{}, ts.Metric.Labels)

	assert.Nil(t, ts.Metadata)

	assert.Len(t, ts.Points, 1)
	assert.Equal(t, &monitoringpb.TimeInterval{
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(end),
	}, ts.Points[0].Interval)
	hdp := ts.Points[0].Value.GetDistributionValue()
	assert.Equal(t, int64(15), hdp.Count)
	assert.ElementsMatch(t, []int64{1, 2, 3, 4, 5}, hdp.BucketCounts)
	assert.Equal(t, float64(12847.600000000002), hdp.SumOfSquaredDeviation)
	assert.Equal(t, float64(2.8), hdp.Mean)
	assert.Equal(t, []float64{10, 20, 30, 40}, hdp.BucketOptions.GetExplicitBuckets().Bounds)
	assert.Len(t, hdp.Exemplars, 1)
	ex := hdp.Exemplars[0]
	assert.Equal(t, float64(2), ex.Value)
	assert.Equal(t, timestamppb.New(end), ex.Timestamp)
	// We should see trace + dropped labels
	assert.Len(t, ex.Attachments, 2)
	spanctx := &monitoringpb.SpanContext{}
	err := ex.Attachments[0].UnmarshalTo(spanctx)
	assert.Nil(t, err)
	assert.Equal(t, "projects/myproject/traces/00010203040506070809010203040506/spans/0001020304050607", spanctx.SpanName)
	dropped := &monitoringpb.DroppedLabels{}
	err = ex.Attachments[1].UnmarshalTo(dropped)
	assert.Nil(t, err)
	assert.Equal(t, map[string]string{"test": "extra"}, dropped.Label)
}

func TestNoValueHistogramPointToTimeSeries(t *testing.T) {
	mapper, shutdown := newTestMetricMapper()
	defer shutdown()
	mapper.cfg.ProjectID = "myproject"
	mr := &monitoredrespb.MonitoredResource{}
	metric := pdata.NewMetric()
	metric.SetName("myhist")
	metric.SetDataType(pmetric.MetricDataTypeHistogram)
	unit := "1"
	metric.SetUnit(unit)
	hist := metric.Histogram()
	point := hist.DataPoints().AppendEmpty()
	point.SetFlags(pmetric.MetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue))
	end := start.Add(time.Hour)
	point.SetStartTimestamp(pdata.NewTimestampFromTime(start))
	point.SetTimestamp(pdata.NewTimestampFromTime(end))
	point.SetBucketCounts([]uint64{1, 2, 3, 4, 5})
	point.SetCount(15)
	point.SetSum(42)
	point.SetExplicitBounds([]float64{10, 20, 30, 40})

	tsl := mapper.histogramToTimeSeries(mr, labels{}, metric, hist, point)
	// Points without a value are dropped
	assert.Len(t, tsl, 0)
}

func TestNoSumHistogramPointToTimeSeries(t *testing.T) {
	mapper, shutdown := newTestMetricMapper()
	defer shutdown()
	mapper.cfg.ProjectID = "myproject"
	mr := &monitoredrespb.MonitoredResource{}
	metric := pdata.NewMetric()
	metric.SetName("myhist")
	metric.SetDataType(pmetric.MetricDataTypeHistogram)
	unit := "1"
	metric.SetUnit(unit)
	hist := metric.Histogram()
	point := hist.DataPoints().AppendEmpty()
	end := start.Add(time.Hour)
	point.SetStartTimestamp(pdata.NewTimestampFromTime(start))
	point.SetTimestamp(pdata.NewTimestampFromTime(end))
	point.SetBucketCounts([]uint64{1, 2, 3, 4, 5})
	point.SetCount(15)
	// Leave the sum unset
	point.SetExplicitBounds([]float64{10, 20, 30, 40})

	tsl := mapper.histogramToTimeSeries(mr, labels{}, metric, hist, point)
	// Points without a sum are dropped
	assert.Len(t, tsl, 0)
}

func TestEmptyHistogramPointToTimeSeries(t *testing.T) {
	mapper, shutdown := newTestMetricMapper()
	defer shutdown()
	mapper.cfg.ProjectID = "myproject"
	mr := &monitoredrespb.MonitoredResource{}
	metric := pdata.NewMetric()
	metric.SetName("myhist")
	metric.SetDataType(pmetric.MetricDataTypeHistogram)
	unit := "1"
	metric.SetUnit(unit)
	hist := metric.Histogram()
	point := hist.DataPoints().AppendEmpty()
	end := start.Add(time.Hour)
	point.SetStartTimestamp(pdata.NewTimestampFromTime(start))
	point.SetTimestamp(pdata.NewTimestampFromTime(end))
	point.SetBucketCounts([]uint64{0, 0, 0, 0, 0})
	point.SetCount(0)
	point.SetSum(0)
	point.SetExplicitBounds([]float64{10, 20, 30, 40})

	tsl := mapper.histogramToTimeSeries(mr, labels{}, metric, hist, point)
	assert.Len(t, tsl, 1)
	ts := tsl[0]
	// Verify aspects
	assert.Equal(t, metricpb.MetricDescriptor_CUMULATIVE, ts.MetricKind)
	assert.Equal(t, metricpb.MetricDescriptor_DISTRIBUTION, ts.ValueType)
	assert.Equal(t, unit, ts.Unit)
	assert.Same(t, mr, ts.Resource)

	assert.Equal(t, "workload.googleapis.com/myhist", ts.Metric.Type)
	assert.Equal(t, map[string]string{}, ts.Metric.Labels)

	assert.Nil(t, ts.Metadata)

	assert.Len(t, ts.Points, 1)
	assert.Equal(t, &monitoringpb.TimeInterval{
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(end),
	}, ts.Points[0].Interval)
	hdp := ts.Points[0].Value.GetDistributionValue()
	assert.Equal(t, int64(0), hdp.Count)
	assert.ElementsMatch(t, []int64{0, 0, 0, 0, 0}, hdp.BucketCounts)
	// NaN sum produces a mean of 0
	assert.Equal(t, float64(0), hdp.Mean)
	assert.Equal(t, []float64{10, 20, 30, 40}, hdp.BucketOptions.GetExplicitBuckets().Bounds)
}

func TestNaNSumHistogramPointToTimeSeries(t *testing.T) {
	mapper, shutdown := newTestMetricMapper()
	defer shutdown()
	mapper.cfg.ProjectID = "myproject"
	mr := &monitoredrespb.MonitoredResource{}
	metric := pdata.NewMetric()
	metric.SetName("myhist")
	metric.SetDataType(pmetric.MetricDataTypeHistogram)
	unit := "1"
	metric.SetUnit(unit)
	hist := metric.Histogram()
	point := hist.DataPoints().AppendEmpty()
	end := start.Add(time.Hour)
	point.SetStartTimestamp(pdata.NewTimestampFromTime(start))
	point.SetTimestamp(pdata.NewTimestampFromTime(end))
	point.SetBucketCounts([]uint64{1, 2, 3, 4, 5})
	point.SetCount(15)
	point.SetSum(math.NaN())
	point.SetExplicitBounds([]float64{10, 20, 30, 40})

	tsl := mapper.histogramToTimeSeries(mr, labels{}, metric, hist, point)
	assert.Len(t, tsl, 1)
	ts := tsl[0]
	// Verify aspects
	assert.Equal(t, metricpb.MetricDescriptor_CUMULATIVE, ts.MetricKind)
	assert.Equal(t, metricpb.MetricDescriptor_DISTRIBUTION, ts.ValueType)
	assert.Equal(t, unit, ts.Unit)
	assert.Same(t, mr, ts.Resource)

	assert.Equal(t, "workload.googleapis.com/myhist", ts.Metric.Type)
	assert.Equal(t, map[string]string{}, ts.Metric.Labels)

	assert.Nil(t, ts.Metadata)

	assert.Len(t, ts.Points, 1)
	assert.Equal(t, &monitoringpb.TimeInterval{
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(end),
	}, ts.Points[0].Interval)
	hdp := ts.Points[0].Value.GetDistributionValue()
	assert.Equal(t, int64(15), hdp.Count)
	assert.ElementsMatch(t, []int64{1, 2, 3, 4, 5}, hdp.BucketCounts)
	// NaN sum produces a mean of 0
	assert.Equal(t, float64(0), hdp.Mean)
	assert.Equal(t, []float64{10, 20, 30, 40}, hdp.BucketOptions.GetExplicitBuckets().Bounds)
}

func TestExponentialHistogramPointToTimeSeries(t *testing.T) {
	mapper, shutdown := newTestMetricMapper()
	defer shutdown()
	mapper.cfg.ProjectID = "myproject"
	mr := &monitoredrespb.MonitoredResource{}
	metric := pdata.NewMetric()
	metric.SetName("myexphist")
	metric.SetDataType(pmetric.MetricDataTypeExponentialHistogram)
	unit := "1"
	metric.SetUnit(unit)
	hist := metric.ExponentialHistogram()
	point := hist.DataPoints().AppendEmpty()
	end := start.Add(time.Hour)
	point.SetStartTimestamp(pdata.NewTimestampFromTime(start))
	point.SetTimestamp(pdata.NewTimestampFromTime(end))
	point.Positive().SetBucketCounts([]uint64{1, 2, 3})
	point.Negative().SetBucketCounts([]uint64{4, 5})
	point.SetZeroCount(6)
	point.SetCount(21)
	point.SetScale(-1)
	point.SetSum(42)
	exemplar := point.Exemplars().AppendEmpty()
	exemplar.SetDoubleVal(2)
	exemplar.SetTimestamp(pdata.NewTimestampFromTime(end))
	exemplar.SetTraceID(pdata.NewTraceID([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 1, 2, 3, 4, 5, 6}))
	exemplar.SetSpanID(pdata.NewSpanID([8]byte{0, 1, 2, 3, 4, 5, 6, 7}))
	exemplar.FilteredAttributes().InsertString("test", "extra")

	// Add a second point with no value
	hist.DataPoints().AppendEmpty().SetFlags(pmetric.MetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue))

	tsl := mapper.exponentialHistogramToTimeSeries(mr, labels{}, metric, hist, point)
	assert.Len(t, tsl, 1)
	ts := tsl[0]
	// Verify aspects
	assert.Equal(t, metricpb.MetricDescriptor_CUMULATIVE, ts.MetricKind)
	assert.Equal(t, metricpb.MetricDescriptor_DISTRIBUTION, ts.ValueType)
	assert.Equal(t, unit, ts.Unit)
	assert.Same(t, mr, ts.Resource)

	assert.Equal(t, "workload.googleapis.com/myexphist", ts.Metric.Type)
	assert.Equal(t, map[string]string{}, ts.Metric.Labels)

	assert.Nil(t, ts.Metadata)

	assert.Len(t, ts.Points, 1)
	assert.Equal(t, &monitoringpb.TimeInterval{
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(end),
	}, ts.Points[0].Interval)
	hdp := ts.Points[0].Value.GetDistributionValue()
	assert.Equal(t, int64(21), hdp.Count)
	assert.ElementsMatch(t, []int64{15, 1, 2, 3, 0}, hdp.BucketCounts)
	assert.Equal(t, float64(2), hdp.Mean)
	assert.Equal(t, float64(4), hdp.BucketOptions.GetExponentialBuckets().GrowthFactor)
	assert.Equal(t, float64(1), hdp.BucketOptions.GetExponentialBuckets().Scale)
	assert.Equal(t, int32(3), hdp.BucketOptions.GetExponentialBuckets().NumFiniteBuckets)
	assert.Len(t, hdp.Exemplars, 1)
	ex := hdp.Exemplars[0]
	assert.Equal(t, float64(2), ex.Value)
	assert.Equal(t, timestamppb.New(end), ex.Timestamp)
	// We should see trace + dropped labels
	assert.Len(t, ex.Attachments, 2)
	spanctx := &monitoringpb.SpanContext{}
	err := ex.Attachments[0].UnmarshalTo(spanctx)
	assert.Nil(t, err)
	assert.Equal(t, "projects/myproject/traces/00010203040506070809010203040506/spans/0001020304050607", spanctx.SpanName)
	dropped := &monitoringpb.DroppedLabels{}
	err = ex.Attachments[1].UnmarshalTo(dropped)
	assert.Nil(t, err)
	assert.Equal(t, map[string]string{"test": "extra"}, dropped.Label)
}

func TestNaNSumExponentialHistogramPointToTimeSeries(t *testing.T) {
	mapper, shutdown := newTestMetricMapper()
	defer shutdown()
	mapper.cfg.ProjectID = "myproject"
	mr := &monitoredrespb.MonitoredResource{}
	metric := pdata.NewMetric()
	metric.SetName("myexphist")
	metric.SetDataType(pmetric.MetricDataTypeExponentialHistogram)
	unit := "1"
	metric.SetUnit(unit)
	hist := metric.ExponentialHistogram()
	point := hist.DataPoints().AppendEmpty()
	end := start.Add(time.Hour)
	point.SetStartTimestamp(pdata.NewTimestampFromTime(start))
	point.SetTimestamp(pdata.NewTimestampFromTime(end))
	point.Positive().SetBucketCounts([]uint64{1, 2, 3})
	point.Negative().SetBucketCounts([]uint64{4, 5})
	point.SetZeroCount(6)
	point.SetCount(21)
	point.SetScale(-1)
	point.SetSum(math.NaN())

	tsl := mapper.exponentialHistogramToTimeSeries(mr, labels{}, metric, hist, point)
	assert.Len(t, tsl, 1)
	ts := tsl[0]
	// Verify aspects
	assert.Equal(t, metricpb.MetricDescriptor_CUMULATIVE, ts.MetricKind)
	assert.Equal(t, metricpb.MetricDescriptor_DISTRIBUTION, ts.ValueType)
	assert.Equal(t, unit, ts.Unit)
	assert.Same(t, mr, ts.Resource)

	assert.Equal(t, "workload.googleapis.com/myexphist", ts.Metric.Type)
	assert.Equal(t, map[string]string{}, ts.Metric.Labels)

	assert.Nil(t, ts.Metadata)

	assert.Len(t, ts.Points, 1)
	assert.Equal(t, &monitoringpb.TimeInterval{
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(end),
	}, ts.Points[0].Interval)
	hdp := ts.Points[0].Value.GetDistributionValue()
	assert.Equal(t, int64(21), hdp.Count)
	assert.ElementsMatch(t, []int64{15, 1, 2, 3, 0}, hdp.BucketCounts)
	assert.Equal(t, float64(0), hdp.Mean)
	assert.Equal(t, float64(4), hdp.BucketOptions.GetExponentialBuckets().GrowthFactor)
	assert.Equal(t, float64(1), hdp.BucketOptions.GetExponentialBuckets().Scale)
	assert.Equal(t, int32(3), hdp.BucketOptions.GetExponentialBuckets().NumFiniteBuckets)
}

func TestZeroCountExponentialHistogramPointToTimeSeries(t *testing.T) {
	mapper, shutdown := newTestMetricMapper()
	defer shutdown()
	mapper.cfg.ProjectID = "myproject"
	mr := &monitoredrespb.MonitoredResource{}
	metric := pdata.NewMetric()
	metric.SetName("myexphist")
	metric.SetDataType(pmetric.MetricDataTypeExponentialHistogram)
	unit := "1"
	metric.SetUnit(unit)
	hist := metric.ExponentialHistogram()
	point := hist.DataPoints().AppendEmpty()
	end := start.Add(time.Hour)
	point.SetStartTimestamp(pdata.NewTimestampFromTime(start))
	point.SetTimestamp(pdata.NewTimestampFromTime(end))
	point.Positive().SetBucketCounts([]uint64{0, 0, 0})
	point.Negative().SetBucketCounts([]uint64{0, 0})
	point.SetZeroCount(0)
	point.SetCount(0)
	point.SetScale(-1)
	point.SetSum(0)

	tsl := mapper.exponentialHistogramToTimeSeries(mr, labels{}, metric, hist, point)
	assert.Len(t, tsl, 1)
	ts := tsl[0]
	// Verify aspects
	assert.Equal(t, metricpb.MetricDescriptor_CUMULATIVE, ts.MetricKind)
	assert.Equal(t, metricpb.MetricDescriptor_DISTRIBUTION, ts.ValueType)
	assert.Equal(t, unit, ts.Unit)
	assert.Same(t, mr, ts.Resource)

	assert.Equal(t, "workload.googleapis.com/myexphist", ts.Metric.Type)
	assert.Equal(t, map[string]string{}, ts.Metric.Labels)

	assert.Nil(t, ts.Metadata)

	assert.Len(t, ts.Points, 1)
	assert.Equal(t, &monitoringpb.TimeInterval{
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(end),
	}, ts.Points[0].Interval)
	hdp := ts.Points[0].Value.GetDistributionValue()
	assert.Equal(t, int64(0), hdp.Count)
	assert.ElementsMatch(t, []int64{0, 0, 0, 0, 0}, hdp.BucketCounts)
	assert.Equal(t, float64(0), hdp.Mean)
	assert.Equal(t, float64(4), hdp.BucketOptions.GetExponentialBuckets().GrowthFactor)
	assert.Equal(t, float64(1), hdp.BucketOptions.GetExponentialBuckets().Scale)
	assert.Equal(t, int32(3), hdp.BucketOptions.GetExponentialBuckets().NumFiniteBuckets)
}

func TestExemplarNoAttachements(t *testing.T) {
	mapper, shutdown := newTestMetricMapper()
	defer shutdown()
	exemplar := pdata.NewExemplar()
	exemplar.SetTimestamp(pdata.NewTimestampFromTime(start))
	exemplar.SetDoubleVal(1)

	result := mapper.exemplar(exemplar)
	assert.Equal(t, float64(1), result.Value)
	assert.Equal(t, timestamppb.New(start), result.Timestamp)
	assert.Len(t, result.Attachments, 0)
}

func TestExemplarOnlyDroppedLabels(t *testing.T) {
	mapper, shutdown := newTestMetricMapper()
	defer shutdown()
	exemplar := pdata.NewExemplar()
	exemplar.SetTimestamp(pdata.NewTimestampFromTime(start))
	exemplar.SetDoubleVal(1)
	exemplar.FilteredAttributes().InsertString("test", "drop")

	result := mapper.exemplar(exemplar)
	assert.Equal(t, float64(1), result.Value)
	assert.Equal(t, timestamppb.New(start), result.Timestamp)
	assert.Len(t, result.Attachments, 1)

	dropped := &monitoringpb.DroppedLabels{}
	err := result.Attachments[0].UnmarshalTo(dropped)
	assert.Nil(t, err)
	assert.Equal(t, map[string]string{"test": "drop"}, dropped.Label)
}

func TestExemplarOnlyTraceId(t *testing.T) {
	mapper, shutdown := newTestMetricMapper()
	defer shutdown()
	mapper.cfg.ProjectID = "p"
	exemplar := pdata.NewExemplar()
	exemplar.SetTimestamp(pdata.NewTimestampFromTime(start))
	exemplar.SetDoubleVal(1)
	exemplar.SetTraceID(pdata.NewTraceID([16]byte{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
	}))
	exemplar.SetSpanID(pdata.NewSpanID([8]byte{
		0, 0, 0, 0, 0, 0, 0, 2,
	}))

	result := mapper.exemplar(exemplar)
	assert.Equal(t, float64(1), result.Value)
	assert.Equal(t, timestamppb.New(start), result.Timestamp)
	assert.Len(t, result.Attachments, 1)

	context := &monitoringpb.SpanContext{}
	err := result.Attachments[0].UnmarshalTo(context)
	assert.Nil(t, err)
	assert.Equal(t, "projects/p/traces/00000000000000000000000000000001/spans/0000000000000002", context.SpanName)
}

func TestSumPointToTimeSeries(t *testing.T) {
	mapper, shutdown := newTestMetricMapper()
	defer shutdown()
	mr := &monitoredrespb.MonitoredResource{}

	newCase := func() (pdata.Metric, pdata.Sum, pdata.NumberDataPoint) {
		metric := pdata.NewMetric()
		metric.SetDataType(pmetric.MetricDataTypeSum)
		sum := metric.Sum()
		point := sum.DataPoints().AppendEmpty()
		// Add a second point with no value
		sum.DataPoints().AppendEmpty().SetFlags(pmetric.MetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue))
		return metric, sum, point
	}

	t.Run("Cumulative monotonic", func(t *testing.T) {
		metric, sum, point := newCase()
		metric.SetName("mysum")
		unit := "s"
		metric.SetUnit(unit)
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
		var value int64 = 10
		point.SetIntVal(value)
		end := start.Add(time.Hour)
		point.SetStartTimestamp(pdata.NewTimestampFromTime(start))
		point.SetTimestamp(pdata.NewTimestampFromTime(end))

		tsl := mapper.sumPointToTimeSeries(mr, labels{}, metric, sum, point)
		assert.Equal(t, 1, len(tsl))
		ts := tsl[0]
		assert.Equal(t, ts.MetricKind, metricpb.MetricDescriptor_CUMULATIVE)
		assert.Equal(t, ts.ValueType, metricpb.MetricDescriptor_INT64)
		assert.Equal(t, ts.Unit, unit)
		assert.Same(t, ts.Resource, mr)

		assert.Equal(t, ts.Metric.Type, "workload.googleapis.com/mysum")
		assert.Equal(t, ts.Metric.Labels, map[string]string{})

		assert.Nil(t, ts.Metadata)

		assert.Len(t, ts.Points, 1)
		assert.Equal(t, ts.Points[0].Interval, &monitoringpb.TimeInterval{
			StartTime: timestamppb.New(start),
			EndTime:   timestamppb.New(end),
		})
		assert.Equal(t, ts.Points[0].Value.GetInt64Value(), value)

		// Test double as well
		point.SetDoubleVal(float64(value))
		tsl = mapper.sumPointToTimeSeries(mr, labels{}, metric, sum, point)
		assert.Equal(t, 1, len(tsl))
		ts = tsl[0]
		assert.Equal(t, ts.MetricKind, metricpb.MetricDescriptor_CUMULATIVE)
		assert.Equal(t, ts.ValueType, metricpb.MetricDescriptor_DOUBLE)
		assert.Equal(t, ts.Points[0].Value.GetDoubleValue(), float64(value))
	})

	t.Run("Delta monotonic", func(t *testing.T) {
		metric, sum, point := newCase()
		metric.SetName("mysum")
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
		point.SetIntVal(10)
		end := start.Add(time.Hour)
		point.SetStartTimestamp(pdata.NewTimestampFromTime(start))
		point.SetTimestamp(pdata.NewTimestampFromTime(end))

		// Should output a "pseudo-cumulative" with same interval as the delta
		tsl := mapper.sumPointToTimeSeries(mr, labels{}, metric, sum, point)
		assert.Equal(t, 1, len(tsl))
		ts := tsl[0]
		assert.Equal(t, ts.MetricKind, metricpb.MetricDescriptor_CUMULATIVE)
		assert.Equal(t, ts.Points[0].Interval, &monitoringpb.TimeInterval{
			StartTime: timestamppb.New(start),
			EndTime:   timestamppb.New(end),
		})
	})

	t.Run("Non monotonic", func(t *testing.T) {
		metric, sum, point := newCase()
		metric.SetName("mysum")
		sum.SetIsMonotonic(false)
		sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
		point.SetIntVal(10)
		end := start.Add(time.Hour)
		point.SetStartTimestamp(pdata.NewTimestampFromTime(start))
		point.SetTimestamp(pdata.NewTimestampFromTime(end))

		// Should output a gauge regardless of temporality, only setting end time
		tsl := mapper.sumPointToTimeSeries(mr, labels{}, metric, sum, point)
		assert.Equal(t, 1, len(tsl))
		ts := tsl[0]
		assert.Equal(t, ts.MetricKind, metricpb.MetricDescriptor_GAUGE)
		assert.Equal(t, ts.Points[0].Interval, &monitoringpb.TimeInterval{
			EndTime: timestamppb.New(end),
		})

		sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
		tsl = mapper.sumPointToTimeSeries(mr, labels{}, metric, sum, point)
		assert.Equal(t, 1, len(tsl))
		ts = tsl[0]
		assert.Equal(t, ts.MetricKind, metricpb.MetricDescriptor_GAUGE)
		assert.Equal(t, ts.Points[0].Interval, &monitoringpb.TimeInterval{
			EndTime: timestamppb.New(end),
		})
	})

	t.Run("Add labels", func(t *testing.T) {
		metric, sum, point := newCase()
		end := start.Add(time.Hour)
		point.SetStartTimestamp(pdata.NewTimestampFromTime(start))
		point.SetTimestamp(pdata.NewTimestampFromTime(end))
		extraLabels := map[string]string{"foo": "bar"}
		tsl := mapper.sumPointToTimeSeries(mr, labels(extraLabels), metric, sum, point)
		assert.Equal(t, 1, len(tsl))
		ts := tsl[0]
		assert.Equal(t, ts.Metric.Labels, extraLabels)

		// Full set of labels
		point.Attributes().InsertString("baz", "bar")
		tsl = mapper.sumPointToTimeSeries(mr, labels(extraLabels), metric, sum, point)
		assert.Equal(t, 1, len(tsl))
		ts = tsl[0]
		assert.Equal(t, ts.Metric.Labels, map[string]string{"foo": "bar", "baz": "bar"})
	})
}

func TestGaugePointToTimeSeries(t *testing.T) {
	mapper, shutdown := newTestMetricMapper()
	defer shutdown()
	mr := &monitoredrespb.MonitoredResource{}

	newCase := func() (pdata.Metric, pdata.Gauge, pdata.NumberDataPoint) {
		metric := pdata.NewMetric()
		metric.SetDataType(pmetric.MetricDataTypeGauge)
		gauge := metric.Gauge()
		point := gauge.DataPoints().AppendEmpty()
		// Add a second point with no value
		gauge.DataPoints().AppendEmpty().SetFlags(pmetric.MetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue))
		return metric, gauge, point
	}

	metric, gauge, point := newCase()
	metric.SetName("mygauge")
	unit := "1"
	metric.SetUnit(unit)
	var value int64 = 10
	point.SetIntVal(value)
	end := start.Add(time.Hour)
	point.SetTimestamp(pdata.NewTimestampFromTime(end))

	tsl := mapper.gaugePointToTimeSeries(mr, labels{}, metric, gauge, point)
	assert.Len(t, tsl, 1)
	ts := tsl[0]
	assert.Equal(t, ts.MetricKind, metricpb.MetricDescriptor_GAUGE)
	assert.Equal(t, ts.ValueType, metricpb.MetricDescriptor_INT64)
	assert.Equal(t, ts.Unit, unit)
	assert.Same(t, ts.Resource, mr)

	assert.Equal(t, ts.Metric.Type, "workload.googleapis.com/mygauge")
	assert.Equal(t, ts.Metric.Labels, map[string]string{})

	assert.Nil(t, ts.Metadata)

	assert.Len(t, ts.Points, 1)
	assert.Equal(t, ts.Points[0].Interval, &monitoringpb.TimeInterval{
		EndTime: timestamppb.New(end),
	})
	assert.Equal(t, ts.Points[0].Value.GetInt64Value(), value)

	// Test double as well
	point.SetDoubleVal(float64(value))
	tsl = mapper.gaugePointToTimeSeries(mr, labels{}, metric, gauge, point)
	assert.Len(t, tsl, 1)
	ts = tsl[0]
	assert.Equal(t, ts.MetricKind, metricpb.MetricDescriptor_GAUGE)
	assert.Equal(t, ts.ValueType, metricpb.MetricDescriptor_DOUBLE)
	assert.Equal(t, ts.Points[0].Value.GetDoubleValue(), float64(value))

	// Add extra labels
	extraLabels := map[string]string{"foo": "bar"}
	tsl = mapper.gaugePointToTimeSeries(mr, labels(extraLabels), metric, gauge, point)
	assert.Len(t, tsl, 1)
	ts = tsl[0]
	assert.Equal(t, ts.Metric.Labels, extraLabels)

	// Full set of labels
	point.Attributes().InsertString("baz", "bar")
	tsl = mapper.gaugePointToTimeSeries(mr, labels(extraLabels), metric, gauge, point)
	assert.Len(t, tsl, 1)
	ts = tsl[0]
	assert.Equal(t, ts.Metric.Labels, map[string]string{"foo": "bar", "baz": "bar"})
}

func TestSummaryPointToTimeSeries(t *testing.T) {
	mapper, shutdown := newTestMetricMapper()
	defer shutdown()
	mr := &monitoredrespb.MonitoredResource{}

	metric := pdata.NewMetric()
	metric.SetDataType(pmetric.MetricDataTypeSummary)
	summary := metric.Summary()
	point := summary.DataPoints().AppendEmpty()

	metric.SetName("mysummary")
	unit := "1"
	metric.SetUnit(unit)
	var count uint64 = 10
	var sum float64 = 100.0
	point.SetCount(count)
	point.SetSum(sum)
	quantile := point.QuantileValues().AppendEmpty()
	quantile.SetQuantile(1.0)
	quantile.SetValue(1.0)
	end := start.Add(time.Hour)
	point.SetStartTimestamp(pdata.NewTimestampFromTime(start))
	point.SetTimestamp(pdata.NewTimestampFromTime(end))

	// Add a second point with no value
	summary.DataPoints().AppendEmpty().SetFlags(pmetric.MetricDataPointFlags(pmetric.MetricDataPointFlagNoRecordedValue))

	ts := mapper.metricToTimeSeries(mr, labels{}, metric)
	assert.Len(t, ts, 3)
	sumResult := ts[0]
	countResult := ts[1]
	quantileResult := ts[2]

	// Test sum mapping
	assert.Equal(t, sumResult.MetricKind, metricpb.MetricDescriptor_CUMULATIVE)
	assert.Equal(t, sumResult.ValueType, metricpb.MetricDescriptor_DOUBLE)
	assert.Equal(t, sumResult.Unit, unit)
	assert.Same(t, sumResult.Resource, mr)
	assert.Equal(t, sumResult.Metric.Type, "workload.googleapis.com/mysummary_sum")
	assert.Equal(t, sumResult.Metric.Labels, map[string]string{})
	assert.Nil(t, sumResult.Metadata)
	assert.Len(t, sumResult.Points, 1)
	assert.Equal(t, sumResult.Points[0].Interval, &monitoringpb.TimeInterval{
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(end),
	})
	assert.Equal(t, sumResult.Points[0].Value.GetDoubleValue(), sum)
	// Test count mapping
	assert.Equal(t, countResult.MetricKind, metricpb.MetricDescriptor_CUMULATIVE)
	assert.Equal(t, countResult.ValueType, metricpb.MetricDescriptor_DOUBLE)
	assert.Equal(t, countResult.Unit, unit)
	assert.Same(t, countResult.Resource, mr)
	assert.Equal(t, countResult.Metric.Type, "workload.googleapis.com/mysummary_count")
	assert.Equal(t, countResult.Metric.Labels, map[string]string{})
	assert.Nil(t, countResult.Metadata)
	assert.Len(t, countResult.Points, 1)
	assert.Equal(t, countResult.Points[0].Interval, &monitoringpb.TimeInterval{
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(end),
	})
	assert.Equal(t, countResult.Points[0].Value.GetDoubleValue(), float64(count))
	// Test quantile mapping
	assert.Equal(t, quantileResult.MetricKind, metricpb.MetricDescriptor_GAUGE)
	assert.Equal(t, quantileResult.ValueType, metricpb.MetricDescriptor_DOUBLE)
	assert.Equal(t, quantileResult.Unit, unit)
	assert.Same(t, quantileResult.Resource, mr)
	assert.Equal(t, quantileResult.Metric.Type, "workload.googleapis.com/mysummary")
	assert.Equal(t, quantileResult.Metric.Labels, map[string]string{
		"quantile": "1",
	})
	assert.Nil(t, quantileResult.Metadata)
	assert.Len(t, quantileResult.Points, 1)
	assert.Equal(t, quantileResult.Points[0].Interval, &monitoringpb.TimeInterval{
		EndTime: timestamppb.New(end),
	})
	assert.Equal(t, quantileResult.Points[0].Value.GetDoubleValue(), 1.0)
}

func TestMetricNameToType(t *testing.T) {
	mapper, shutdown := newTestMetricMapper()
	defer shutdown()
	assert.Equal(
		t,
		mapper.metricNameToType("foo"),
		"workload.googleapis.com/foo",
		"Should use workload metric domain with default config",
	)
}

func TestAttributesToLabels(t *testing.T) {
	// string to string
	assert.Equal(
		t,
		attributesToLabels(pcommon.NewMapFromRaw(map[string]interface{}{
			"foo": "bar",
			"bar": "baz",
		})),
		labels{"foo": "bar", "bar": "baz"},
	)

	// various key special cases
	assert.Equal(
		t,
		attributesToLabels(pcommon.NewMapFromRaw(map[string]interface{}{
			"foo.bar":   "bar",
			"_foo":      "bar",
			"123.hello": "bar",
			"":          "bar",
		})),
		labels{
			"foo_bar":       "bar",
			"_foo":          "bar",
			"key_123_hello": "bar",
			"":              "bar",
		},
	)

	// value special cases
	assert.Equal(
		t,
		attributesToLabels(pcommon.NewMapFromRaw(map[string]interface{}{
			"a": true,
			"b": false,
			"c": int64(12),
			"d": float64(12.3),
			"e": nil,
			"f": []byte{0xde, 0xad, 0xbe, 0xef},
			"g": []interface{}{"x", nil, "y"},
			"h": map[string]interface{}{"a": "b"},
		})),
		labels{
			"a": "true",
			"b": "false",
			"c": "12",
			"d": "12.3",
			"e": "",
			"f": "3q2+7w==",
			"g": `["x",null,"y"]`,
			"h": `{"a":"b"}`,
		},
	)
}

func TestNumberDataPointToValue(t *testing.T) {
	point := pdata.NewNumberDataPoint()

	point.SetIntVal(12)
	value, valueType := numberDataPointToValue(point)
	assert.Equal(t, valueType, metricpb.MetricDescriptor_INT64)
	assert.EqualValues(t, value.GetInt64Value(), 12)

	point.SetDoubleVal(12.3)
	value, valueType = numberDataPointToValue(point)
	assert.Equal(t, valueType, metricpb.MetricDescriptor_DOUBLE)
	assert.EqualValues(t, value.GetDoubleValue(), 12.3)
}

type metricDescriptorTest struct {
	name          string
	metricCreator func() pdata.Metric
	extraLabels   labels
	expected      []*metricpb.MetricDescriptor
}

func TestMetricDescriptorMapping(t *testing.T) {
	tests := []metricDescriptorTest{
		{
			name: "Gauge",
			metricCreator: func() pdata.Metric {
				metric := pdata.NewMetric()
				metric.SetDataType(pmetric.MetricDataTypeGauge)
				metric.SetName("custom.googleapis.com/test.metric")
				metric.SetDescription("Description")
				metric.SetUnit("1")
				gauge := metric.Gauge()
				point := gauge.DataPoints().AppendEmpty()
				point.SetDoubleVal(10)
				point.Attributes().InsertString("test.label", "test_value")
				return metric
			},
			expected: []*metricpb.MetricDescriptor{
				{
					Name:        "custom.googleapis.com/test.metric",
					DisplayName: "test.metric",
					Type:        "custom.googleapis.com/test.metric",
					MetricKind:  metricpb.MetricDescriptor_GAUGE,
					ValueType:   metricpb.MetricDescriptor_DOUBLE,
					Unit:        "1",
					Description: "Description",
					Labels: []*label.LabelDescriptor{
						{
							Key: "test_label",
						},
					},
				},
			},
		},
		{
			name: "Cumulative Monotonic Sum",
			metricCreator: func() pdata.Metric {
				metric := pdata.NewMetric()
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.SetName("custom.googleapis.com/test.metric")
				metric.SetDescription("Description")
				metric.SetUnit("1")
				sum := metric.Sum()
				sum.SetIsMonotonic(true)
				sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				point := sum.DataPoints().AppendEmpty()
				point.SetDoubleVal(10)
				point.Attributes().InsertString("test.label", "test_value")
				return metric
			},
			expected: []*metricpb.MetricDescriptor{
				{
					Name:        "custom.googleapis.com/test.metric",
					DisplayName: "test.metric",
					Type:        "custom.googleapis.com/test.metric",
					MetricKind:  metricpb.MetricDescriptor_CUMULATIVE,
					ValueType:   metricpb.MetricDescriptor_DOUBLE,
					Unit:        "1",
					Description: "Description",
					Labels: []*label.LabelDescriptor{
						{
							Key: "test_label",
						},
					},
				},
			},
		},
		{
			name: "Delta Monotonic Sum",
			metricCreator: func() pdata.Metric {
				metric := pdata.NewMetric()
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.SetName("test.metric")
				metric.SetDescription("Description")
				metric.SetUnit("1")
				sum := metric.Sum()
				sum.SetIsMonotonic(true)
				sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
				point := sum.DataPoints().AppendEmpty()
				point.SetDoubleVal(10)
				point.Attributes().InsertString("test.label", "test_value")
				return metric
			},
			expected: []*metricpb.MetricDescriptor{
				{
					Name:        "test.metric",
					DisplayName: "test.metric",
					Type:        "workload.googleapis.com/test.metric",
					MetricKind:  metricpb.MetricDescriptor_CUMULATIVE,
					ValueType:   metricpb.MetricDescriptor_DOUBLE,
					Unit:        "1",
					Description: "Description",
					Labels: []*label.LabelDescriptor{
						{
							Key: "test_label",
						},
					},
				},
			},
		},
		{
			name: "Non-Monotonic Sum",
			metricCreator: func() pdata.Metric {
				metric := pdata.NewMetric()
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.SetName("test.metric")
				metric.SetDescription("Description")
				metric.SetUnit("1")
				sum := metric.Sum()
				sum.SetIsMonotonic(false)
				sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				point := sum.DataPoints().AppendEmpty()
				point.SetDoubleVal(10)
				point.Attributes().InsertString("test.label", "test_value")
				return metric
			},
			expected: []*metricpb.MetricDescriptor{
				{
					Name:        "test.metric",
					DisplayName: "test.metric",
					Type:        "workload.googleapis.com/test.metric",
					MetricKind:  metricpb.MetricDescriptor_GAUGE,
					ValueType:   metricpb.MetricDescriptor_DOUBLE,
					Unit:        "1",
					Description: "Description",
					Labels: []*label.LabelDescriptor{
						{
							Key: "test_label",
						},
					},
				},
			},
		},
		{
			name: "Cumulative Histogram",
			metricCreator: func() pdata.Metric {
				metric := pdata.NewMetric()
				metric.SetDataType(pmetric.MetricDataTypeHistogram)
				metric.SetName("test.metric")
				metric.SetDescription("Description")
				metric.SetUnit("1")
				histogram := metric.Histogram()
				histogram.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				point := histogram.DataPoints().AppendEmpty()
				point.Attributes().InsertString("test.label", "test_value")
				return metric
			},
			expected: []*metricpb.MetricDescriptor{
				{
					Name:        "test.metric",
					DisplayName: "test.metric",
					Type:        "workload.googleapis.com/test.metric",
					MetricKind:  metricpb.MetricDescriptor_CUMULATIVE,
					ValueType:   metricpb.MetricDescriptor_DISTRIBUTION,
					Unit:        "1",
					Description: "Description",
					Labels: []*label.LabelDescriptor{
						{
							Key: "test_label",
						},
					},
				},
			},
		},
		{
			name: "Cumulative Exponential Histogram",
			metricCreator: func() pdata.Metric {
				metric := pdata.NewMetric()
				metric.SetDataType(pmetric.MetricDataTypeExponentialHistogram)
				metric.SetName("test.metric")
				metric.SetDescription("Description")
				metric.SetUnit("1")
				histogram := metric.ExponentialHistogram()
				histogram.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				return metric
			},
			expected: []*metricpb.MetricDescriptor{
				{
					Name:        "test.metric",
					DisplayName: "test.metric",
					Type:        "workload.googleapis.com/test.metric",
					MetricKind:  metricpb.MetricDescriptor_CUMULATIVE,
					ValueType:   metricpb.MetricDescriptor_DISTRIBUTION,
					Unit:        "1",
					Description: "Description",
					Labels:      []*label.LabelDescriptor{},
				},
			},
		},
		{
			name: "Summary",
			metricCreator: func() pdata.Metric {
				metric := pdata.NewMetric()
				metric.SetDataType(pmetric.MetricDataTypeSummary)
				metric.SetName("test.metric")
				metric.SetDescription("Description")
				metric.SetUnit("1")
				summary := metric.Summary()
				point := summary.DataPoints().AppendEmpty()
				point.Attributes().InsertString("test.label", "value")
				return metric
			},
			expected: []*metricpb.MetricDescriptor{
				{
					DisplayName: "test.metric_sum",
					Type:        "workload.googleapis.com/test.metric_sum",
					MetricKind:  metricpb.MetricDescriptor_CUMULATIVE,
					ValueType:   metricpb.MetricDescriptor_DOUBLE,
					Unit:        "1",
					Description: "Description",
					Labels: []*label.LabelDescriptor{
						{
							Key: "test_label",
						},
					},
				},
				{
					DisplayName: "test.metric_count",
					Type:        "workload.googleapis.com/test.metric_count",
					MetricKind:  metricpb.MetricDescriptor_CUMULATIVE,
					ValueType:   metricpb.MetricDescriptor_DOUBLE,
					Unit:        "1",
					Description: "Description",
					Labels: []*label.LabelDescriptor{
						{
							Key: "test_label",
						},
					},
				},
				{
					Type:        "workload.googleapis.com/test.metric",
					DisplayName: "test.metric",
					MetricKind:  metricpb.MetricDescriptor_GAUGE,
					ValueType:   metricpb.MetricDescriptor_DOUBLE,
					Unit:        "1",
					Description: "Description",
					Labels: []*label.LabelDescriptor{
						{
							Key: "test_label",
						},
						{
							Key:         "quantile",
							Description: "the value at a given quantile of a distribution",
						},
					},
				},
			},
		},
		{
			name: "Multiple points",
			metricCreator: func() pdata.Metric {
				metric := pdata.NewMetric()
				metric.SetDataType(pmetric.MetricDataTypeGauge)
				metric.SetName("custom.googleapis.com/test.metric")
				metric.SetDescription("Description")
				metric.SetUnit("1")
				gauge := metric.Gauge()

				for i := 0; i < 5; i++ {
					point := gauge.DataPoints().AppendEmpty()
					point.SetDoubleVal(10)
					point.Attributes().InsertString("test.label", "test_value")
				}
				return metric
			},
			expected: []*metricpb.MetricDescriptor{
				{
					Name:        "custom.googleapis.com/test.metric",
					DisplayName: "test.metric",
					Type:        "custom.googleapis.com/test.metric",
					MetricKind:  metricpb.MetricDescriptor_GAUGE,
					ValueType:   metricpb.MetricDescriptor_DOUBLE,
					Unit:        "1",
					Description: "Description",
					Labels: []*label.LabelDescriptor{
						{
							Key: "test_label",
						},
					},
				},
			},
		},
		{
			name: "with extraLabels from resource",
			metricCreator: func() pdata.Metric {
				metric := pdata.NewMetric()
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.SetName("custom.googleapis.com/test.metric")
				metric.SetDescription("Description")
				metric.SetUnit("1")
				sum := metric.Sum()
				sum.SetIsMonotonic(true)
				sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				point := sum.DataPoints().AppendEmpty()
				point.SetDoubleVal(10)
				point.Attributes().InsertString("test.label", "test_value")
				return metric
			},
			extraLabels: labels{
				"service.name":        "myservice",
				"service.instance_id": "abcdef",
				"service.namespace":   "myns",
			},
			expected: []*metricpb.MetricDescriptor{
				{
					Name:        "custom.googleapis.com/test.metric",
					DisplayName: "test.metric",
					Type:        "custom.googleapis.com/test.metric",
					MetricKind:  metricpb.MetricDescriptor_CUMULATIVE,
					ValueType:   metricpb.MetricDescriptor_DOUBLE,
					Unit:        "1",
					Description: "Description",
					Labels: []*label.LabelDescriptor{
						{
							Key: "service_name",
						},
						{
							Key: "service_instance_id",
						},
						{
							Key: "service_namespace",
						},
						{
							Key: "test_label",
						},
					},
				},
			},
		},
		{
			name: "Attributes that collide",
			metricCreator: func() pdata.Metric {
				metric := pdata.NewMetric()
				metric.SetDataType(pmetric.MetricDataTypeGauge)
				metric.SetName("custom.googleapis.com/test.metric")
				metric.SetDescription("Description")
				metric.SetUnit("1")
				gauge := metric.Gauge()
				point := gauge.DataPoints().AppendEmpty()
				point.SetDoubleVal(10)
				point.Attributes().InsertString("test.label", "test_value")
				point.Attributes().InsertString("test_label", "other_value")
				return metric
			},
			expected: []*metricpb.MetricDescriptor{
				{
					Name:        "custom.googleapis.com/test.metric",
					DisplayName: "test.metric",
					Type:        "custom.googleapis.com/test.metric",
					MetricKind:  metricpb.MetricDescriptor_GAUGE,
					ValueType:   metricpb.MetricDescriptor_DOUBLE,
					Unit:        "1",
					Description: "Description",
					Labels: []*label.LabelDescriptor{
						{
							// even though we had two attributes originally,
							// they are a single label after sanitization
							Key: "test_label",
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mapper, shutdown := newTestMetricMapper()
			defer shutdown()
			metric := test.metricCreator()
			md := mapper.metricDescriptor(metric, test.extraLabels)
			diff := cmp.Diff(
				test.expected,
				md,
				protocmp.Transform(),
				protocmp.SortRepeatedFields(&metricpb.MetricDescriptor{}, "labels"),
			)
			if diff != "" {
				assert.Failf(t, "Actual and expected differ:", diff)
			}
		})
	}
}

type knownDomainsTest struct {
	name         string
	metricType   string
	knownDomains []string
}

func TestKnownDomains(t *testing.T) {
	tests := []knownDomainsTest{
		{
			name:       "test",
			metricType: "prefix/test",
		},
		{
			name:       "googleapis.com/test",
			metricType: "googleapis.com/test",
		},
		{
			name:       "kubernetes.io/test",
			metricType: "kubernetes.io/test",
		},
		{
			name:       "istio.io/test",
			metricType: "istio.io/test",
		},
		{
			name:       "knative.dev/test",
			metricType: "knative.dev/test",
		},
		{
			name:         "knative.dev/test",
			metricType:   "prefix/knative.dev/test",
			knownDomains: []string{"example.com"},
		},
		{
			name:         "example.com/test",
			metricType:   "example.com/test",
			knownDomains: []string{"example.com"},
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%v to %v", test.name, test.metricType), func(t *testing.T) {
			mapper, shutdown := newTestMetricMapper()
			defer shutdown()
			if len(test.knownDomains) > 0 {
				mapper.cfg.MetricConfig.KnownDomains = test.knownDomains
			}
			mapper.cfg.MetricConfig.Prefix = "prefix"
			assert.Equal(t, test.metricType, mapper.metricNameToType(test.name))
		})
	}
}

func TestInstrumentationScopeToLabels(t *testing.T) {
	newInstrumentationScope := func(name, version string) pdata.InstrumentationScope {
		is := pdata.NewInstrumentationScope()
		is.SetName("foo")
		is.SetVersion("1.2.3")
		return is
	}

	tests := []struct {
		name                 string
		InstrumentationScope pdata.InstrumentationScope
		metricConfig         MetricConfig
		output               labels
	}{{
		name:                 "fetchFromInstrumentationScope",
		metricConfig:         MetricConfig{InstrumentationLibraryLabels: true},
		InstrumentationScope: newInstrumentationScope("foo", "1.2.3"),
		output:               labels{"instrumentation_source": "foo", "instrumentation_version": "1.2.3"},
	}, {
		name:                 "disabledInConfig",
		metricConfig:         MetricConfig{InstrumentationLibraryLabels: false},
		InstrumentationScope: newInstrumentationScope("foo", "1.2.3"),
		output:               labels{},
	}, {
		name:                 "notSetInInstrumentationScope",
		metricConfig:         MetricConfig{InstrumentationLibraryLabels: true},
		InstrumentationScope: pdata.NewInstrumentationScope(),
		output:               labels{"instrumentation_source": "", "instrumentation_version": ""},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m, shutdown := newTestMetricMapper()
			defer shutdown()
			m.cfg.MetricConfig = test.metricConfig

			out := m.instrumentationScopeToLabels(test.InstrumentationScope)

			assert.Equal(t, out, test.output)
		})
	}
}
