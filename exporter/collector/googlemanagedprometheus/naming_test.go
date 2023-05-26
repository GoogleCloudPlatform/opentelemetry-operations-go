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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const promFeatureGate = "pkg.translator.prometheus.NormalizeName"

func TestGetMetricName(t *testing.T) {
	// Enable the prometheus naming feature-gate during the test
	registry := featuregate.NewRegistry()
	gate := registry.MustRegister(
		promFeatureGate,
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("Controls whether metrics names are automatically normalized to follow Prometheus naming convention"),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/8950"),
	)
	require.NoError(t, registry.Set(promFeatureGate, true))
	defer func() {
		require.NoError(t, registry.Set(promFeatureGate, false))
	}()
	for _, tc := range []struct {
		desc      string
		baseName  string
		metric    func(pmetric.Metric)
		expected  string
		expectErr bool
	}{
		{
			desc:     "sum without total gets added",
			baseName: "foo",
			metric: func(m pmetric.Metric) {
				m.SetName("foo")
				sum := m.SetEmptySum()
				sum.SetIsMonotonic(true)
			},
			expected: "foo_total/counter",
		},
		{
			desc:     "sum with total",
			baseName: "foo_total",
			metric: func(m pmetric.Metric) {
				m.SetName("foo_total")
				sum := m.SetEmptySum()
				sum.SetIsMonotonic(true)
			},
			expected: "foo_total/counter",
		},
		{
			desc:     "sum with unit",
			baseName: "foo",
			metric: func(m pmetric.Metric) {
				m.SetName("foo_total")
				m.SetUnit("s")
				sum := m.SetEmptySum()
				sum.SetIsMonotonic(true)
			},
			expected: "foo_seconds_total/counter",
		},
		{
			desc:     "gauge",
			baseName: "bar",
			metric: func(m pmetric.Metric) {
				m.SetName("bar")
				m.SetEmptyGauge()
			},
			expected: "bar/gauge",
		},
		{
			desc:     "summary sum",
			baseName: "baz_sum",
			metric: func(m pmetric.Metric) {
				m.SetName("baz")
				m.SetEmptySummary()
			},
			expected: "baz_sum/summary:counter",
		},
		{
			desc:     "summary count",
			baseName: "baz_count",
			metric: func(m pmetric.Metric) {
				m.SetName("baz")
				m.SetEmptySummary()
			},
			expected: "baz_count/summary",
		},
		{
			desc:     "summary quantile",
			baseName: "baz",
			metric: func(m pmetric.Metric) {
				m.SetName("baz")
				m.SetEmptySummary()
			},
			expected: "baz/summary",
		},
		{
			desc:     "histogram",
			baseName: "hello",
			metric: func(m pmetric.Metric) {
				m.SetName("hello")
				m.SetEmptyHistogram()
			},
			expected: "hello/histogram",
		},
		{
			desc:     "other",
			baseName: "other",
			metric: func(m pmetric.Metric) {
				m.SetName("other")
				m.SetEmptyExponentialHistogram()
			},
			expectErr: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			assert.True(t, gate.IsEnabled())
			metric := pmetric.NewMetric()
			tc.metric(metric)
			got, err := GetMetricName(tc.baseName, metric)
			if tc.expectErr == (err == nil) {
				t.Errorf("MetricName(%v, %+v)=err(%v); want err: %v", tc.baseName, metric, err, tc.expectErr)
			}
			if got != tc.expected {
				t.Errorf("MetricName(%v, %+v)=%v; want %v", tc.baseName, metric, got, tc.expected)
			}
		})
	}
}
