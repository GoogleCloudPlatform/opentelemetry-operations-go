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

func TestAddTargetInfo(t *testing.T) {
	for _, tc := range []struct {
		input    pmetric.Metrics
		expected pmetric.ResourceMetricsSlice
		name     string
	}{
		{
			name: "add target info from resource metric",
			input: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()

				// foo-label should be copied to target_info, not locationLabel
				rm.Resource().Attributes().PutStr(locationLabel, "us-east")
				rm.Resource().Attributes().PutStr("foo-label", "bar")

				// scope should not be copied to target_info
				sm := rm.ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("myscope")

				// other metrics should not be copied to target_info
				metric := sm.Metrics().AppendEmpty()
				metric.SetName("baz-metric")
				metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(2112)
				return metrics
			}(),
			expected: func() pmetric.ResourceMetricsSlice {
				rms := pmetric.NewResourceMetricsSlice()
				rm := rms.AppendEmpty()

				// resource attributes will be changed to a MonitoredResource on export, not in AddTargetInfo
				// therefore they should still be present here
				rm.Resource().Attributes().PutStr(locationLabel, "us-east")
				rm.Resource().Attributes().PutStr("foo-label", "bar")
				sm := rm.ScopeMetrics().AppendEmpty()
				metric := sm.Metrics().AppendEmpty()
				metric.SetName("target_info")
				metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
				metric.Gauge().DataPoints().At(0).Attributes().PutStr("foo-label", "bar")
				return rms
			}(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rms := AddTargetInfo(tc.input)
			assert.EqualValues(t, tc.expected, rms)
		})
	}
}
