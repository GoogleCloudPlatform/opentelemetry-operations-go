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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

func TestMetricToTimeSeries(t *testing.T) {
	ilm := pdata.NewInstrumentationLibraryMetrics()
	me := ilm.Metrics().AppendEmpty()
	me.SetDataType(pdata.MetricDataTypeSum)
	{
		sum := me.Sum()
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
		point := sum.DataPoints().AppendEmpty()
		point.SetDoubleVal(10)
	}

	mr := &monitoredrespb.MonitoredResource{}

	ts := metricToTimeSeries(
		mr,
		labels{},
		ilm,
		me,
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
