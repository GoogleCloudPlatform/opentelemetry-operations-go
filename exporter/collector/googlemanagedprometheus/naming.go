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

	"go.opentelemetry.io/collector/pdata/pmetric"
)

func GetMetricName(baseName string, metric pmetric.Metric) (string, error) {
	switch metric.Type() {
	case pmetric.MetricTypeSum:
		return baseName + "/counter", nil
	case pmetric.MetricTypeGauge:
		return baseName + "/gauge", nil
	case pmetric.MetricTypeSummary:
		// summaries are sent as the following series:
		// * Sum: prometheus.googleapis.com/<baseName>_sum/summary:counter
		// * Count: prometheus.googleapis.com/<baseName>_count/summary
		// * Quantiles: prometheus.googleapis.com/<baseName>/summary
		if strings.HasSuffix(baseName, "_sum") {
			return baseName + "/summary:counter", nil
		}
		return baseName + "/summary", nil
	case pmetric.MetricTypeHistogram:
		return baseName + "/histogram", nil
	default:
		return "", fmt.Errorf("unsupported metric datatype: %v", metric.Type())
	}
}
