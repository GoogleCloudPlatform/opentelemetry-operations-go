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

package googlemanagedprometheus

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestGetMetricName(t *testing.T) {
	for _, tc := range []struct {
		desc          string
		baseName      string
		compliantName string
		metric        func(pmetric.Metric)
		expected      string
		expectErr     bool
	}{
		{
			desc:          "monotonic sum",
			baseName:      "foo",
			compliantName: "foo_seconds_total",
			metric: func(m pmetric.Metric) {
				m.SetName("foo")
				m.SetUnit("s")
				sum := m.SetEmptySum()
				sum.SetIsMonotonic(true)
			},
			expected: "foo_seconds_total/counter",
		},
		{
			desc:          "non-monotonic sum",
			baseName:      "foo",
			compliantName: "foo_total",
			metric: func(m pmetric.Metric) {
				m.SetName("foo")
				sum := m.SetEmptySum()
				sum.SetIsMonotonic(false)
			},
			expected: "foo_total/gauge",
		},
		{
			desc:          "gauge",
			baseName:      "bar",
			compliantName: "bar",
			metric: func(m pmetric.Metric) {
				m.SetName("bar")
				m.SetEmptyGauge()
			},
			expected: "bar/gauge",
		},
		{
			desc:          "summary sum",
			baseName:      "baz_sum",
			compliantName: "baz",
			metric: func(m pmetric.Metric) {
				m.SetName("baz")
				m.SetEmptySummary()
			},
			expected: "baz_sum/summary:counter",
		},
		{
			desc:          "summary count",
			baseName:      "baz_count",
			compliantName: "baz",
			metric: func(m pmetric.Metric) {
				m.SetName("baz")
				m.SetEmptySummary()
			},
			expected: "baz_count/summary",
		},
		{
			desc:          "summary quantile",
			baseName:      "baz",
			compliantName: "baz",
			metric: func(m pmetric.Metric) {
				m.SetName("baz")
				m.SetEmptySummary()
			},
			expected: "baz/summary",
		},
		{
			desc:          "histogram",
			baseName:      "hello",
			compliantName: "hello",
			metric: func(m pmetric.Metric) {
				m.SetName("hello")
				m.SetEmptyHistogram()
			},
			expected: "hello/histogram",
		},
		{
			desc:          "exponential histogram",
			baseName:      "hello",
			compliantName: "hello",
			metric: func(m pmetric.Metric) {
				m.SetName("hello")
				m.SetEmptyExponentialHistogram()
			},
			expected: "hello/histogram",
		},
		{
			desc:          "untyped gauge with feature gate enabled returns unknown",
			baseName:      "bar",
			compliantName: "bar",
			metric: func(m pmetric.Metric) {
				m.SetName("bar")
				m.Metadata().PutStr("prometheus.type", "unknown")
				m.SetEmptyGauge()
			},
			expected: "bar/unknown",
		},
		{
			desc:          "untyped sum with feature gate enabled returns unknown:counter",
			baseName:      "bar",
			compliantName: "bar",
			metric: func(m pmetric.Metric) {
				m.SetName("bar")
				m.Metadata().PutStr("prometheus.type", "unknown")
				m.SetEmptySum()
				m.Sum().SetIsMonotonic(true)
			},
			expected: "bar/unknown:counter",
		},
		{
			desc:          "untyped sum with feature gate enabled + name normalization returns unknown:counter without _tota",
			baseName:      "bar",
			compliantName: "bar_total",
			metric: func(m pmetric.Metric) {
				m.SetName("bar")
				m.Metadata().PutStr("prometheus.type", "unknown")
				m.SetEmptySum()
				m.Sum().SetIsMonotonic(true)
			},
			expected: "bar/unknown:counter",
		},
		{
			desc:          "untyped sum with preexisting 'total' suffix returns unknown:counter with _total",
			baseName:      "bar.total.total.total.total",
			compliantName: "bar_total",
			metric: func(m pmetric.Metric) {
				m.SetName("bar.total.total.total.total")
				m.Metadata().PutStr("prometheus.type", "unknown")
				m.SetEmptySum()
				m.Sum().SetIsMonotonic(true)
			},
			expected: "bar_total/unknown:counter",
		},
		{
			desc:          "normal sum without total adds _total",
			baseName:      "foo",
			compliantName: "foo_total",
			metric: func(m pmetric.Metric) {
				m.SetName("foo")
				sum := m.SetEmptySum()
				sum.SetIsMonotonic(true)
			},
			expected: "foo_total/counter",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			metric := pmetric.NewMetric()
			tc.metric(metric)
			got, err := GetMetricName(tc.baseName, tc.compliantName, metric)
			if tc.expectErr == (err == nil) {
				t.Errorf("MetricName(%v, %+v)=err(%v); want err: %v", tc.baseName, metric, err, tc.expectErr)
			}
			if got != tc.expected {
				t.Errorf("MetricName(%v, %+v)=%v; want %v", tc.baseName, metric, got, tc.expected)
			}
		})
	}
}
