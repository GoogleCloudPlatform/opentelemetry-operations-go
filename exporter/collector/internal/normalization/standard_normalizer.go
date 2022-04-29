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
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/datapointstorage"
)

// NewStandardNormalizer performs normalization on cumulative points which:
//  (a) don't have a start time, OR
//  (b) have been sent a preceding "reset" point as described in https://github.com/open-telemetry/opentelemetry-specification/blob/9555f9594c7ffe5dc333b53da5e0f880026cead1/specification/metrics/datamodel.md#resets-and-gaps
// The first point without a start time or the reset point is cached, and is
// NOT exported. Subsequent points "subtract" the initial point prior to exporting.
func NewStandardNormalizer(shutdown <-chan struct{}, logger *zap.Logger) Normalizer {
	return &standardNormalizer{
		cache: datapointstorage.NewCache(shutdown),
		log:   logger,
	}
}

type standardNormalizer struct {
	cache datapointstorage.Cache
	log   *zap.Logger
}

func (s *standardNormalizer) NormalizeExponentialHistogramDataPoint(point pdata.ExponentialHistogramDataPoint, identifier string) *pdata.ExponentialHistogramDataPoint {
	// if the point doesn't need to be normalized, use original point
	normalizedPoint := &point
	start, ok := s.cache.GetExponentialHistogramDataPoint(identifier)
	if ok {
		if !start.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
			// We found a cached start timestamp that wouldn't produce a valid point.
			// Drop it and log.
			s.log.Info(
				"data point being processed older than last recorded reset, will not be emitted",
				zap.String("lastRecordedReset", start.Timestamp().String()),
				zap.String("dataPoint", point.Timestamp().String()),
			)
			return nil
		}
		if point.Scale() != start.Scale() {
			// TODO(#366): It is possible, but difficult to compare exponential
			// histograms with different scales. For now, treat a change in
			// scale as a reset.
			s.cache.SetExponentialHistogramDataPoint(identifier, &point)
			return nil
		}
		// Make a copy so we don't mutate underlying data
		newPoint := pdata.NewExponentialHistogramDataPoint()
		point.CopyTo(newPoint)
		// Use the start timestamp from the normalization point
		newPoint.SetStartTimestamp(start.Timestamp())
		// Adjust the value based on the start point's value
		newPoint.SetCount(point.Count() - start.Count())
		// We drop points without a sum, so no need to check here.
		newPoint.SetSum(point.Sum() - start.Sum())
		newPoint.SetZeroCount(point.ZeroCount() - start.ZeroCount())
		normalizeExponentialBuckets(newPoint.Positive(), start.Positive())
		normalizeExponentialBuckets(newPoint.Negative(), start.Negative())
		normalizedPoint = &newPoint
	}
	if (!ok && point.StartTimestamp() == 0) || !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
		// This is the first time we've seen this metric, or we received
		// an explicit reset point as described in
		//
		// Record it in history and drop the point.
		s.cache.SetExponentialHistogramDataPoint(identifier, &point)
		return nil
	}
	return normalizedPoint
}

func normalizeExponentialBuckets(pointBuckets, startBuckets pdata.Buckets) {
	newBuckets := make([]uint64, len(pointBuckets.BucketCounts()))
	offsetDiff := int(pointBuckets.Offset() - startBuckets.Offset())
	for i := range pointBuckets.BucketCounts() {
		startOffset := i + offsetDiff
		// if there is no corresponding bucket for the starting bucketcounts, don't normalize
		if startOffset < 0 || startOffset >= len(startBuckets.BucketCounts()) {
			newBuckets[i] = pointBuckets.BucketCounts()[i]
		} else {
			newBuckets[i] = pointBuckets.BucketCounts()[i] - startBuckets.BucketCounts()[startOffset]
		}
	}
	pointBuckets.SetOffset(pointBuckets.Offset())
	pointBuckets.SetBucketCounts(newBuckets)
}

