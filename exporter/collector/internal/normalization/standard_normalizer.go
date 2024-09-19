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
//
//	(a) don't have a start time, OR
//	(b) have been sent a preceding "reset" point as described in https://github.com/open-telemetry/opentelemetry-specification/blob/9555f9594c7ffe5dc333b53da5e0f880026cead1/specification/metrics/datamodel.md#resets-and-gaps
//
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

func (s *standardNormalizer) NormalizeExponentialHistogramDataPoint(point pmetric.ExponentialHistogramDataPoint, identifier uint64) bool {
	start, hasStart := s.startCache.GetExponentialHistogramDataPoint(identifier)
	if !hasStart {
		if point.StartTimestamp() == 0 || !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
			// This is the first time we've seen this metric, or we received
			// an explicit reset point as described in
			// https://github.com/open-telemetry/opentelemetry-specification/blob/9555f9594c7ffe5dc333b53da5e0f880026cead1/specification/metrics/datamodel.md#resets-and-gaps
			// Record it in history and drop the point.
			s.startCache.SetExponentialHistogramDataPoint(identifier, point)
			s.previousCache.SetExponentialHistogramDataPoint(identifier, point)
			return false
		}
		// No normalization required, since we haven't cached anything, and the start TS is non-zero.
		return true
	}

	// TODO(#366): It is possible, but difficult to compare exponential
	// histograms with different scales. For now, treat a change in
	// scale as a reset.  Drop this point, and normalize against it for
	// subsequent points.
	if point.Scale() != start.Scale() {
		s.startCache.SetExponentialHistogramDataPoint(identifier, point)
		s.previousCache.SetExponentialHistogramDataPoint(identifier, point)
		return false
	}

	previous, hasPrevious := s.previousCache.GetExponentialHistogramDataPoint(identifier)
	if !hasPrevious {
		// This should never happen, but fall-back to the start point if we
		// don't find a previous point
		previous = start
	}
	if !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) ||
		(point.StartTimestamp() == 0 && lessThanExponentialHistogramDataPoint(point, previous)) {
		// This is a reset point, but we have seen this timeseries before, so we know the reset happened in the time period since the last point.
		// Assume the reset occurred at T - 1 ms, and leave the value untouched.
		point.SetStartTimestamp(pcommon.Timestamp(uint64(point.Timestamp()) - uint64(time.Millisecond)))
		s.previousCache.SetExponentialHistogramDataPoint(identifier, point)
		// For subsequent points, we don't want to modify the value, but we do
		// want to make the start timestamp match the point we write here.
		// Store a point with the same timestamps, but zero value to achieve
		// that behavior.
		zeroPoint := pmetric.NewExponentialHistogramDataPoint()
		zeroPoint.SetTimestamp(point.StartTimestamp())
		zeroPoint.SetScale(point.Scale())
		s.startCache.SetExponentialHistogramDataPoint(identifier, zeroPoint)
		return true
	}
	if !start.Timestamp().AsTime().Before(point.Timestamp().AsTime()) {
		// We found a cached start timestamp that wouldn't produce a valid point.
		// Drop it and log.
		s.log.Info(
			"data point being processed older than last recorded reset, will not be emitted",
			zap.String("lastRecordedReset", start.Timestamp().String()),
			zap.String("dataPoint", point.Timestamp().String()),
		)
		return false
	}
	// There was no reset, so normalize the point against the start point
	subtractExponentialHistogramDataPoint(point, start)
	s.previousCache.SetExponentialHistogramDataPoint(identifier, point)
	return true
}

// lessThanExponentialHistogramDataPoint returns a < b.
func lessThanExponentialHistogramDataPoint(a, b pmetric.ExponentialHistogramDataPoint) bool {
	return a.Count() < b.Count() || a.Sum() < b.Sum()
}

