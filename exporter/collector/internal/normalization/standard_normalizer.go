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
	"go.uber.org/zap"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/datapointstorage"
)

// NewStandardNormalizer performs normalization on cumulative points which:
//  (a) don't have a start time, OR
//  (b) have been sent a preceding "reset" point as described in https://github.com/open-telemetry/opentelemetry-specification/blob/9555f9594c7ffe5dc333b53da5e0f880026cead1/specification/metrics/datamodel.md#resets-and-gaps
// The first point without a start time or the reset point is cached, and is
// NOT exported. Subsequent points "subtract" the initial point prior to exporting.
// This normalizer also detects subsequent resets, and produces a new start time for those points.
// It doesn't modify values after a reset, but does give it a new start time.
func NewStandardNormalizer(shutdown <-chan struct{}, logger *zap.Logger) Normalizer {
	return &standardNormalizer{
		startCache:    datapointstorage.NewCache(shutdown),
		previousCache: datapointstorage.NewCache(shutdown),
		log:           logger,
	}
}

type standardNormalizer struct {
	startCache    *datapointstorage.Cache
	previousCache *datapointstorage.Cache
	log           *zap.Logger
}

func (s *standardNormalizer) NormalizeExponentialHistogramDataPoint(point pmetric.ExponentialHistogramDataPoint, identifier string) *pmetric.ExponentialHistogramDataPoint {
	start, hasStart := s.startCache.GetExponentialHistogramDataPoint(identifier)
	if !hasStart {
		if point.StartTimestamp() == 0 || !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
			// This is the first time we've seen this metric, or we received
			// an explicit reset point as described in
			// https://github.com/open-telemetry/opentelemetry-specification/blob/9555f9594c7ffe5dc333b53da5e0f880026cead1/specification/metrics/datamodel.md#resets-and-gaps
			// Record it in history and drop the point.
			s.startCache.SetExponentialHistogramDataPoint(identifier, &point)
			s.previousCache.SetExponentialHistogramDataPoint(identifier, &point)
			return nil
		}
		// No normalization required, since we haven't cached anything, and the start TS is non-zero.
		return &point
	}

	// TODO(#366): It is possible, but difficult to compare exponential
	// histograms with different scales. For now, treat a change in
	// scale as a reset.  Drop this point, and normalize against it for
	// subsequent points.
	if point.Scale() != start.Scale() {
		s.startCache.SetExponentialHistogramDataPoint(identifier, &point)
		s.previousCache.SetExponentialHistogramDataPoint(identifier, &point)
		return nil
	}

	previous, hasPrevious := s.previousCache.GetExponentialHistogramDataPoint(identifier)
	if !hasPrevious {
		// This should never happen, but fall-back to the start point if we
		// don't find a previous point
		previous = start
	}
	if !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) ||
		(point.StartTimestamp() == 0 && lessThanExponentialHistogramDataPoint(&point, previous)) {
		// Make a copy so we don't mutate underlying data
		newPoint := pmetric.NewExponentialHistogramDataPoint()
		// This is a reset point, but we have seen this timeseries before, so we know the reset happened in the time period since the last point.
		// Assume the reset occurred at T - 1 ms, and leave the value untouched.
		point.CopyTo(newPoint)
		newPoint.SetStartTimestamp(pcommon.Timestamp(uint64(point.Timestamp()) - uint64(time.Millisecond)))
		s.previousCache.SetExponentialHistogramDataPoint(identifier, &newPoint)
		// For subsequent points, we don't want to modify the value, but we do
		// want to make the start timestamp match the point we write here.
		// Store a point with the same timestamps, but zero value to achieve
		// that behavior.
		zeroPoint := pmetric.NewExponentialHistogramDataPoint()
		zeroPoint.SetTimestamp(newPoint.StartTimestamp())
		zeroPoint.SetScale(newPoint.Scale())
		s.startCache.SetExponentialHistogramDataPoint(identifier, &zeroPoint)
		return &newPoint
	}
	if !start.Timestamp().AsTime().Before(point.Timestamp().AsTime()) {
		// We found a cached start timestamp that wouldn't produce a valid point.
		// Drop it and log.
		s.log.Info(
			"data point being processed older than last recorded reset, will not be emitted",
			zap.String("lastRecordedReset", start.Timestamp().String()),
			zap.String("dataPoint", point.Timestamp().String()),
		)
		return nil
	}
	// There was no reset, so normalize the point against the start point
	newPoint := subtractExponentialHistogramDataPoint(&point, start)
	s.previousCache.SetExponentialHistogramDataPoint(identifier, newPoint)
	return newPoint
}

// lessThanExponentialHistogramDataPoint returns a < b
func lessThanExponentialHistogramDataPoint(a, b *pmetric.ExponentialHistogramDataPoint) bool {
	return a.Count() < b.Count() || a.Sum() < b.Sum()
}

