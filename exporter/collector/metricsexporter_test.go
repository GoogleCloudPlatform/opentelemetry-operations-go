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