// subtractExponentialHistogramDataPoint subtracts b from a.
func subtractExponentialHistogramDataPoint(a, b pmetric.ExponentialHistogramDataPoint) {
	// Use the timestamp from the normalization point
	a.SetStartTimestamp(b.Timestamp())
	// Adjust the value based on the start point's value
	a.SetCount(a.Count() - b.Count())
	// We drop points without a sum, so no need to check here.
	a.SetSum(a.Sum() - b.Sum())
	a.SetZeroCount(a.ZeroCount() - b.ZeroCount())
	a.Positive().BucketCounts().FromRaw(subtractExponentialBuckets(a.Positive(), b.Positive()))
	a.Negative().BucketCounts().FromRaw(subtractExponentialBuckets(a.Negative(), b.Negative()))
}

// subtractExponentialBuckets subtracts b from a.
func subtractExponentialBuckets(a, b pmetric.ExponentialHistogramDataPointBuckets) []uint64 {
	newBuckets := make([]uint64, a.BucketCounts().Len())
	offsetDiff := int(a.Offset() - b.Offset())
	for i := 0; i < a.BucketCounts().Len(); i++ {
		bOffset := i + offsetDiff
		// if there is no corresponding bucket for the starting BucketCounts, don't normalize
		if bOffset < 0 || bOffset >= b.BucketCounts().Len() {
			newBuckets[i] = a.BucketCounts().At(i)
		} else {
			newBuckets[i] = a.BucketCounts().At(i) - b.BucketCounts().At(bOffset)
		}
	}
	return newBuckets
}

func (s *standardNormalizer) NormalizeHistogramDataPoint(point pmetric.HistogramDataPoint, identifier uint64) bool {
	start, hasStart := s.startCache.GetHistogramDataPoint(identifier)
	if !hasStart {
		if point.StartTimestamp() == 0 || !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
			// This is the first time we've seen this metric, or we received
			// an explicit reset point as described in
			// https://github.com/open-telemetry/opentelemetry-specification/blob/9555f9594c7ffe5dc333b53da5e0f880026cead1/specification/metrics/datamodel.md#resets-and-gaps
			// Record it in history and drop the point.
			s.startCache.SetHistogramDataPoint(identifier, point)
			s.previousCache.SetHistogramDataPoint(identifier, point)
			return false
		}
		// No normalization required, since we haven't cached anything, and the start TS is non-zero.
		return true
	}

	// The number of buckets changed, so we can't normalize points anymore.
	// Treat this as a reset.
	if !bucketBoundariesEqual(point.ExplicitBounds(), start.ExplicitBounds()) {
		s.startCache.SetHistogramDataPoint(identifier, point)
		s.previousCache.SetHistogramDataPoint(identifier, point)
		return false
	}

	previous, hasPrevious := s.previousCache.GetHistogramDataPoint(identifier)
	if !hasPrevious {
		// This should never happen, but fall-back to the start point if we
		// don't find a previous point
		previous = start
	}
	if !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) ||
		(point.StartTimestamp() == 0 && lessThanHistogramDataPoint(point, previous)) {
		// This is a reset point, but we have seen this timeseries before, so we know the reset happened in the time period since the last point.
		// Assume the reset occurred at T - 1 ms, and leave the value untouched.
		point.SetStartTimestamp(pcommon.Timestamp(uint64(point.Timestamp()) - uint64(time.Millisecond)))
		s.previousCache.SetHistogramDataPoint(identifier, point)
		// For subsequent points, we don't want to modify the value, but we do
		// want to make the start timestamp match the point we write here.
		// Store a point with the same timestamps, but zero value to achieve
		// that behavior.
		zeroPoint := pmetric.NewHistogramDataPoint()
		zeroPoint.SetTimestamp(point.StartTimestamp())
		point.ExplicitBounds().CopyTo(zeroPoint.ExplicitBounds())
		zeroPoint.BucketCounts().FromRaw(make([]uint64, point.BucketCounts().Len()))
		s.startCache.SetHistogramDataPoint(identifier, zeroPoint)
		return true
	}
	if !start.Timestamp().AsTime().Before(point.Timestamp().AsTime()) {
		// We found a cached start timestamp that wouldn't produce a valid point.
		// Drop it and log.
		s.log.Info(
			"data point being processed older than last recorded reset, will not be emitted",
			zap.String("lastRecordedReset", start.Timestamp().String()),
			zap.String("dataPoint", point.Timestamp().String()),
		)
		return false
	}
	// There was no reset, so normalize the point against the start point
	subtractHistogramDataPoint(point, start)
	s.previousCache.SetHistogramDataPoint(identifier, point)
	return true
}