// subtractExponentialHistogramDataPoint returns a - b
func subtractExponentialHistogramDataPoint(a, b *pmetric.ExponentialHistogramDataPoint) *pmetric.ExponentialHistogramDataPoint {
	// Make a copy so we don't mutate underlying data
	newPoint := pmetric.NewExponentialHistogramDataPoint()
	a.CopyTo(newPoint)
	// Use the timestamp from the normalization point
	newPoint.SetStartTimestamp(b.Timestamp())
	// Adjust the value based on the start point's value
	newPoint.SetCount(a.Count() - b.Count())
	// We drop points without a sum, so no need to check here.
	newPoint.SetSum(a.Sum() - b.Sum())
	newPoint.SetZeroCount(a.ZeroCount() - b.ZeroCount())
	newPoint.Positive().SetMBucketCounts(subtractExponentialBuckets(a.Positive(), b.Positive()))
	newPoint.Negative().SetMBucketCounts(subtractExponentialBuckets(a.Negative(), b.Negative()))
	return &newPoint
}

// subtractExponentialBuckets returns a - b
func subtractExponentialBuckets(a, b pmetric.Buckets) []uint64 {
	newBuckets := make([]uint64, len(a.MBucketCounts()))
	offsetDiff := int(a.Offset() - b.Offset())
	for i := range a.MBucketCounts() {
		bOffset := i + offsetDiff
		// if there is no corresponding bucket for the starting MBucketCounts, don't normalize
		if bOffset < 0 || bOffset >= len(b.MBucketCounts()) {
			newBuckets[i] = a.MBucketCounts()[i]
		} else {
			newBuckets[i] = a.MBucketCounts()[i] - b.MBucketCounts()[bOffset]
		}
	}
	return newBuckets
}

func (s *standardNormalizer) NormalizeHistogramDataPoint(point pmetric.HistogramDataPoint, identifier string) *pmetric.HistogramDataPoint {
	start, hasStart := s.startCache.GetHistogramDataPoint(identifier)
	if !hasStart {
		if point.StartTimestamp() == 0 || !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
			// This is the first time we've seen this metric, or we received
			// an explicit reset point as described in
			// https://github.com/open-telemetry/opentelemetry-specification/blob/9555f9594c7ffe5dc333b53da5e0f880026cead1/specification/metrics/datamodel.md#resets-and-gaps
			// Record it in history and drop the point.
			s.startCache.SetHistogramDataPoint(identifier, &point)
			s.previousCache.SetHistogramDataPoint(identifier, &point)
			return nil
		}
		// No normalization required, since we haven't cached anything, and the start TS is non-zero.
		return &point
	}

	// The number of buckets changed, so we can't normalize points anymore.
	// Treat this as a reset.
	if !bucketBoundariesEqual(point.MExplicitBounds(), start.MExplicitBounds()) {
		s.startCache.SetHistogramDataPoint(identifier, &point)
		s.previousCache.SetHistogramDataPoint(identifier, &point)
		return nil
	}

	previous, hasPrevious := s.previousCache.GetHistogramDataPoint(identifier)
	if !hasPrevious {
		// This should never happen, but fall-back to the start point if we
		// don't find a previous point
		previous = start
	}
	if !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) ||
		(point.StartTimestamp() == 0 && lessThanHistogramDataPoint(&point, previous)) {
		// Make a copy so we don't mutate underlying data
		newPoint := pmetric.NewHistogramDataPoint()
		// This is a reset point, but we have seen this timeseries before, so we know the reset happened in the time period since the last point.
		// Assume the reset occurred at T - 1 ms, and leave the value untouched.
		point.CopyTo(newPoint)
		newPoint.SetStartTimestamp(pcommon.Timestamp(uint64(point.Timestamp()) - uint64(time.Millisecond)))
		s.previousCache.SetHistogramDataPoint(identifier, &newPoint)
		// For subsequent points, we don't want to modify the value, but we do
		// want to make the start timestamp match the point we write here.
		// Store a point with the same timestamps, but zero value to achieve
		// that behavior.
		zeroPoint := pmetric.NewHistogramDataPoint()
		zeroPoint.SetTimestamp(newPoint.StartTimestamp())
		zeroPoint.SetMExplicitBounds(newPoint.MExplicitBounds())
		zeroPoint.SetMBucketCounts(make([]uint64, len(newPoint.MBucketCounts())))
		s.startCache.SetHistogramDataPoint(identifier, &zeroPoint)
		return &newPoint
	}
	if !start.Timestamp().AsTime().Before(point.Timestamp().AsTime()) {
		// We found a cached start timestamp that wouldn't produce a valid point.
		// Drop it and log.
		s.log.Info(
			"data point being processed older than last recorded reset, will not be emitted",
			zap.String("lastRecordedReset", start.Timestamp().String()),
			zap.String("dataPoint", point.Timestamp().String()),
		)
		return nil
	}
	// There was no reset, so normalize the point against the start point
	newPoint := subtractHistogramDataPoint(&point, start)
	s.previousCache.SetHistogramDataPoint(identifier, newPoint)
	return newPoint
}

