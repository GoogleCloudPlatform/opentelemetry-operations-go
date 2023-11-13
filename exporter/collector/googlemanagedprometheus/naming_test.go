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
		featuregate.StageBeta,
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
			desc:     "sum without total",
			baseName: "foo",
			metric: func(m pmetric.Metric) {
				m.SetName("foo")
				sum := m.SetEmptySum()
				sum.SetIsMonotonic(true)
			},
			expected: "foo_total/counter",
		},
		{
			desc:     "sum without total is unaffected when normalization disabled",
			baseName: "foo",
			metric: func(m pmetric.Metric) {
				//nolint:errcheck
				featuregate.GlobalRegistry().Set("pkg.translator.prometheus.NormalizeName", false)
				m.SetName("foo")
				sum := m.SetEmptySum()
				sum.SetIsMonotonic(true)
			},
			expected: "foo/counter",
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
			desc:     "sum with unit is unaffected when normalization disabled",
			baseName: "foo",
			metric: func(m pmetric.Metric) {
				//nolint:errcheck
				featuregate.GlobalRegistry().Set("pkg.translator.prometheus.NormalizeName", false)
				m.SetName("foo_total")
				m.SetUnit("s")
				sum := m.SetEmptySum()
				sum.SetIsMonotonic(true)
			},
			expected: "foo_total/counter",
		},
		{
			desc:     "non-monotonic sum",
			baseName: "foo_total",
			metric: func(m pmetric.Metric) {
				m.SetName("foo_total")
				sum := m.SetEmptySum()
				sum.SetIsMonotonic(false)
			},
			expected: "foo_total/gauge",
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
		{
			desc:     "untyped gauge with feature gate disabled does nothing",
			baseName: "bar",
			metric: func(m pmetric.Metric) {
				//nolint:errcheck
				featuregate.GlobalRegistry().Set(gcpUntypedDoubleExportGateKey, false)
				m.SetName("bar")
				m.SetEmptyGauge()
				m.Gauge().DataPoints().AppendEmpty().Attributes().PutStr(GCPOpsAgentUntypedMetricKey, "true")
			},
			expected: "bar/gauge",
		},
		{
			desc:     "untyped sum with gcp.untypedDoubleExport disabled only normalizes",
			baseName: "bar",
			metric: func(m pmetric.Metric) {
				//nolint:errcheck
				featuregate.GlobalRegistry().Set(gcpUntypedDoubleExportGateKey, false)
				m.SetName("bar")
				m.SetEmptySum()
				m.Sum().SetIsMonotonic(true)
				m.Sum().DataPoints().AppendEmpty().Attributes().PutStr(GCPOpsAgentUntypedMetricKey, "true")
			},
			expected: "bar_total/counter",
		},
		{
			desc:     "untyped sum with feature gates disabled does nothing",
			baseName: "bar",
			metric: func(m pmetric.Metric) {
				//nolint:errcheck
				featuregate.GlobalRegistry().Set("pkg.translator.prometheus.NormalizeName", false)
				//nolint:errcheck
				featuregate.GlobalRegistry().Set(gcpUntypedDoubleExportGateKey, false)
				m.SetName("bar")
				m.SetEmptySum()
				m.Sum().SetIsMonotonic(true)
				m.Sum().DataPoints().AppendEmpty().Attributes().PutStr(GCPOpsAgentUntypedMetricKey, "true")
			},
			expected: "bar/counter",
		},
		{
			desc:     "untyped gauge with feature gate enabled returns unknown",
			baseName: "bar",
			metric: func(m pmetric.Metric) {
				//nolint:errcheck
				featuregate.GlobalRegistry().Set(gcpUntypedDoubleExportGateKey, true)
				m.SetName("bar")
				m.SetEmptyGauge()
				m.Gauge().DataPoints().AppendEmpty().Attributes().PutStr(GCPOpsAgentUntypedMetricKey, "true")
			},
			expected: "bar/unknown",
		},
		{
			desc:     "untyped sum with feature gate enabled returns unknown:counter",
			baseName: "bar",
			metric: func(m pmetric.Metric) {
				//nolint:errcheck
				featuregate.GlobalRegistry().Set(gcpUntypedDoubleExportGateKey, true)
				m.SetName("bar")
				m.SetEmptySum()
				m.Sum().SetIsMonotonic(true)
				m.Sum().DataPoints().AppendEmpty().Attributes().PutStr(GCPOpsAgentUntypedMetricKey, "true")
			},
			expected: "bar/unknown:counter",
		},
		{
			desc:     "untyped sum with feature gate enabled + name normalization returns unknown:counter without _total",
			baseName: "bar",
			metric: func(m pmetric.Metric) {
				//nolint:errcheck
				featuregate.GlobalRegistry().Set(gcpUntypedDoubleExportGateKey, true)
				//nolint:errcheck
				featuregate.GlobalRegistry().Set("pkg.translator.prometheus.NormalizeName", true)
				m.SetName("bar")
				m.SetEmptySum()
				m.Sum().SetIsMonotonic(true)
				m.Sum().DataPoints().AppendEmpty().Attributes().PutStr(GCPOpsAgentUntypedMetricKey, "true")
			},
			expected: "bar/unknown:counter",
		},
		{
			desc:     "untyped sum with multiple data points and preexisting 'total' suffix and feature gate enabled and name normalization returns unknown:counter with _total",
			baseName: "bar.total.total.total.total",
			metric: func(m pmetric.Metric) {
				//nolint:errcheck
				featuregate.GlobalRegistry().Set(gcpUntypedDoubleExportGateKey, true)
				//nolint:errcheck
				featuregate.GlobalRegistry().Set("pkg.translator.prometheus.NormalizeName", true)
				m.SetName("bar.total.total.total.total")
				m.SetEmptySum()
				m.Sum().SetIsMonotonic(true)
				m.Sum().DataPoints().AppendEmpty().Attributes().PutStr(GCPOpsAgentUntypedMetricKey, "true")
				m.Sum().DataPoints().AppendEmpty().Attributes().PutStr(GCPOpsAgentUntypedMetricKey, "true")
			},
			// prometheus name normalization removes all preexisting "total"s before appending its own
			expected: "bar_total/unknown:counter",
		},
		{
			desc:     "normal sum without total and feature gate enabled + name normalization adds _total",
			baseName: "foo",
			metric: func(m pmetric.Metric) {
				//nolint:errcheck
				featuregate.GlobalRegistry().Set(gcpUntypedDoubleExportGateKey, true)
				//nolint:errcheck
				featuregate.GlobalRegistry().Set("pkg.translator.prometheus.NormalizeName", true)
				m.SetName("foo")
				sum := m.SetEmptySum()
				sum.SetIsMonotonic(true)
			},
			expected: "foo_total/counter",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// Name translation uses global feature gate registry, set to defaults before each test.
			//nolint:errcheck
			featuregate.GlobalRegistry().Set("pkg.translator.prometheus.NormalizeName", true)

			assert.True(t, gate.IsEnabled())
			metric := pmetric.NewMetric()
			tc.metric(metric)
			got, err := DefaultConfig().GetMetricName(tc.baseName, metric)
			if tc.expectErr == (err == nil) {
				t.Errorf("MetricName(%v, %+v)=err(%v); want err: %v", tc.baseName, metric, err, tc.expectErr)
			}
			if got != tc.expected {
				t.Errorf("MetricName(%v, %+v)=%v; want %v", tc.baseName, metric, got, tc.expected)
			}
		})
	}
}