// lessThanHistogramDataPoint returns a < b.
func lessThanHistogramDataPoint(a, b pmetric.HistogramDataPoint) bool {
	return a.Count() < b.Count() || a.Sum() < b.Sum()
}

// subtractHistogramDataPoint subtracts b from a.
func subtractHistogramDataPoint(a, b pmetric.HistogramDataPoint) {
	// Use the timestamp from the normalization point
	a.SetStartTimestamp(b.Timestamp())
	// Adjust the value based on the start point's value
	a.SetCount(a.Count() - b.Count())
	// We drop points without a sum, so no need to check here.
	a.SetSum(a.Sum() - b.Sum())
	aBuckets := a.BucketCounts()
	bBuckets := b.BucketCounts()
	newBuckets := make([]uint64, aBuckets.Len())
	for i := 0; i < aBuckets.Len(); i++ {
		newBuckets[i] = aBuckets.At(i) - bBuckets.At(i)
	}
	a.BucketCounts().FromRaw(newBuckets)
}

func bucketBoundariesEqual(a, b pcommon.Float64Slice) bool {
	if a.Len() != b.Len() {
		return false
	}
	for i := 0; i < a.Len(); i++ {
		if a.At(i) != b.At(i) {
			return false
		}
	}
	return true
}

// NormalizeNumberDataPoint normalizes a cumulative, monotonic sum.
// It returns the normalized point, and true if the point should be kept.
func (s *standardNormalizer) NormalizeNumberDataPoint(point pmetric.NumberDataPoint, identifier uint64) bool {
	start, hasStart := s.startCache.GetNumberDataPoint(identifier)
	if !hasStart {
		if point.StartTimestamp() == 0 || !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
			// This is the first time we've seen this metric, or we received
			// an explicit reset point as described in
			// https://github.com/open-telemetry/opentelemetry-specification/blob/9555f9594c7ffe5dc333b53da5e0f880026cead1/specification/metrics/datamodel.md#resets-and-gaps
			// Record it in history and drop the point.
			s.startCache.SetNumberDataPoint(identifier, point)
			s.previousCache.SetNumberDataPoint(identifier, point)
			return false
		}
		// No normalization required, since we haven't cached anything, and the start TS is non-zer0.
		return true
	}

	previous, hasPrevious := s.previousCache.GetNumberDataPoint(identifier)
	if !hasPrevious {
		// This should never happen, but fall-back to the start point if we
		// don't find a previous point
		previous = start
	}
	if !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) || (point.StartTimestamp() == 0 && lessThanNumberDataPoint(point, previous)) {
		// This is a reset point, but we have seen this timeseries before, so we know the reset happened in the time period since the last point.
		// Assume the reset occurred at T - 1 ms, and leave the value untouched.
		point.SetStartTimestamp(pcommon.Timestamp(uint64(point.Timestamp()) - uint64(time.Millisecond)))
		s.previousCache.SetNumberDataPoint(identifier, point)
		// For subsequent points, we don't want to modify the value, but we do
		// want to make the start timestamp match the point we write here.
		// Store a point with the same timestamps, but zero value to achieve
		// that behavior.
		zeroPoint := pmetric.NewNumberDataPoint()
		zeroPoint.SetTimestamp(point.StartTimestamp())
		s.startCache.SetNumberDataPoint(identifier, zeroPoint)
		return true
	}
	if !start.Timestamp().AsTime().Before(point.Timestamp().AsTime()) {
		// We found a cached start timestamp that wouldn't produce a valid point.
		// Drop it and log.
		s.log.Info(
			"data point being processed older than last recorded reset, will not be emitted",
			zap.String("lastRecordedReset", start.Timestamp().String()),
			zap.String("dataPoint", point.Timestamp().String()),
		)
		return false
	}
	// There was no reset, so normalize the point against the start point
	subtractNumberDataPoint(point, start)
	s.previousCache.SetNumberDataPoint(identifier, point)
	return true
}

