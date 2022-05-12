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

package normalization

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// NewDisabledNormalizer returns a Normalizer which does not perform any
// normalization. This may be useful if standard normalization consumes too
// much memory. For explicit reset points described in
// https://github.com/open-telemetry/opentelemetry-specification/blob/9555f9594c7ffe5dc333b53da5e0f880026cead1/specification/metrics/datamodel.md#resets-and-gaps
// it adjusts the end time to be after the start time to make the points valid
// without changing the start time (which causes the point to reset).
func NewDisabledNormalizer() Normalizer {
	return &disabledNormalizer{}
}

type disabledNormalizer struct{}

// NormalizeExponentialHistogramDataPoint returns the point without normalizing.
func (d *disabledNormalizer) NormalizeExponentialHistogramDataPoint(point pmetric.ExponentialHistogramDataPoint, _ string) *pmetric.ExponentialHistogramDataPoint {
	if !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
		// Handle explicit reset points.
		// Make a copy so we don't mutate underlying data.
		newPoint := pmetric.NewExponentialHistogramDataPoint()
		point.CopyTo(newPoint)
		// StartTime = Timestamp - 1 ms
		newPoint.SetStartTimestamp(pcommon.Timestamp(uint64(point.Timestamp()) - uint64(time.Millisecond)))
		return &newPoint
	}
	return &point
}

// NormalizeHistogramDataPoint returns the point without normalizing.
func (d *disabledNormalizer) NormalizeHistogramDataPoint(point pmetric.HistogramDataPoint, _ string) *pmetric.HistogramDataPoint {
	if !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
		// Handle explicit reset points.
		// Make a copy so we don't mutate underlying data.
		newPoint := pmetric.NewHistogramDataPoint()
		point.CopyTo(newPoint)
		// StartTime = Timestamp - 1 ms
		newPoint.SetStartTimestamp(pcommon.Timestamp(uint64(point.Timestamp()) - uint64(time.Millisecond)))
		return &newPoint
	}
	return &point
}

// NormalizeNumberDataPoint returns the point without normalizing.
func (d *disabledNormalizer) NormalizeNumberDataPoint(point pmetric.NumberDataPoint, _ string) *pmetric.NumberDataPoint {
	if !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
		// Handle explicit reset points.
		// Make a copy so we don't mutate underlying data.
		newPoint := pmetric.NewNumberDataPoint()
		point.CopyTo(newPoint)
		// StartTime = Timestamp - 1 ms
		newPoint.SetStartTimestamp(pcommon.Timestamp(uint64(point.Timestamp()) - uint64(time.Millisecond)))
		return &newPoint
	}
	return &point
}

// NormalizeSummaryDataPoint returns the point without normalizing.
func (d *disabledNormalizer) NormalizeSummaryDataPoint(point pmetric.SummaryDataPoint, _ string) *pmetric.SummaryDataPoint {
	if !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
		// Handle explicit reset points.
		// Make a copy so we don't mutate underlying data.
		newPoint := pmetric.NewSummaryDataPoint()
		point.CopyTo(newPoint)
		// StartTime = Timestamp - 1 ms
		newPoint.SetStartTimestamp(pcommon.Timestamp(uint64(point.Timestamp()) - uint64(time.Millisecond)))
		return &newPoint
	}
	return &point
}