func (s *standardNormalizer) NormalizeHistogramDataPoint(point pdata.HistogramDataPoint, identifier string) *pdata.HistogramDataPoint {
	// if the point doesn't need to be normalized, use original point
	normalizedPoint := &point
	start, ok := s.cache.GetHistogramDataPoint(identifier)
	if ok {
		if !start.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
			// We found a cached start timestamp that wouldn't produce a valid point.
			// Drop it and log.
			s.log.Info(
				"data point being processed older than last recorded reset, will not be emitted",
				zap.String("lastRecordedReset", start.Timestamp().String()),
				zap.String("dataPoint", point.Timestamp().String()),
			)
			return nil
		}
		// Make a copy so we don't mutate underlying data
		newPoint := pdata.NewHistogramDataPoint()
		point.CopyTo(newPoint)
		// Use the start timestamp from the normalization point
		newPoint.SetStartTimestamp(start.Timestamp())
		// Adjust the value based on the start point's value
		newPoint.SetCount(point.Count() - start.Count())
		// We drop points without a sum, so no need to check here.
		newPoint.SetSum(point.Sum() - start.Sum())
		pointBuckets := point.BucketCounts()
		startBuckets := start.BucketCounts()
		if !bucketBoundariesEqual(point.ExplicitBounds(), start.ExplicitBounds()) {
			// The number of buckets changed, so we can't normalize points anymore.
			// Treat this as a reset by recording and dropping this point.
			s.cache.SetHistogramDataPoint(identifier, &point)
			return nil
		}
		newBuckets := make([]uint64, len(pointBuckets))
		for i := range pointBuckets {
			newBuckets[i] = pointBuckets[i] - startBuckets[i]
		}
		newPoint.SetBucketCounts(newBuckets)
		normalizedPoint = &newPoint
	}
	if (!ok && point.StartTimestamp() == 0) || !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
		// This is the first time we've seen this metric, or we received
		// an explicit reset point as described in
		// https://github.com/open-telemetry/opentelemetry-specification/blob/9555f9594c7ffe5dc333b53da5e0f880026cead1/specification/metrics/datamodel.md#resets-and-gaps
		// Record it in history and drop the point.
		s.cache.SetHistogramDataPoint(identifier, &point)
		return nil
	}
	return normalizedPoint
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
func (s *standardNormalizer) NormalizeNumberDataPoint(point pdata.NumberDataPoint, identifier string) *pdata.NumberDataPoint {
	// if the point doesn't need to be normalized, use original point
	normalizedPoint := &point
	start, ok := s.cache.GetNumberDataPoint(identifier)
	if ok {
		if !start.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
			// We found a cached start timestamp that wouldn't produce a valid point.
			// Drop it and log.
			s.log.Info(
				"data point being processed older than last recorded reset, will not be emitted",
				zap.String("lastRecordedReset", start.Timestamp().String()),
				zap.String("dataPoint", point.Timestamp().String()),
			)
			return nil
		}
		// Make a copy so we don't mutate underlying data
		newPoint := pdata.NewNumberDataPoint()
		point.CopyTo(newPoint)
		// Use the start timestamp from the normalization point
		newPoint.SetStartTimestamp(start.Timestamp())
		// Adjust the value based on the start point's value
		switch newPoint.ValueType() {
		case pmetric.MetricValueTypeInt:
			newPoint.SetIntVal(point.IntVal() - start.IntVal())
		case pmetric.MetricValueTypeDouble:
			newPoint.SetDoubleVal(point.DoubleVal() - start.DoubleVal())
		}
		normalizedPoint = &newPoint
	}
	if (!ok && point.StartTimestamp() == 0) || !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
		// This is the first time we've seen this metric, or we received
		// an explicit reset point as described in
		// https://github.com/open-telemetry/opentelemetry-specification/blob/9555f9594c7ffe5dc333b53da5e0f880026cead1/specification/metrics/datamodel.md#resets-and-gaps
		// Record it in history and drop the point.
		s.cache.SetNumberDataPoint(identifier, &point)
		return nil
	}
	return normalizedPoint
}

func (s *standardNormalizer) NormalizeSummaryDataPoint(point pdata.SummaryDataPoint, identifier string) *pdata.SummaryDataPoint {
	// if the point doesn't need to be normalized, use original point
	normalizedPoint := &point
	start, ok := s.cache.GetSummaryDataPoint(identifier)
	if ok {
		if !start.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
			// We found a cached start timestamp that wouldn't produce a valid point.
			// Drop it and log.
			s.log.Info(
				"data point being processed older than last recorded reset, will not be emitted",
				zap.String("lastRecordedReset", start.Timestamp().String()),
				zap.String("dataPoint", point.Timestamp().String()),
			)
			return nil
		}
		// Make a copy so we don't mutate underlying data.
		newPoint := pdata.NewSummaryDataPoint()
		// Quantile values are copied, and are not modified. Quantiles are
		// computed over the same time period as sum and count, but it isn't
		// possible to normalize them.
		point.CopyTo(newPoint)
		// Use the start timestamp from the normalization point
		newPoint.SetStartTimestamp(start.Timestamp())
		// Adjust the value based on the start point's value
		newPoint.SetCount(point.Count() - start.Count())
		// We drop points without a sum, so no need to check here.
		newPoint.SetSum(point.Sum() - start.Sum())
		normalizedPoint = &newPoint
	}
	if (!ok && point.StartTimestamp() == 0) || !point.StartTimestamp().AsTime().Before(point.Timestamp().AsTime()) {
		// This is the first time we've seen this metric, or we received
		// an explicit reset point as described in
		// https://github.com/open-telemetry/opentelemetry-specification/blob/9555f9594c7ffe5dc333b53da5e0f880026cead1/specification/metrics/datamodel.md#resets-and-gaps
		// Record it in history and drop the point.
		s.cache.SetSummaryDataPoint(identifier, &point)
		return nil
	}
	return normalizedPoint
}
