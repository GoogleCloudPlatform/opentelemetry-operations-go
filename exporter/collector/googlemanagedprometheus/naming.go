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
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func GetMetricName(baseName string, metric pdata.Metric) string {
	return baseName + gmpMetricSuffix(metric)
}

func gmpMetricSuffix(metric pdata.Metric) string {
	switch metric.DataType() {
	case pmetric.MetricDataTypeSum:
		return "/counter"
	case pmetric.MetricDataTypeGauge:
		return "/gauge"
	case pmetric.MetricDataTypeSummary:
		return "/summary"
	case pmetric.MetricDataTypeHistogram:
		return "/histogram"
	default:
		return ""
	}
}
