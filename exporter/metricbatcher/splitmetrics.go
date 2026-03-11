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

package metricbatcher

import (
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// splitResourceMetrics splits a metricdata.ResourceMetrics into multiple ResourceMetrics, sequentially,
// ensuring no ResourceMetrics has more than `size` data points. It does not mutate the `src` object.
func splitResourceMetrics(size int, src *metricdata.ResourceMetrics) []*metricdata.ResourceMetrics {
	if size <= 0 {
		return []*metricdata.ResourceMetrics{src}
	}
	var batches []*metricdata.ResourceMetrics
	var currentBatch *metricdata.ResourceMetrics
	currentPoints := 0
	for i := 0; i < len(src.ScopeMetrics); i++ {
		sm := src.ScopeMetrics[i]
		smPoints := scopeMetricsDPC(sm)
		// Fast path: the entire ScopeMetrics fits in the current batch.
		if currentPoints+smPoints <= size {
			if currentBatch == nil {
				currentBatch = &metricdata.ResourceMetrics{Resource: src.Resource}
				batches = append(batches, currentBatch)
				currentPoints = 0
			}
			currentBatch.ScopeMetrics = append(currentBatch.ScopeMetrics, sm)
			currentPoints += smPoints
			if currentPoints == size {
				currentBatch = nil
			}
			continue
		}
		// Slow path: the ScopeMetrics overflows the batch size. Inspect individual Metrics.
		var currentSm *metricdata.ScopeMetrics
		for j := 0; j < len(sm.Metrics); j++ {
			m := sm.Metrics[j]
			mPoints := metricDPC(m)
			// If the entire Metric fits, append it.
			if currentPoints+mPoints <= size {
				if currentBatch == nil {
					currentBatch = &metricdata.ResourceMetrics{Resource: src.Resource}
					batches = append(batches, currentBatch)
					currentPoints = 0
				}
				if currentSm == nil {
					currentBatch.ScopeMetrics = append(currentBatch.ScopeMetrics, metricdata.ScopeMetrics{Scope: sm.Scope})
					currentSm = &currentBatch.ScopeMetrics[len(currentBatch.ScopeMetrics)-1]
				}
				currentSm.Metrics = append(currentSm.Metrics, m)
				currentPoints += mPoints
				if currentPoints == size {
					currentBatch = nil
					currentSm = nil
				}
				continue
			}
			// The Metric overflows. Split its data points across multiple batches.
			mRemaining := mPoints
			mOffset := 0
			for mRemaining > 0 {
				if currentBatch == nil {
					currentBatch = &metricdata.ResourceMetrics{Resource: src.Resource}
					batches = append(batches, currentBatch)
					currentPoints = 0
				}
				if currentSm == nil {
					currentBatch.ScopeMetrics = append(currentBatch.ScopeMetrics, metricdata.ScopeMetrics{Scope: sm.Scope})
					currentSm = &currentBatch.ScopeMetrics[len(currentBatch.ScopeMetrics)-1]
				}
				take := size - currentPoints
				if take > mRemaining {
					take = mRemaining
				}
				mChunk := copyMetricData(m, mOffset, take)
				currentSm.Metrics = append(currentSm.Metrics, mChunk)
				currentPoints += take
				mRemaining -= take
				mOffset += take
				if currentPoints == size {
					currentBatch = nil
					currentSm = nil
				}
			}
		}
	}
	return batches
}

func copyMetricData(m metricdata.Metrics, offset, take int) metricdata.Metrics {
	dest := metricdata.Metrics{
		Name:        m.Name,
		Description: m.Description,
		Unit:        m.Unit,
	}
	switch a := m.Data.(type) {
	case metricdata.Gauge[int64]:
		dest.Data = metricdata.Gauge[int64]{DataPoints: a.DataPoints[offset : offset+take]}
	case metricdata.Gauge[float64]:
		dest.Data = metricdata.Gauge[float64]{DataPoints: a.DataPoints[offset : offset+take]}
	case metricdata.Sum[int64]:
		dest.Data = metricdata.Sum[int64]{DataPoints: a.DataPoints[offset : offset+take], Temporality: a.Temporality, IsMonotonic: a.IsMonotonic}
	case metricdata.Sum[float64]:
		dest.Data = metricdata.Sum[float64]{DataPoints: a.DataPoints[offset : offset+take], Temporality: a.Temporality, IsMonotonic: a.IsMonotonic}
	case metricdata.Histogram[int64]:
		dest.Data = metricdata.Histogram[int64]{DataPoints: a.DataPoints[offset : offset+take], Temporality: a.Temporality}
	case metricdata.Histogram[float64]:
		dest.Data = metricdata.Histogram[float64]{DataPoints: a.DataPoints[offset : offset+take], Temporality: a.Temporality}
	case metricdata.ExponentialHistogram[int64]:
		dest.Data = metricdata.ExponentialHistogram[int64]{DataPoints: a.DataPoints[offset : offset+take], Temporality: a.Temporality}
	case metricdata.ExponentialHistogram[float64]:
		dest.Data = metricdata.ExponentialHistogram[float64]{DataPoints: a.DataPoints[offset : offset+take], Temporality: a.Temporality}
	case metricdata.Summary:
		dest.Data = metricdata.Summary{DataPoints: a.DataPoints[offset : offset+take]}
	}
	return dest
}

// resourceMetricsDPC calculates the total number of data points in the metricdata.ResourceMetrics.
func resourceMetricsDPC(rs *metricdata.ResourceMetrics) int {
	dataPointCount := 0
	ilms := rs.ScopeMetrics
	for k := 0; k < len(ilms); k++ {
		dataPointCount += scopeMetricsDPC(ilms[k])
	}
	return dataPointCount
}

// scopeMetricsDPC calculates the total number of data points in the metricdata.ScopeMetrics.
func scopeMetricsDPC(ilm metricdata.ScopeMetrics) int {
	dataPointCount := 0
	ms := ilm.Metrics
	for k := 0; k < len(ms); k++ {
		dataPointCount += metricDPC(ms[k])
	}
	return dataPointCount
}

// metricDPC calculates the total number of data points in the metricdata.Metrics.
func metricDPC(ms metricdata.Metrics) int {
	switch a := ms.Data.(type) {
	case metricdata.Gauge[int64]:
		return len(a.DataPoints)
	case metricdata.Gauge[float64]:
		return len(a.DataPoints)
	case metricdata.Sum[int64]:
		return len(a.DataPoints)
	case metricdata.Sum[float64]:
		return len(a.DataPoints)
	case metricdata.Histogram[int64]:
		return len(a.DataPoints)
	case metricdata.Histogram[float64]:
		return len(a.DataPoints)
	case metricdata.ExponentialHistogram[int64]:
		return len(a.DataPoints)
	case metricdata.ExponentialHistogram[float64]:
		return len(a.DataPoints)
	case metricdata.Summary:
		return len(a.DataPoints)
	}
	return 0
}
