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

	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestGetMetricName(t *testing.T) {
	baseName := "my_metric_name"
	for _, tc := range []struct {
		desc     string
		datatype pmetric.MetricDataType
		expected string
	}{
		{
			desc:     "sum",
			datatype: pmetric.MetricDataTypeSum,
			expected: baseName + "/counter",
		},
		{
			desc:     "gauge",
			datatype: pmetric.MetricDataTypeGauge,
			expected: baseName + "/gauge",
		},
		{
			desc:     "summary",
			datatype: pmetric.MetricDataTypeSummary,
			expected: baseName + "/summary",
		},
		{
			desc:     "histogram",
			datatype: pmetric.MetricDataTypeHistogram,
			expected: baseName + "/histogram",
		},
		{
			desc:     "other",
			datatype: pmetric.MetricDataTypeExponentialHistogram,
			expected: baseName,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			metric := pdata.NewMetric()
			metric.SetDataType(tc.datatype)
			got := GetMetricName(baseName, metric)
			if got != tc.expected {
				t.Errorf("MetricName(%v, %v)=%v; want %v", baseName, tc.datatype, got, tc.expected)
			}
		})
	}
}