// lessThanHistogramDataPoint returns a < b
func lessThanHistogramDataPoint(a, b *pmetric.HistogramDataPoint) bool {
	return a.Count() < b.Count() || a.Sum() < b.Sum()
}

// subtractHistogramDataPoint returns a - b
func subtractHistogramDataPoint(a, b *pmetric.HistogramDataPoint) *pmetric.HistogramDataPoint {
	// Make a copy so we don't mutate underlying data
	newPoint := pmetric.NewHistogramDataPoint()
	a.CopyTo(newPoint)
	// Use the timestamp from the normalization point
	newPoint.SetStartTimestamp(b.Timestamp())
	// Adjust the value based on the start point's value
	newPoint.SetCount(a.Count() - b.Count())
	// We drop points without a sum, so no need to check here.
	newPoint.SetSum(a.Sum() - b.Sum())
	aBuckets := a.MBucketCounts()
	bBuckets := b.MBucketCounts()
	newBuckets := make([]uint64, len(aBuckets))
	for i := range aBuckets {
		newBuckets[i] = aBuckets[i] - bBuckets[i]
	}
	newPoint.SetMBucketCounts(newBuckets)
	return &newPoint
}

func bucketBoundariesEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// NormalizeNumberDataPoint normalizes a cumulative, monotonic sum.
// It returns the normalized point, or nil if the point should be dropped.
func (s *standardNormalizer) NormalizeNumberDataPoint(point pmetric.NumberDataPoint, identifier string) *pmetric.NumberDataPoint {
	start, hasStart := s.startCache.GetNumberDataPoint(identifier)
	if !hasStart {
		if point.StartTimestamp() == 0 || !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
			// This is the first time we've seen this metric, or we received
			// an explicit reset point as described in
			// https://github.com/open-telemetry/opentelemetry-specification/blob/9555f9594c7ffe5dc333b53da5e0f880026cead1/specification/metrics/datamodel.md#resets-and-gaps
			// Record it in history and drop the point.
			s.startCache.SetNumberDataPoint(identifier, &point)
			s.previousCache.SetNumberDataPoint(identifier, &point)
			return nil
		}
		// No normalization required, since we haven't cached anything, and the start TS is non-zer0.
		return &point
	}

	previous, hasPrevious := s.previousCache.GetNumberDataPoint(identifier)
	if !hasPrevious {
		// This should never happen, but fall-back to the start point if we
		// don't find a previous point
		previous = start
	}
	if !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) || (point.StartTimestamp() == 0 && lessThanNumberDataPoint(&point, previous)) {
		// Make a copy so we don't mutate underlying data
		newPoint := pmetric.NewNumberDataPoint()
		// This is a reset point, but we have seen this timeseries before, so we know the reset happened in the time period since the last point.
		// Assume the reset occurred at T - 1 ms, and leave the value untouched.
		point.CopyTo(newPoint)
		newPoint.SetStartTimestamp(pcommon.Timestamp(uint64(point.Timestamp()) - uint64(time.Millisecond)))
		s.previousCache.SetNumberDataPoint(identifier, &newPoint)
		// For subsequent points, we don't want to modify the value, but we do
		// want to make the start timestamp match the point we write here.
		// Store a point with the same timestamps, but zero value to achieve
		// that behavior.
		zeroPoint := pmetric.NewNumberDataPoint()
		zeroPoint.SetTimestamp(newPoint.StartTimestamp())
		s.startCache.SetNumberDataPoint(identifier, &zeroPoint)
		return &newPoint
	}
	if !start.Timestamp().AsTime().Before(point.Timestamp().AsTime()) {
		// We found a cached start timestamp that wouldn't produce a valid point.
		// Drop it and log.
		s.log.Info(
			"data point being processed older than last recorded reset, will not be emitted",
			zap.String("lastRecordedReset", start.Timestamp().String()),
			zap.String("dataPoint", point.Timestamp().String()),
		)
		return nil
	}
	// There was no reset, so normalize the point against the start point
	newPoint := subtractNumberDataPoint(&point, start)
	s.previousCache.SetNumberDataPoint(identifier, newPoint)
	return newPoint
}

