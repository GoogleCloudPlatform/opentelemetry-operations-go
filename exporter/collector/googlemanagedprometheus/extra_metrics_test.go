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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func testMetric(timestamp time.Time) pmetric.Metrics {
	return appendMetric(pmetric.NewMetrics(), timestamp)
}

func appendMetric(metrics pmetric.Metrics, timestamp time.Time) pmetric.Metrics {
	rm := metrics.ResourceMetrics().AppendEmpty()

	// foo-label should be copied to target_info, not locationLabel
	rm.Resource().Attributes().PutStr(semconv.AttributeServiceName, "my-service")
	rm.Resource().Attributes().PutStr(locationLabel, "us-east")
	rm.Resource().Attributes().PutStr("foo-label", "bar")

	// scope should not be copied to target_info
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("myscope")
	sm.Scope().SetVersion("v0.0.1")

	// other metrics should not be copied to target_info
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("gauge-metric")
	metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(2112)
	metric.Gauge().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	metric = sm.Metrics().AppendEmpty()
	metric.SetName("sum-metric")
	metric.SetEmptySum().DataPoints().AppendEmpty().SetIntValue(2112)
	metric.Sum().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	metric = sm.Metrics().AppendEmpty()
	metric.SetName("summary-metric")
	metric.SetEmptySummary().DataPoints().AppendEmpty().SetCount(2112)
	metric.Summary().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	return metrics
}

