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
	baseName := "my_metric_name"
	for _, tc := range []struct {
		desc      string
		baseName  string
		expected  string
		datatype  pmetric.MetricDataType
		expectErr bool
	}{
		{
			desc:     "sum",
			baseName: "foo_total",
			datatype: pmetric.MetricDataTypeSum,
			expected: "foo_total/counter",
		},
		{
			desc:     "gauge",
			baseName: "bar",
			datatype: pmetric.MetricDataTypeGauge,
			expected: "bar/gauge",
		},
		{
			desc:     "summary sum",
			baseName: "baz_sum",
			datatype: pmetric.MetricDataTypeSummary,
			expected: "baz_sum/summary:counter",
		},
		{
			desc:     "summary count",
			baseName: "baz_count",
			datatype: pmetric.MetricDataTypeSummary,
			expected: "baz_count/summary",
		},
		{
			desc:     "summary quantile",
			baseName: "baz",
			datatype: pmetric.MetricDataTypeSummary,
			expected: "baz/summary",
		},
		{
			desc:     "histogram sum",
			baseName: "hello_sum",
			datatype: pmetric.MetricDataTypeHistogram,
			expected: "hello_sum/histogram",
		},
		{
			desc:     "histogram count",
			baseName: "hello_count",
			datatype: pmetric.MetricDataTypeHistogram,
			expected: "hello_count/histogram",
		},
		{
			desc:     "histogram bucket",
			baseName: "hello",
			datatype: pmetric.MetricDataTypeHistogram,
			expected: "hello/histogram",
		},
		{
			desc:      "other",
			baseName:  "other",
			datatype:  pmetric.MetricDataTypeExponentialHistogram,
			expectErr: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			metric := pmetric.NewMetric()
			metric.SetDataType(tc.datatype)
			got, err := GetMetricName(tc.baseName, metric)
			if tc.expectErr == (err == nil) {
				t.Errorf("MetricName(%v, %v)=err(%v); want err: %v", baseName, tc.datatype, err, tc.expectErr)
			}
			if got != tc.expected {
				t.Errorf("MetricName(%v, %v)=%v; want %v", baseName, tc.datatype, got, tc.expected)
			}
		})
	}
}
