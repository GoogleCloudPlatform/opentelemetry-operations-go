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
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	start = time.Unix(0, 0)
)

func TestMetricToTimeSeries(t *testing.T) {
	mapper := metricMapper{cfg: &Config{}}
	metric := pdata.NewMetric()
	metric.SetDataType(pdata.MetricDataTypeSum)
	sum := metric.Sum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
	point := sum.DataPoints().AppendEmpty()
	point.SetDoubleVal(10)
	mr := &monitoredrespb.MonitoredResource{}

	ts := mapper.metricToTimeSeries(
		mr,
		labels{},
		metric,
	)

	require.Len(t, ts, 1, "Should create one timeseries for sum")
	require.Same(t, ts[0].Resource, mr, "Should assign the passed in monitored resource")
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
	mr := &monitoredrespb.MonitoredResource{}

	newCase := func() (pdata.Metric, pdata.Sum, pdata.NumberDataPoint) {
		metric := pdata.NewMetric()
		metric.SetDataType(pdata.MetricDataTypeSum)
		sum := metric.Sum()
		point := sum.DataPoints().AppendEmpty()
		return metric, sum, point
	}

	t.Run("cumulative monotonic", func(t *testing.T) {
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

	t.Run("delta monotonic", func(t *testing.T) {
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

	t.Run("non monotonic", func(t *testing.T) {
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

	t.Run("add extra labels", func(t *testing.T) {
		metric, sum, point := newCase()
		extraLabels := map[string]string{"foo": "bar"}
		ts := mapper.sumPointToTimeSeries(mr, labels(extraLabels), metric, sum, point)
		assert.Equal(t, ts.Metric.Labels, extraLabels)
	})
}

func TestMetricNameToType(t *testing.T) {
	mapper := metricMapper{cfg: &Config{}}
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