func TestAddExtraMetrics(t *testing.T) {
	timestamp := time.Now()
	for _, tc := range []struct {
		input                   pmetric.Metrics
		expected                pmetric.ResourceMetricsSlice
		name                    string
		config                  Config
		enableDoubleFeatureGate bool
	}{
		{
			name:   "add target info from resource metric",
			config: Config{ExtraMetricsConfig: ExtraMetricsConfig{EnableTargetInfo: true}},
			input:  testMetric(timestamp),
			expected: func() pmetric.ResourceMetricsSlice {
				metrics := testMetric(timestamp).ResourceMetrics()

				// Insert a new, empty ScopeMetricsSlice for this resource that will hold target_info
				sm := metrics.At(0).ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName("target_info")
				metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
				metric.Gauge().DataPoints().At(0).Attributes().PutStr("foo-label", "bar")
				metric.Gauge().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
				return metrics
			}(),
		},
		{
			name:   "deduplicate target info from multiple resource metric",
			config: Config{ExtraMetricsConfig: ExtraMetricsConfig{EnableTargetInfo: true}},
			input: func() pmetric.Metrics {
				metrics := appendMetric(testMetric(timestamp), timestamp)
				// Add a non-identifying attribute to the second resource, which should be ignored.
				metrics.ResourceMetrics().At(1).Resource().Attributes().PutStr("ignored-label", "ignored-value")
				return metrics
			}(),
			expected: func() pmetric.ResourceMetricsSlice {
				metrics := appendMetric(testMetric(timestamp), timestamp).ResourceMetrics()
				// Make sure it matches the input
				metrics.At(1).Resource().Attributes().PutStr("ignored-label", "ignored-value")

				// Insert a new, empty ScopeMetricsSlice for this resource that will hold target_info
				sm := metrics.At(0).ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName("target_info")
				metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
				metric.Gauge().DataPoints().At(0).Attributes().PutStr("foo-label", "bar")
				metric.Gauge().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
				return metrics
			}(),
		},
		{
			name:   "add scope info from scope metrics",
			config: Config{ExtraMetricsConfig: ExtraMetricsConfig{EnableScopeInfo: true}},
			input:  testMetric(timestamp),
			expected: func() pmetric.ResourceMetricsSlice {
				metrics := testMetric(timestamp).ResourceMetrics()

				// Insert the scope_info metric into the existing ScopeMetricsSlice
				sm := metrics.At(0).ScopeMetrics().At(0)
				scopeInfoMetric := sm.Metrics().AppendEmpty()
				scopeInfoMetric.SetName("otel_scope_info")
				scopeInfoMetric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)

				// add otel_scope_* attributes to all metrics in this scope (including otel_scope_info)
				for i := 0; i < sm.Metrics().Len(); i++ {
					metric := sm.Metrics().At(i)
					switch metric.Type() {
					case pmetric.MetricTypeGauge:
						metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
						metric.Gauge().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
					case pmetric.MetricTypeSum:
						metric.Sum().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Sum().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
						metric.Sum().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
					case pmetric.MetricTypeSummary:
						metric.Summary().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Summary().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
						metric.Summary().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
					}
				}
				return metrics
			}(),
		},
		{
			name:   "add scope info with attributes",
			config: Config{ExtraMetricsConfig: ExtraMetricsConfig{EnableScopeInfo: true}},
			input: func() pmetric.Metrics {
				metrics := testMetric(timestamp)
				metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes().PutStr("foo_attribute", "bar")
				return metrics
			}(),
			expected: func() pmetric.ResourceMetricsSlice {
				metrics := testMetric(timestamp).ResourceMetrics()
				metrics.At(0).ScopeMetrics().At(0).Scope().Attributes().PutStr("foo_attribute", "bar")

				// Insert the scope_info metric into the existing ScopeMetricsSlice
				sm := metrics.At(0).ScopeMetrics().At(0)
				scopeInfoMetric := sm.Metrics().AppendEmpty()
				scopeInfoMetric.SetName("otel_scope_info")
				scopeInfoMetric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
				scopeInfoMetric.Gauge().DataPoints().At(0).Attributes().PutStr("foo_attribute", "bar")

				// add otel_scope_* attributes to all metrics in this scope (including otel_scope_info)
				for i := 0; i < sm.Metrics().Len(); i++ {
					metric := sm.Metrics().At(i)
					switch metric.Type() {
					case pmetric.MetricTypeGauge:
						metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
						metric.Gauge().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
					case pmetric.MetricTypeSum:
						metric.Sum().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Sum().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
						metric.Sum().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
					case pmetric.MetricTypeSummary:
						metric.Summary().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Summary().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
						metric.Summary().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
					}
				}
				return metrics
			}(),
		},
		{
			name:   "add duplicate scope info with attributes",
			config: Config{ExtraMetricsConfig: ExtraMetricsConfig{EnableScopeInfo: true}},
			input: func() pmetric.Metrics {
				metrics := appendMetric(testMetric(timestamp), timestamp)
				// set different attributes on scopes with the same name + version
				metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes().PutStr("foo_attribute", "bar")
				metrics.ResourceMetrics().At(1).ScopeMetrics().At(0).Scope().Attributes().PutStr("foo_attribute", "not_bar")

				// Add a duplicate scope within the same resource
				sm := metrics.ResourceMetrics().At(1).ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("myscope")
				sm.Scope().SetVersion("v0.0.1")
				return metrics
			}(),
			expected: func() pmetric.ResourceMetricsSlice {
				// Make sure it matches the input
				metrics := appendMetric(testMetric(timestamp), timestamp).ResourceMetrics()
				metrics.At(0).ScopeMetrics().At(0).Scope().Attributes().PutStr("foo_attribute", "bar")
				// This scope attribute is on a duplicate scope_info metric, so it does not
				metrics.At(1).ScopeMetrics().At(0).Scope().Attributes().PutStr("foo_attribute", "not_bar")
				sm := metrics.At(1).ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("myscope")
				sm.Scope().SetVersion("v0.0.1")

				// Insert the scope_info metric into the existing ScopeMetricsSlice
				sm = metrics.At(0).ScopeMetrics().At(0)
				scopeInfoMetric := sm.Metrics().AppendEmpty()
				scopeInfoMetric.SetName("otel_scope_info")
				scopeInfoMetric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
				scopeInfoMetric.Gauge().DataPoints().At(0).Attributes().PutStr("foo_attribute", "bar")

				// add otel_scope_* attributes to all metrics in this scope (including otel_scope_info)
				for i := 0; i < sm.Metrics().Len(); i++ {
					metric := sm.Metrics().At(i)
					switch metric.Type() {
					case pmetric.MetricTypeGauge:
						metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
						scopeInfoMetric.Gauge().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
					case pmetric.MetricTypeSum:
						metric.Sum().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Sum().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
					case pmetric.MetricTypeSummary:
						metric.Summary().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Summary().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
					}
				}

				// add otel_scope_* attributes to all metrics in the second scope
				sm = metrics.At(1).ScopeMetrics().At(0)
				for i := 0; i < sm.Metrics().Len(); i++ {
					metric := sm.Metrics().At(i)
					switch metric.Type() {
					case pmetric.MetricTypeGauge:
						metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
						scopeInfoMetric.Gauge().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
					case pmetric.MetricTypeSum:
						metric.Sum().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Sum().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
					case pmetric.MetricTypeSummary:
						metric.Summary().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Summary().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
					}
				}
				return metrics
			}(),
		},
		{
			name: "add both scope info and target info",
			config: Config{ExtraMetricsConfig: ExtraMetricsConfig{
				EnableScopeInfo:  true,
				EnableTargetInfo: true,
			}},
			input: testMetric(timestamp),
			expected: func() pmetric.ResourceMetricsSlice {
				metrics := testMetric(timestamp).ResourceMetrics()
				scopeMetrics := metrics.At(0).ScopeMetrics()

				// Insert a new, empty ScopeMetricsSlice for this resource that will hold target_info
				sm := scopeMetrics.AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName("target_info")
				metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
				metric.Gauge().DataPoints().At(0).Attributes().PutStr("foo-label", "bar")
				metric.Gauge().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

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
					switch metric.Type() {
					case pmetric.MetricTypeGauge:
						metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
						metric.Gauge().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
					case pmetric.MetricTypeSum:
						metric.Sum().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Sum().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
						metric.Sum().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
					case pmetric.MetricTypeSummary:
						metric.Summary().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Summary().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
						metric.Summary().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
					}
				}

				return metrics
			}(),
		},
		{
			name: "metric as double",
			config: Config{ExtraMetricsConfig: ExtraMetricsConfig{
				EnableScopeInfo:  true,
				EnableTargetInfo: true,
			}},
			input:                   testMetric(timestamp),
			enableDoubleFeatureGate: true,
			expected: func() pmetric.ResourceMetricsSlice {
				metrics := testMetric(timestamp).ResourceMetrics()
				scopeMetrics := metrics.At(0).ScopeMetrics()

				// Insert a new, empty ScopeMetricsSlice for this resource that will hold target_info
				sm := scopeMetrics.AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName("target_info")
				// This changes the value to double because of the feature gate.
				metric.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(1)
				metric.Gauge().DataPoints().At(0).Attributes().PutStr("foo-label", "bar")
				metric.Gauge().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

				// Insert the scope_info metric into the existing ScopeMetricsSlice
				sm = scopeMetrics.At(0)
				scopeInfoMetric := sm.Metrics().AppendEmpty()
				scopeInfoMetric.SetName("otel_scope_info")
				// This changes the value to double because of the feature gate.
				scopeInfoMetric.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(1)

				// add otel_scope_* attributes to all metrics in all scopes
				// this includes otel_scope_info for the existing (input) ScopeMetrics,
				// and target_info (which will have an empty scope)
				for i := 0; i < sm.Metrics().Len(); i++ {
					metric := sm.Metrics().At(i)
					switch metric.Type() {
					case pmetric.MetricTypeGauge:
						dataPoint := metric.Gauge().DataPoints().At(0)
						metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
						metric.Gauge().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
						// Change the original value to double
						if dataPoint.IntValue() == 2112 {
							dataPoint.SetDoubleValue(float64(2112.0))
						}
					case pmetric.MetricTypeSum:
						dataPoint := metric.Sum().DataPoints().At(0)
						dataPoint.Attributes().PutStr("otel_scope_name", "myscope")
						dataPoint.Attributes().PutStr("otel_scope_version", "v0.0.1")
						dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
						// Change the original value to double
						if dataPoint.IntValue() == 2112 {
							dataPoint.SetDoubleValue(float64(2112.0))
						}
					case pmetric.MetricTypeSummary:
						dataPoint := metric.Summary().DataPoints().At(0)
						dataPoint.Attributes().PutStr("otel_scope_name", "myscope")
						dataPoint.Attributes().PutStr("otel_scope_version", "v0.0.1")
						dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
					}
				}

				return metrics
			}(),
		},
		{
			name:   "scope info for other metric types",
			config: Config{ExtraMetricsConfig: ExtraMetricsConfig{EnableScopeInfo: true}},
			input: func() pmetric.Metrics {
				metrics := testMetric(timestamp)
				sum := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				sum.SetName("sum-metric")
				sum.SetEmptySum().DataPoints().AppendEmpty().SetIntValue(1234)
				sum.Sum().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

				summary := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				summary.SetName("summary-metric")
				summary.SetEmptySummary().DataPoints().AppendEmpty().SetSum(float64(1.0))
				summary.Summary().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

				histogram := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				histogram.SetName("histogram-metric")
				_ = histogram.SetEmptyHistogram().DataPoints().AppendEmpty().StartTimestamp().AsTime().Year()
				histogram.Histogram().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

				expHistogram := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				expHistogram.SetName("exponential-histogram")
				_ = expHistogram.SetEmptyExponentialHistogram().DataPoints().AppendEmpty().StartTimestamp().AsTime().Year()
				expHistogram.ExponentialHistogram().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
				return metrics
			}(),
			expected: func() pmetric.ResourceMetricsSlice {
				testMetrics := testMetric(timestamp)
				sum := testMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				sum.SetName("sum-metric")
				sum.SetEmptySum().DataPoints().AppendEmpty().SetIntValue(1234)
				sum.Sum().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

				summary := testMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				summary.SetName("summary-metric")
				summary.SetEmptySummary().DataPoints().AppendEmpty().SetSum(float64(1.0))
				summary.Summary().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

				histogram := testMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				histogram.SetName("histogram-metric")
				_ = histogram.SetEmptyHistogram().DataPoints().AppendEmpty().StartTimestamp().AsTime().Year()
				histogram.Histogram().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

				expHistogram := testMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				expHistogram.SetName("exponential-histogram")
				_ = expHistogram.SetEmptyExponentialHistogram().DataPoints().AppendEmpty().StartTimestamp().AsTime().Year()
				expHistogram.ExponentialHistogram().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

				metrics := testMetrics.ResourceMetrics()
				// Insert the scope_info metric into the existing ScopeMetricsSlice
				sm := metrics.At(0).ScopeMetrics().At(0)
				scopeInfoMetric := sm.Metrics().AppendEmpty()
				scopeInfoMetric.SetName("otel_scope_info")
				scopeInfoMetric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
				scopeInfoMetric.Gauge().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

				// add otel_scope_* attributes to all metrics in this scope (including otel_scope_info)
				for i := 0; i < sm.Metrics().Len(); i++ {
					metric := sm.Metrics().At(i)
					switch metric.Type() {
					case pmetric.MetricTypeSum:
						metric.Sum().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Sum().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
					case pmetric.MetricTypeGauge:
						metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Gauge().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
					case pmetric.MetricTypeSummary:
						metric.Summary().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Summary().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
					case pmetric.MetricTypeHistogram:
						metric.Histogram().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.Histogram().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
					case pmetric.MetricTypeExponentialHistogram:
						metric.ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("otel_scope_name", "myscope")
						metric.ExponentialHistogram().DataPoints().At(0).Attributes().PutStr("otel_scope_version", "v0.0.1")
					}
				}
				return metrics
			}(),
		},
		{
			name:   "add untyped Sum metric from Gauge",
			config: Config{},
			input: func() pmetric.Metrics {
				metrics := testMetric(timestamp)
				metrics.ResourceMetrics().At(0).
					ScopeMetrics().At(0).
					Metrics().At(0).
					Metadata().PutStr("prometheus.type", "unknown")
				return metrics
			}(),
			expected: func() pmetric.ResourceMetricsSlice {
				metrics := testMetric(timestamp).ResourceMetrics()

				metrics.At(0).
					ScopeMetrics().At(0).
					Metrics().At(0).
					Metadata().PutStr("prometheus.type", "unknown")

				metric := metrics.At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				metric.SetName("gauge-metric")
				metric.Metadata().PutStr("prometheus.type", "unknown")
				metric.SetEmptySum().DataPoints().AppendEmpty().SetIntValue(2112)
				metric.Sum().SetIsMonotonic(true)
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				metric.Sum().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

				return metrics
			}(),
		},
		{
			name:   "add untyped Sum metric from Gauge (double value)",
			config: Config{},
			input: func() pmetric.Metrics {
				metrics := testMetric(timestamp)
				metrics.ResourceMetrics().At(0).
					ScopeMetrics().At(0).
					Metrics().At(0).
					Metadata().PutStr("prometheus.type", "unknown")
				metrics.ResourceMetrics().At(0).
					ScopeMetrics().At(0).
					Metrics().At(0).
					Gauge().DataPoints().At(0).
					SetDoubleValue(123.5)
				return metrics
			}(),
			expected: func() pmetric.ResourceMetricsSlice {
				metrics := testMetric(timestamp).ResourceMetrics()

				metrics.At(0).ScopeMetrics().At(0).Metrics().At(0).Metadata().PutStr("prometheus.type", "unknown")
				metrics.At(0).
					ScopeMetrics().At(0).
					Metrics().At(0).
					Gauge().DataPoints().At(0).SetDoubleValue(123.5)

				metric := metrics.At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				metric.SetName("gauge-metric")
				metric.Metadata().PutStr("prometheus.type", "unknown")
				metric.SetEmptySum().DataPoints().AppendEmpty().SetDoubleValue(123.5)
				metric.Sum().SetIsMonotonic(true)
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				metric.Sum().DataPoints().At(0).SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

				return metrics
			}(),
		},
		{
			name:   "untyped Gauge does nothing if feature gate is enabled and key!=unknown",
			config: Config{},
			input: func() pmetric.Metrics {
				metrics := testMetric(timestamp)
				metrics.ResourceMetrics().At(0).
					ScopeMetrics().At(0).
					Metrics().At(0).
					Metadata().PutStr("prometheus.type", "foo")
				return metrics
			}(),
			expected: func() pmetric.ResourceMetricsSlice {
				metrics := testMetric(timestamp).ResourceMetrics()
				metrics.At(0).
					ScopeMetrics().At(0).
					Metrics().At(0).
					Metadata().PutStr("prometheus.type", "foo")
				return metrics
			}(),
		},
		{
			name:   "untyped non-Gauge does nothing if feature gate is enabled",
			config: Config{},
			input: func() pmetric.Metrics {
				metrics := testMetric(timestamp)
				metric := metrics.ResourceMetrics().At(0).
					ScopeMetrics().At(0).
					Metrics().AppendEmpty()

				metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Metadata().PutStr("prometheus.type", "unknown")
				return metrics
			}(),
			expected: func() pmetric.ResourceMetricsSlice {
				metrics := testMetric(timestamp).ResourceMetrics()
				metric := metrics.At(0).
					ScopeMetrics().At(0).
					Metrics().AppendEmpty()

				metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Metadata().PutStr("prometheus.type", "unknown")
				return metrics
			}(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			originalValue := intToDoubleFeatureGate.IsEnabled()
			require.NoError(t, featuregate.GlobalRegistry().Set(intToDoubleFeatureGate.ID(), tc.enableDoubleFeatureGate))
			defer func() {
				require.NoError(t, featuregate.GlobalRegistry().Set(intToDoubleFeatureGate.ID(), originalValue))
			}()
			m := tc.input
			tc.config.ExtraMetrics(m)
			assert.EqualValues(t, tc.expected, m.ResourceMetrics())
		})
	}
}