// lessThanNumberDataPoint returns a < b
func lessThanNumberDataPoint(a, b *pmetric.NumberDataPoint) bool {
	switch a.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		return a.IntVal() < b.IntVal()
	case pmetric.NumberDataPointValueTypeDouble:
		return a.DoubleVal() < b.DoubleVal()
	}
	return false
}

// subtractNumberDataPoint returns a - b
func subtractNumberDataPoint(a, b *pmetric.NumberDataPoint) *pmetric.NumberDataPoint {
	// Make a copy so we don't mutate underlying data
	newPoint := pmetric.NewNumberDataPoint()
	a.CopyTo(newPoint)
	// Use the timestamp from the normalization point
	newPoint.SetStartTimestamp(b.Timestamp())
	// Adjust the value based on the start point's value
	switch newPoint.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		newPoint.SetIntVal(a.IntVal() - b.IntVal())
	case pmetric.NumberDataPointValueTypeDouble:
		newPoint.SetDoubleVal(a.DoubleVal() - b.DoubleVal())
	}
	return &newPoint
}

func (s *standardNormalizer) NormalizeSummaryDataPoint(point pmetric.SummaryDataPoint, identifier string) *pmetric.SummaryDataPoint {
	start, hasStart := s.startCache.GetSummaryDataPoint(identifier)
	if !hasStart {
		if point.StartTimestamp() == 0 || !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
			// This is the first time we've seen this metric, or we received
			// an explicit reset point as described in
			// https://github.com/open-telemetry/opentelemetry-specification/blob/9555f9594c7ffe5dc333b53da5e0f880026cead1/specification/metrics/datamodel.md#resets-and-gaps
			// Record it in history and drop the point.
			s.startCache.SetSummaryDataPoint(identifier, &point)
			s.previousCache.SetSummaryDataPoint(identifier, &point)
			return nil
		}
		// No normalization required, since we haven't cached anything, and the start TS is non-zer0.
		return &point
	}

	previous, hasPrevious := s.previousCache.GetSummaryDataPoint(identifier)
	if !hasPrevious {
		// This should never happen, but fall-back to the start point if we
		// don't find a previous point
		previous = start
	}
	if !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) || (point.StartTimestamp() == 0 && lessThanSummaryDataPoint(&point, previous)) {
		// Make a copy so we don't mutate underlying data
		newPoint := pmetric.NewSummaryDataPoint()
		// This is a reset point, but we have seen this timeseries before, so we know the reset happened in the time period since the last point.
		// Assume the reset occurred at T - 1 ms, and leave the value untouched.
		point.CopyTo(newPoint)
		newPoint.SetStartTimestamp(pcommon.Timestamp(uint64(point.Timestamp()) - uint64(time.Millisecond)))
		s.previousCache.SetSummaryDataPoint(identifier, &newPoint)
		// For subsequent points, we don't want to modify the value, but we do
		// want to make the start timestamp match the point we write here.
		// Store a point with the same timestamps, but zero value to achieve
		// that behavior.
		zeroPoint := pmetric.NewSummaryDataPoint()
		zeroPoint.SetTimestamp(newPoint.StartTimestamp())
		s.startCache.SetSummaryDataPoint(identifier, &zeroPoint)
		return &newPoint
	}
	if !start.Timestamp().AsTime().Before(point.Timestamp().AsTime()) {
		// We found a cached start timestamp that wouldn't produce a valid point.
		// Drop it and log.
		s.log.Info(
			"data point being processed older than last recorded reset, will not be emitted",
			zap.String("lastRecordedReset", start.Timestamp().String()),
			zap.String("dataPoint", point.Timestamp().String()),
		)
		return nil
	}
	// There was no reset, so normalize the point against the start point
	newPoint := subtractSummaryDataPoint(&point, start)
	s.previousCache.SetSummaryDataPoint(identifier, newPoint)
	return newPoint
}

// lessThanSummaryDataPoint returns a < b
func lessThanSummaryDataPoint(a, b *pmetric.SummaryDataPoint) bool {
	return a.Count() < b.Count() || a.Sum() < b.Sum()
}

// subtractSummaryDataPoint returns a - b
func subtractSummaryDataPoint(a, b *pmetric.SummaryDataPoint) *pmetric.SummaryDataPoint {
	// Make a copy so we don't mutate underlying data.
	newPoint := pmetric.NewSummaryDataPoint()
	// Quantile values are copied, and are not modified. Quantiles are
	// computed over the same time period as sum and count, but it isn't
	// possible to normalize them.
	a.CopyTo(newPoint)
	// Use the timestamp from the normalization point
	newPoint.SetStartTimestamp(b.Timestamp())
	// Adjust the value based on the start point's value
	newPoint.SetCount(a.Count() - b.Count())
	// We drop points without a sum, so no need to check here.
	newPoint.SetSum(a.Sum() - b.Sum())
	return &newPoint
}
