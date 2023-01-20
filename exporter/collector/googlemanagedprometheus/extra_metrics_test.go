// Copyright 2023 Google LLC
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
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func testMetric() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()

	// foo-label should be copied to target_info, not locationLabel
	rm.Resource().Attributes().PutStr(locationLabel, "us-east")
	rm.Resource().Attributes().PutStr("foo-label", "bar")

	// scope should not be copied to target_info
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("myscope")
	sm.Scope().SetVersion("v0.0.1")

	// other metrics should not be copied to target_info
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("baz-metric")
	metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(2112)
	return metrics
}

func TestAddExtraMetrics(t *testing.T) {
	for _, tc := range []struct {
		testFunc func(pmetric.Metrics) pmetric.ResourceMetricsSlice
		input    pmetric.Metrics
		expected pmetric.ResourceMetricsSlice
		name     string
	}{
		{
			name: "add target info from resource metric",
			testFunc: func(m pmetric.Metrics) pmetric.ResourceMetricsSlice {
				AddTargetInfoMetric(m)
				return m.ResourceMetrics()
			},
			input: testMetric(),
			expected: func() pmetric.ResourceMetricsSlice {
				metrics := testMetric().ResourceMetrics()

				// Insert a new, empty ScopeMetricsSlice for this resource that will hold target_info
				sm := metrics.At(0).ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName("target_info")
				metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
				metric.Gauge().DataPoints().At(0).Attributes().PutStr("foo-label", "bar")
				return metrics
			}(),
		},
		{
			name: "add scope info from scope metrics",
			testFunc: func(m pmetric.Metrics) pmetric.ResourceMetricsSlice {
				AddScopeInfoMetric(m)
				return m.ResourceMetrics()
			},
			input: testMetric(),
			expected: func() pmetric.ResourceMetricsSlice {
				metrics := testMetric().ResourceMetrics()

				// Insert the scope_info metric into the existing ScopeMetricsSlice
				sm := metrics.At(0).ScopeMetrics().At(0)
				scopeInfoMetric := sm.Metrics().AppendEmpty()
				scopeInfoMetric.SetName("otel_scope_info")
				scopeInfoMetric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)

				// add otel_scope_* attributes to all metrics in this scope (including otel_scope_info)
				for i := 0; i < sm.Metrics().Len(); i++ {
					metric := sm.Metrics().At(i)
					metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
					metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
				}
				return metrics
			}(),
		},
		{
			name: "add both scope info and target info",
			testFunc: func(m pmetric.Metrics) pmetric.ResourceMetricsSlice {
				AddScopeInfoMetric(m)
				AddTargetInfoMetric(m)
				return m.ResourceMetrics()
			},
			input: testMetric(),
			expected: func() pmetric.ResourceMetricsSlice {
				metrics := testMetric().ResourceMetrics()
				scopeMetrics := metrics.At(0).ScopeMetrics()

				// Insert a new, empty ScopeMetricsSlice for this resource that will hold target_info
				sm := scopeMetrics.AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName("target_info")
				metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
				metric.Gauge().DataPoints().At(0).Attributes().PutStr("foo-label", "bar")

				// Insert the scope_info metric into the existing ScopeMetricsSlice
				sm = scopeMetrics.At(0)
				scopeInfoMetric := sm.Metrics().AppendEmpty()
				scopeInfoMetric.SetName("otel_scope_info")
				scopeInfoMetric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)

				// add otel_scope_* attributes to all metrics in all scopes
				// this includes otel_scope_info for the existing (input) ScopeMetrics,
				// and target_info (which will have an empty scope)
				for i := 0; i < sm.Metrics().Len(); i++ {
					metric := sm.Metrics().At(i)
					metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
					metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
				}

				return metrics
			}(),
		},
		{
			name: "ordering of scope/target should not matter",
			testFunc: func(m pmetric.Metrics) pmetric.ResourceMetricsSlice {
				AddTargetInfoMetric(m)
				AddScopeInfoMetric(m)
				return m.ResourceMetrics()
			},
			input: testMetric(),
			expected: func() pmetric.ResourceMetricsSlice {
				metrics := testMetric().ResourceMetrics()
				scopeMetrics := metrics.At(0).ScopeMetrics()

				// Insert a new, empty ScopeMetricsSlice for this resource that will hold target_info
				sm := scopeMetrics.AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName("target_info")
				metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
				metric.Gauge().DataPoints().At(0).Attributes().PutStr("foo-label", "bar")

				// Insert the scope_info metric into the existing ScopeMetricsSlice
				sm = scopeMetrics.At(0)
				scopeInfoMetric := sm.Metrics().AppendEmpty()
				scopeInfoMetric.SetName("otel_scope_info")
				scopeInfoMetric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)

				// add otel_scope_* attributes to all metrics in all scopes
				// this includes otel_scope_info for the existing (input) ScopeMetrics,
				// and target_info (which will have an empty scope)
				for i := 0; i < sm.Metrics().Len(); i++ {
					metric := sm.Metrics().At(i)
					metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
					metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
				}

				return metrics
			}(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rms := tc.testFunc(tc.input)
			assert.EqualValues(t, tc.expected, rms)
		})
	}
}
