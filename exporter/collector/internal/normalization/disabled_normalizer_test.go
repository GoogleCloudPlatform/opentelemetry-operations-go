// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package normalization

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestDisabledNormalizerStartTimestamp(t *testing.T) {
	end := pcommon.NewTimestampFromTime(time.Date(2026, 1, 1, 0, 0, 10, 0, time.UTC))
	before := pcommon.NewTimestampFromTime(end.AsTime().Add(-time.Minute))
	adjusted := pcommon.Timestamp(uint64(end) - uint64(time.Millisecond))

	for _, tc := range []struct {
		name  string
		start pcommon.Timestamp
		keep  bool
		want  pcommon.Timestamp
	}{
		{"no start time", 0, false, 0},
		{"start equals end", end, true, adjusted},
		{"start after end", end + 1, true, adjusted},
		{"start before end", before, true, before},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := NewDisabledNormalizer()

			number := pmetric.NewNumberDataPoint()
			number.SetStartTimestamp(tc.start)
			number.SetTimestamp(end)
			if keep := n.NormalizeNumberDataPoint(number, 0); keep != tc.keep {
				t.Errorf("NormalizeNumberDataPoint() keep = %v, want %v", keep, tc.keep)
			}
			if got := number.StartTimestamp(); got != tc.want {
				t.Errorf("NormalizeNumberDataPoint() start = %v, want %v", got, tc.want)
			}

			histogram := pmetric.NewHistogramDataPoint()
			histogram.SetStartTimestamp(tc.start)
			histogram.SetTimestamp(end)
			if keep := n.NormalizeHistogramDataPoint(histogram, 0); keep != tc.keep {
				t.Errorf("NormalizeHistogramDataPoint() keep = %v, want %v", keep, tc.keep)
			}
			if got := histogram.StartTimestamp(); got != tc.want {
				t.Errorf("NormalizeHistogramDataPoint() start = %v, want %v", got, tc.want)
			}

			exponential := pmetric.NewExponentialHistogramDataPoint()
			exponential.SetStartTimestamp(tc.start)
			exponential.SetTimestamp(end)
			if keep := n.NormalizeExponentialHistogramDataPoint(exponential, 0); keep != tc.keep {
				t.Errorf("NormalizeExponentialHistogramDataPoint() keep = %v, want %v", keep, tc.keep)
			}
			if got := exponential.StartTimestamp(); got != tc.want {
				t.Errorf("NormalizeExponentialHistogramDataPoint() start = %v, want %v", got, tc.want)
			}

			summary := pmetric.NewSummaryDataPoint()
			summary.SetStartTimestamp(tc.start)
			summary.SetTimestamp(end)
			if keep := n.NormalizeSummaryDataPoint(summary, 0); keep != tc.keep {
				t.Errorf("NormalizeSummaryDataPoint() keep = %v, want %v", keep, tc.keep)
			}
			if got := summary.StartTimestamp(); got != tc.want {
				t.Errorf("NormalizeSummaryDataPoint() start = %v, want %v", got, tc.want)
			}
		})
	}
}
