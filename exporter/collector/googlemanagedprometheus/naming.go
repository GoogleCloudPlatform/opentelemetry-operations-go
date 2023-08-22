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
	"fmt"
	"strings"
	"unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// GetMetricName returns the metric name with GMP-specific suffixes. The.
func (c Config) GetMetricName(baseName string, metric pmetric.Metric) (string, error) {
	// First, build a name that is compliant with prometheus conventions
	compliantName := prometheus.BuildCompliantName(metric, "", c.AddMetricSuffixes)
	// Second, ad the GMP-specific suffix
	switch metric.Type() {
	case pmetric.MetricTypeSum:
		if !metric.Sum().IsMonotonic() {
			// Non-monotonic sums are converted to GCM gauges
			return compliantName + "/gauge", nil
		}
		return getUnknownMetricName(metric.Sum().DataPoints(), "/counter", "counter", metric.Name(), compliantName), nil
	case pmetric.MetricTypeGauge:
		return getUnknownMetricName(metric.Gauge().DataPoints(), "/gauge", "", metric.Name(), compliantName), nil
	case pmetric.MetricTypeSummary:
		// summaries are sent as the following series:
		// * Sum: prometheus.googleapis.com/<baseName>_sum/summary:counter
		// * Count: prometheus.googleapis.com/<baseName>_count/summary
		// * Quantiles: prometheus.googleapis.com/<baseName>/summary
		if strings.HasSuffix(baseName, "_sum") {
			return compliantName + "_sum/summary:counter", nil
		}
		if strings.HasSuffix(baseName, "_count") {
			return compliantName + "_count/summary", nil
		}
		return compliantName + "/summary", nil
	case pmetric.MetricTypeHistogram:
		return compliantName + "/histogram", nil
	default:
		return "", fmt.Errorf("unsupported metric datatype: %v", metric.Type())
	}
}

// getUnknownMetricSuffix will set the metric suffix for untyped metrics to
// "/unknown" (eg, for Gauge) or "/unknown:{secondarySuffix}" (eg, "/unknown:counter" for Sum).
// It also removes the "_total" suffix on an unknown counter, if this suffix was not present in
// the original metric name before calling prometheus.BuildCompliantName(), which is hacky.
// It is based on the untyped_prometheus_metric data point attribute, and behind a feature gate.
func getUnknownMetricName(points pmetric.NumberDataPointSlice, suffix, secondarySuffix, originalName, compliantName string) string {
	if !untypedDoubleExportFeatureGate.IsEnabled() {
		return compliantName + suffix
	}

	nameTokens := strings.FieldsFunc(
		originalName,
		func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) },
	)

	untyped := false
	newSuffix := suffix
	for i := 0; i < points.Len(); i++ {
		point := points.At(i)
		if val, ok := point.Attributes().Get(GCPOpsAgentUntypedMetricKey); ok && val.AsString() == "true" {
			untyped = true
			// delete the special Ops Agent untyped attribute
			point.Attributes().Remove(GCPOpsAgentUntypedMetricKey)
			newSuffix = "/unknown"
			if len(secondarySuffix) > 0 {
				newSuffix = newSuffix + ":" + secondarySuffix
			}
			// even though we have the suffix, keep looping to remove the attribute from other points, if any
		}
	}

	// de-normalize "_total" suffix for counters where not present on original metric name
	if untyped && nameTokens[len(nameTokens)-1] != "total" && strings.HasSuffix(compliantName, "_total") {
		compliantName = strings.TrimSuffix(compliantName, "_total")
	}
	return compliantName + newSuffix
}