// lessThanNumberDataPoint returns a < b.
func lessThanNumberDataPoint(a, b pmetric.NumberDataPoint) bool {
	switch a.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		return a.IntValue() < b.IntValue()
	case pmetric.NumberDataPointValueTypeDouble:
		return a.DoubleValue() < b.DoubleValue()
	}
	return false
}

// subtractNumberDataPoint subtracts b from a.
func subtractNumberDataPoint(a, b pmetric.NumberDataPoint) {
	// Use the timestamp from the normalization point
	a.SetStartTimestamp(b.Timestamp())
	// Adjust the value based on the start point's value
	switch a.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		a.SetIntValue(a.IntValue() - b.IntValue())
	case pmetric.NumberDataPointValueTypeDouble:
		a.SetDoubleValue(a.DoubleValue() - b.DoubleValue())
	}
}

func (s *standardNormalizer) NormalizeSummaryDataPoint(point pmetric.SummaryDataPoint, identifier uint64) bool {
	start, hasStart := s.startCache.GetSummaryDataPoint(identifier)
	if !hasStart {
		if point.StartTimestamp() == 0 || !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
			// This is the first time we've seen this metric, or we received
			// an explicit reset point as described in
			// https://github.com/open-telemetry/opentelemetry-specification/blob/9555f9594c7ffe5dc333b53da5e0f880026cead1/specification/metrics/datamodel.md#resets-and-gaps
			// Record it in history and drop the point.
			s.startCache.SetSummaryDataPoint(identifier, point)
			s.previousCache.SetSummaryDataPoint(identifier, point)
			return false
		}
		// No normalization required, since we haven't cached anything, and the start TS is non-zer0.
		return true
	}

	previous, hasPrevious := s.previousCache.GetSummaryDataPoint(identifier)
	if !hasPrevious {
		// This should never happen, but fall-back to the start point if we
		// don't find a previous point
		previous = start
	}
	if !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) || (point.StartTimestamp() == 0 && lessThanSummaryDataPoint(point, previous)) {
		// This is a reset point, but we have seen this timeseries before, so we know the reset happened in the time period since the last point.
		// Assume the reset occurred at T - 1 ms, and leave the value untouched.
		point.SetStartTimestamp(pcommon.Timestamp(uint64(point.Timestamp()) - uint64(time.Millisecond)))
		s.previousCache.SetSummaryDataPoint(identifier, point)
		// For subsequent points, we don't want to modify the value, but we do
		// want to make the start timestamp match the point we write here.
		// Store a point with the same timestamps, but zero value to achieve
		// that behavior.
		zeroPoint := pmetric.NewSummaryDataPoint()
		zeroPoint.SetTimestamp(point.StartTimestamp())
		s.startCache.SetSummaryDataPoint(identifier, zeroPoint)
		return true
	}
	if !start.Timestamp().AsTime().Before(point.Timestamp().AsTime()) {
		// We found a cached start timestamp that wouldn't produce a valid point.
		// Drop it and log.
		s.log.Info(
			"data point being processed older than last recorded reset, will not be emitted",
			zap.String("lastRecordedReset", start.Timestamp().String()),
			zap.String("dataPoint", point.Timestamp().String()),
		)
		return false
	}
	// There was no reset, so normalize the point against the start point
	subtractSummaryDataPoint(point, start)
	s.previousCache.SetSummaryDataPoint(identifier, point)
	return true
}

// lessThanSummaryDataPoint returns a < b.
func lessThanSummaryDataPoint(a, b pmetric.SummaryDataPoint) bool {
	return a.Count() < b.Count() || a.Sum() < b.Sum()
}

// subtractSummaryDataPoint subtracts b from a.
func subtractSummaryDataPoint(a, b pmetric.SummaryDataPoint) {
	// Quantile values are not modified. Quantiles are computed over the same
	// time period as sum and count, but it isn't possible to normalize them.
	// Use the timestamp from the normalization point
	a.SetStartTimestamp(b.Timestamp())
	// Adjust the value based on the start point's value
	a.SetCount(a.Count() - b.Count())
	// We drop points without a sum, so no need to check here.
	a.SetSum(a.Sum() - b.Sum())
}
