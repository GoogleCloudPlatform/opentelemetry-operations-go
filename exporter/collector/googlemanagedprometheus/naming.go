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

	"go.opentelemetry.io/collector/pdata/pmetric"
)

// GetMetricName returns the GMP-specific suffix. baseName includes type (e.g. summary) suffixes.
// compliantName is the name of the metric with any sanitization already performed.
func GetMetricName(baseName, compliantName string, metric pmetric.Metric) (string, error) {
	// Second, ad the GMP-specific suffix
	switch metric.Type() {
	case pmetric.MetricTypeSum:
		if !metric.Sum().IsMonotonic() {
			// Non-monotonic sums are converted to GCM gauges
			return compliantName + "/gauge", nil
		}
		return getUnknownMetricName(metric, metric.Sum().DataPoints(), "/counter", "counter", compliantName), nil
	case pmetric.MetricTypeGauge:
		return getUnknownMetricName(metric, metric.Gauge().DataPoints(), "/gauge", "", compliantName), nil
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
		fallthrough
	case pmetric.MetricTypeExponentialHistogram:
		return compliantName + "/histogram", nil
	default:
		return "", fmt.Errorf("unsupported metric datatype: %v", metric.Type())
	}
}

// getUnknownMetricSuffix will set the metric suffix for untyped metrics to
// "/unknown" (eg, for Gauge) or "/unknown:{secondarySuffix}" (eg, "/unknown:counter" for Sum).
// It also removes the "_total" suffix on an unknown counter, if this suffix was not present in
// the original metric name before calling prometheus.BuildCompliantName(), which is hacky.
func getUnknownMetricName(metric pmetric.Metric, points pmetric.NumberDataPointSlice, suffix, secondarySuffix, compliantName string) string {
	nameTokens := strings.FieldsFunc(
		metric.Name(),
		func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) },
	)

	newSuffix := suffix
	if isUnknown(metric) {
		newSuffix = "/unknown"
		if len(secondarySuffix) > 0 {
			newSuffix = newSuffix + ":" + secondarySuffix
		}

		// de-normalize "_total" suffix for counters where not present on original metric name
		if nameTokens[len(nameTokens)-1] != "total" && strings.HasSuffix(compliantName, "_total") {
			compliantName = strings.TrimSuffix(compliantName, "_total")
		}
	}
	return compliantName + newSuffix
}
