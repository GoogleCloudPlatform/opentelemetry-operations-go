// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metricbatcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestSplitResourceMetrics(t *testing.T) {
	points := func(n int) []metricdata.DataPoint[int64] {
		var dps []metricdata.DataPoint[int64]
		for i := 0; i < n; i++ {
			dps = append(dps, metricdata.DataPoint[int64]{Value: int64(i)})
		}
		return dps
	}

	tests := []struct {
		name     string
		size     int
		input    *metricdata.ResourceMetrics
		expected [][][]int // Expected representation of the batching structure
	}{
		{
			name: "no splitting needed",
			size: 10,
			input: &metricdata.ResourceMetrics{
				ScopeMetrics: []metricdata.ScopeMetrics{
					{
						Metrics: []metricdata.Metrics{
							{Data: metricdata.Gauge[int64]{DataPoints: points(3)}},
							{Data: metricdata.Gauge[int64]{DataPoints: points(2)}},
						},
					},
				},
			},
			expected: [][][]int{{{3, 2}}},
		},
		{
			name: "split on metric boundary",
			size: 3,
			input: &metricdata.ResourceMetrics{
				ScopeMetrics: []metricdata.ScopeMetrics{
					{
						Metrics: []metricdata.Metrics{
							{Data: metricdata.Gauge[int64]{DataPoints: points(3)}},
							{Data: metricdata.Gauge[int64]{DataPoints: points(2)}},
						},
					},
				},
			},
			expected: [][][]int{
				{{3}},
				{{2}},
			},
		},
		{
			name: "split inside a single metric",
			size: 2,
			input: &metricdata.ResourceMetrics{
				ScopeMetrics: []metricdata.ScopeMetrics{
					{
						Metrics: []metricdata.Metrics{
							{Data: metricdata.Gauge[int64]{DataPoints: points(5)}},
						},
					},
				},
			},
			expected: [][][]int{
				{{2}},
				{{2}},
				{{1}},
			},
		},
		{
			name: "split across scopes",
			size: 4,
			input: &metricdata.ResourceMetrics{
				ScopeMetrics: []metricdata.ScopeMetrics{
					{
						Metrics: []metricdata.Metrics{
							{Data: metricdata.Gauge[int64]{DataPoints: points(2)}},
							{Data: metricdata.Gauge[int64]{DataPoints: points(3)}},
						},
					},
					{
						Metrics: []metricdata.Metrics{
							{Data: metricdata.Gauge[int64]{DataPoints: points(2)}},
						},
					},
				},
			},
			expected: [][][]int{
				{{2, 2}},
				{{1}, {2}}, // The 3-point metric's 3rd point overflowed into the second batch, filling 1, leaving 3 left for the new scope
			},
		},
		{
			name: "zero points input",
			size: 5,
			input: &metricdata.ResourceMetrics{
				ScopeMetrics: []metricdata.ScopeMetrics{
					{
						Metrics: []metricdata.Metrics{
							{Data: metricdata.Gauge[int64]{DataPoints: points(0)}},
						},
					},
				},
			},
			expected: [][][]int{{{0}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batches := splitResourceMetrics(tt.size, tt.input)

			var actual [][][]int
			for _, batch := range batches {
				var scopes [][]int
				for _, sm := range batch.ScopeMetrics {
					var metrics []int
					for _, m := range sm.Metrics {
						metrics = append(metrics, metricDPC(m))
					}
					scopes = append(scopes, metrics)
				}
				actual = append(actual, scopes)
			}
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestCopyMetricDataPreservesTypesAndFields(t *testing.T) {
	m := metricdata.Metrics{
		Name:        "test_sum",
		Description: "desc",
		Unit:        "1",
		Data: metricdata.Sum[int64]{
			Temporality: metricdata.CumulativeTemporality,
			IsMonotonic: true,
			DataPoints: []metricdata.DataPoint[int64]{
				{Value: 1}, {Value: 2},
			},
		},
	}

	copied := copyMetricData(m, 0, 1)

	assert.Equal(t, "test_sum", copied.Name)
	assert.IsType(t, metricdata.Sum[int64]{}, copied.Data)

	sum := copied.Data.(metricdata.Sum[int64])
	assert.Equal(t, metricdata.CumulativeTemporality, sum.Temporality)
	assert.True(t, sum.IsMonotonic)
	assert.Len(t, sum.DataPoints, 1)
	assert.Equal(t, int64(1), sum.DataPoints[0].Value)
}
