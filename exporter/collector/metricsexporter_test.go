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

package collector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	"google.golang.org/genproto/googleapis/api/label"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	start = time.Unix(0, 0)
)

func TestMetricToTimeSeries(t *testing.T) {
	mr := &monitoredrespb.MonitoredResource{}

	t.Run("Sum", func(t *testing.T) {
		mapper := metricMapper{cfg: &Config{}}
		metric := pdata.NewMetric()
		metric.SetDataType(pdata.MetricDataTypeSum)
		sum := metric.Sum()
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
		// Add three points
		sum.DataPoints().AppendEmpty().SetDoubleVal(10)
		sum.DataPoints().AppendEmpty().SetDoubleVal(15)
		sum.DataPoints().AppendEmpty().SetDoubleVal(16)

		ts := mapper.metricToTimeSeries(
			mr,
			labels{},
			metric,
		)
		require.Len(t, ts, 3, "Should create one timeseries for each sum point")
		require.Same(t, ts[0].Resource, mr, "Should assign the passed in monitored resource")
	})

	t.Run("Gauge", func(t *testing.T) {
		mapper := metricMapper{cfg: &Config{}}
		metric := pdata.NewMetric()
		metric.SetDataType(pdata.MetricDataTypeGauge)
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

func TestSumPointToTimeSeries(t *testing.T) {
	mapper := metricMapper{cfg: &Config{}}
	mapper.SetMetricDefaults()
	mr := &monitoredrespb.MonitoredResource{}

	newCase := func() (pdata.Metric, pdata.Sum, pdata.NumberDataPoint) {
		metric := pdata.NewMetric()
		metric.SetDataType(pdata.MetricDataTypeSum)
		sum := metric.Sum()
		point := sum.DataPoints().AppendEmpty()
		return metric, sum, point
	}

	t.Run("Cumulative monotonic", func(t *testing.T) {
		metric, sum, point := newCase()
		metric.SetName("mysum")
		unit := "s"
		metric.SetUnit(unit)
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
		var value int64 = 10
		point.SetIntVal(value)
		end := start.Add(time.Hour)
		point.SetStartTimestamp(pdata.NewTimestampFromTime(start))
		point.SetTimestamp(pdata.NewTimestampFromTime(end))

		ts := mapper.sumPointToTimeSeries(mr, labels{}, metric, sum, point)
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
		ts = mapper.sumPointToTimeSeries(mr, labels{}, metric, sum, point)
		assert.Equal(t, ts.MetricKind, metricpb.MetricDescriptor_CUMULATIVE)
		assert.Equal(t, ts.ValueType, metricpb.MetricDescriptor_DOUBLE)
		assert.Equal(t, ts.Points[0].Value.GetDoubleValue(), float64(value))
	})

	t.Run("Delta monotonic", func(t *testing.T) {
		metric, sum, point := newCase()
		metric.SetName("mysum")
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
		point.SetIntVal(10)
		end := start.Add(time.Hour)
		point.SetStartTimestamp(pdata.NewTimestampFromTime(start))
		point.SetTimestamp(pdata.NewTimestampFromTime(end))

		// Should output a "pseudo-cumulative" with same interval as the delta
		ts := mapper.sumPointToTimeSeries(mr, labels{}, metric, sum, point)
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
		sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
		point.SetIntVal(10)
		end := start.Add(time.Hour)
		point.SetStartTimestamp(pdata.NewTimestampFromTime(start))
		point.SetTimestamp(pdata.NewTimestampFromTime(end))

		// Should output a gauge regardless of temporality, only setting end time
		ts := mapper.sumPointToTimeSeries(mr, labels{}, metric, sum, point)
		assert.Equal(t, ts.MetricKind, metricpb.MetricDescriptor_GAUGE)
		assert.Equal(t, ts.Points[0].Interval, &monitoringpb.TimeInterval{
			EndTime: timestamppb.New(end),
		})

		sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
		ts = mapper.sumPointToTimeSeries(mr, labels{}, metric, sum, point)
		assert.Equal(t, ts.MetricKind, metricpb.MetricDescriptor_GAUGE)
		assert.Equal(t, ts.Points[0].Interval, &monitoringpb.TimeInterval{
			EndTime: timestamppb.New(end),
		})
	})

	t.Run("Add labels", func(t *testing.T) {
		metric, sum, point := newCase()
		extraLabels := map[string]string{"foo": "bar"}
		ts := mapper.sumPointToTimeSeries(mr, labels(extraLabels), metric, sum, point)
		assert.Equal(t, ts.Metric.Labels, extraLabels)

		// Full set of labels
		point.Attributes().InsertString("baz", "bar")
		ts = mapper.sumPointToTimeSeries(mr, labels(extraLabels), metric, sum, point)
		assert.Equal(t, ts.Metric.Labels, map[string]string{"foo": "bar", "baz": "bar"})
	})
}

func TestGaugePointToTimeSeries(t *testing.T) {
	mapper := metricMapper{cfg: &Config{}}
	mapper.SetMetricDefaults()
	mr := &monitoredrespb.MonitoredResource{}

	newCase := func() (pdata.Metric, pdata.Gauge, pdata.NumberDataPoint) {
		metric := pdata.NewMetric()
		metric.SetDataType(pdata.MetricDataTypeGauge)
		gauge := metric.Gauge()
		point := gauge.DataPoints().AppendEmpty()
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

	ts := mapper.gaugePointToTimeSeries(mr, labels{}, metric, gauge, point)
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
	ts = mapper.gaugePointToTimeSeries(mr, labels{}, metric, gauge, point)
	assert.Equal(t, ts.MetricKind, metricpb.MetricDescriptor_GAUGE)
	assert.Equal(t, ts.ValueType, metricpb.MetricDescriptor_DOUBLE)
	assert.Equal(t, ts.Points[0].Value.GetDoubleValue(), float64(value))

	// Add extra labels
	extraLabels := map[string]string{"foo": "bar"}
	ts = mapper.gaugePointToTimeSeries(mr, labels(extraLabels), metric, gauge, point)
	assert.Equal(t, ts.Metric.Labels, extraLabels)

	// Full set of labels
	point.Attributes().InsertString("baz", "bar")
	ts = mapper.gaugePointToTimeSeries(mr, labels(extraLabels), metric, gauge, point)
	assert.Equal(t, ts.Metric.Labels, map[string]string{"foo": "bar", "baz": "bar"})
}

func TestMetricNameToType(t *testing.T) {
	mapper := metricMapper{cfg: &Config{}}
	mapper.SetMetricDefaults()
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
		attributesToLabels(pdata.NewAttributeMapFromMap(map[string]pdata.AttributeValue{
			"foo": pdata.NewAttributeValueString("bar"),
			"bar": pdata.NewAttributeValueString("baz"),
		})),
		labels{"foo": "bar", "bar": "baz"},
	)

	// various key special cases
	assert.Equal(
		t,
		attributesToLabels(pdata.NewAttributeMapFromMap(map[string]pdata.AttributeValue{
			"foo.bar":   pdata.NewAttributeValueString("bar"),
			"_foo":      pdata.NewAttributeValueString("bar"),
			"123.hello": pdata.NewAttributeValueString("bar"),
			"":          pdata.NewAttributeValueString("bar"),
		})),
		labels{
			"foo_bar":       "bar",
			"_foo":          "bar",
			"key_123_hello": "bar",
			"":              "bar",
		},
	)

	attribSlice := pdata.NewAttributeValueArray()
	attribSlice.SliceVal().AppendEmpty().SetStringVal("x")
	attribSlice.SliceVal().AppendEmpty()
	attribSlice.SliceVal().AppendEmpty().SetStringVal("y")

	attribMap := pdata.NewAttributeValueMap()
	attribMap.MapVal().InsertString("a", "b")

	// value special cases
	assert.Equal(
		t,
		attributesToLabels(pdata.NewAttributeMapFromMap(map[string]pdata.AttributeValue{
			"a": pdata.NewAttributeValueBool(true),
			"b": pdata.NewAttributeValueBool(false),
			"c": pdata.NewAttributeValueInt(12),
			"d": pdata.NewAttributeValueDouble(12.3),
			"e": pdata.NewAttributeValueEmpty(),
			"f": pdata.NewAttributeValueBytes([]byte{0xde, 0xad, 0xbe, 0xef}),
			"g": attribSlice,
			"h": attribMap,
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
	expected      []*metricpb.MetricDescriptor
}

func TestMetricDescriptorMapping(t *testing.T) {
	tests := []metricDescriptorTest{
		{
			name: "Gauge",
			metricCreator: func() pdata.Metric {
				metric := pdata.NewMetric()
				metric.SetDataType(pdata.MetricDataTypeGauge)
				metric.SetName("custom.googleapis.com/test.metric")
				metric.SetDescription("Description")
				metric.SetUnit("1")
				gauge := metric.Gauge()
				point := gauge.DataPoints().AppendEmpty()
				point.SetDoubleVal(10)
				point.Attributes().InsertString("test_label", "test_value")
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
				metric.SetDataType(pdata.MetricDataTypeSum)
				metric.SetName("custom.googleapis.com/test.metric")
				metric.SetDescription("Description")
				metric.SetUnit("1")
				sum := metric.Sum()
				sum.SetIsMonotonic(true)
				sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
				point := sum.DataPoints().AppendEmpty()
				point.SetDoubleVal(10)
				point.Attributes().InsertString("test_label", "test_value")
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
				metric.SetDataType(pdata.MetricDataTypeSum)
				metric.SetName("test.metric")
				metric.SetDescription("Description")
				metric.SetUnit("1")
				sum := metric.Sum()
				sum.SetIsMonotonic(true)
				sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
				point := sum.DataPoints().AppendEmpty()
				point.SetDoubleVal(10)
				point.Attributes().InsertString("test_label", "test_value")
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
				metric.SetDataType(pdata.MetricDataTypeSum)
				metric.SetName("test.metric")
				metric.SetDescription("Description")
				metric.SetUnit("1")
				sum := metric.Sum()
				sum.SetIsMonotonic(false)
				sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
				point := sum.DataPoints().AppendEmpty()
				point.SetDoubleVal(10)
				point.Attributes().InsertString("test_label", "test_value")
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
				metric.SetDataType(pdata.MetricDataTypeHistogram)
				metric.SetName("test.metric")
				metric.SetDescription("Description")
				metric.SetUnit("1")
				histogram := metric.Histogram()
				histogram.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
				point := histogram.DataPoints().AppendEmpty()
				point.Attributes().InsertString("test_label", "test_value")
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
				metric.SetDataType(pdata.MetricDataTypeExponentialHistogram)
				metric.SetName("test.metric")
				metric.SetDescription("Description")
				metric.SetUnit("1")
				histogram := metric.ExponentialHistogram()
				histogram.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
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
				metric.SetDataType(pdata.MetricDataTypeSummary)
				metric.SetName("test.metric")
				metric.SetDescription("Description")
				metric.SetUnit("1")
				summary := metric.Summary()
				point := summary.DataPoints().AppendEmpty()
				point.Attributes().InsertString("test_label", "value")
				return metric
			},
			expected: []*metricpb.MetricDescriptor{
				{
					DisplayName: "test.metric_summary_sum",
					Type:        "workload.googleapis.com/test.metric_summary_sum",
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
					DisplayName: "test.metric_summary_count",
					Type:        "workload.googleapis.com/test.metric_summary_count",
					MetricKind:  metricpb.MetricDescriptor_CUMULATIVE,
					ValueType:   metricpb.MetricDescriptor_INT64,
					Unit:        "1",
					Description: "Description",
					Labels: []*label.LabelDescriptor{
						{
							Key: "test_label",
						},
					},
				},
				{
					Type:        "workload.googleapis.com/test.metric_summary_percentile",
					DisplayName: "test.metric_summary_percentile",
					MetricKind:  metricpb.MetricDescriptor_GAUGE,
					ValueType:   metricpb.MetricDescriptor_DOUBLE,
					Unit:        "1",
					Description: "Description",
					Labels: []*label.LabelDescriptor{
						{
							Key: "test_label",
						},
						{
							Key:         "percentile",
							Description: "the value at a given percentile of a distribution",
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mapper := &metricMapper{}
			mapper.SetMetricDefaults()
			metric := test.metricCreator()
			md := mapper.metricDescriptor(metric)
			assert.Equal(t, md, test.expected)
		})
	}
}
