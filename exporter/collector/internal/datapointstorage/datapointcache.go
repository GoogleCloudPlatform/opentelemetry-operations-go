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

package datapointstorage

import (
	"hash"
	"hash/fnv"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/atomic"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

const gcInterval = 20 * time.Minute

type Cache struct {
	numberCache               map[uint64]usedNumberPoint
	summaryCache              map[uint64]usedSummaryPoint
	histogramCache            map[uint64]usedHistogramPoint
	exponentialHistogramCache map[uint64]usedExponentialHistogramPoint
	numberLock                sync.RWMutex
	summaryLock               sync.RWMutex
	histogramLock             sync.RWMutex
	exponentialHistogramLock  sync.RWMutex
}

type usedNumberPoint struct {
	point pmetric.NumberDataPoint
	used  *atomic.Bool
}

type usedSummaryPoint struct {
	point pmetric.SummaryDataPoint
	used  *atomic.Bool
}

type usedHistogramPoint struct {
	point pmetric.HistogramDataPoint
	used  *atomic.Bool
}

type usedExponentialHistogramPoint struct {
	point pmetric.ExponentialHistogramDataPoint
	used  *atomic.Bool
}

// NewCache instantiates a cache and starts background processes.
func NewCache(shutdown <-chan struct{}) *Cache {
	c := &Cache{
		numberCache:               make(map[uint64]usedNumberPoint),
		summaryCache:              make(map[uint64]usedSummaryPoint),
		histogramCache:            make(map[uint64]usedHistogramPoint),
		exponentialHistogramCache: make(map[uint64]usedExponentialHistogramPoint),
	}
	go func() {
		ticker := time.NewTicker(gcInterval)
		//nolint:revive
		for c.gc(shutdown, ticker.C) {
		}
	}()
	return c
}

// GetNumberDataPoint retrieves the point associated with the identifier, and whether
// or not it was found.
func (c *Cache) GetNumberDataPoint(identifier uint64) (pmetric.NumberDataPoint, bool) {
	c.numberLock.RLock()
	defer c.numberLock.RUnlock()
	point, found := c.numberCache[identifier]
	if found {
		point.used.Store(true)
	}
	return point.point, found
}

// SetNumberDataPoint assigns the point to the identifier in the cache.
func (c *Cache) SetNumberDataPoint(identifier uint64, point pmetric.NumberDataPoint) {
	c.numberLock.Lock()
	defer c.numberLock.Unlock()
	if existing, ok := c.numberCache[identifier]; ok {
		existing.used.Store(true)
		minimalNumberDataPointCopyTo(point, existing.point)
		return
	}
	newPoint := pmetric.NewNumberDataPoint()
	minimalNumberDataPointCopyTo(point, newPoint)
	c.numberCache[identifier] = usedNumberPoint{newPoint, atomic.NewBool(true)}
}

// GetSummaryDataPoint retrieves the point associated with the identifier, and whether
// or not it was found.
func (c *Cache) GetSummaryDataPoint(identifier uint64) (pmetric.SummaryDataPoint, bool) {
	c.summaryLock.RLock()
	defer c.summaryLock.RUnlock()
	point, found := c.summaryCache[identifier]
	if found {
		point.used.Store(true)
	}
	return point.point, found
}

// SetSummaryDataPoint assigns the point to the identifier in the cache.
func (c *Cache) SetSummaryDataPoint(identifier uint64, point pmetric.SummaryDataPoint) {
	c.summaryLock.Lock()
	defer c.summaryLock.Unlock()
	if existing, ok := c.summaryCache[identifier]; ok {
		existing.used.Store(true)
		minimalSummaryDataPointCopyTo(point, existing.point)
		return
	}
	newPoint := pmetric.NewSummaryDataPoint()
	minimalSummaryDataPointCopyTo(point, newPoint)
	c.summaryCache[identifier] = usedSummaryPoint{newPoint, atomic.NewBool(true)}
}

// GetHistogramDataPoint retrieves the point associated with the identifier, and whether
// or not it was found.
func (c *Cache) GetHistogramDataPoint(identifier uint64) (pmetric.HistogramDataPoint, bool) {
	c.histogramLock.RLock()
	defer c.histogramLock.RUnlock()
	point, found := c.histogramCache[identifier]
	if found {
		point.used.Store(true)
	}
	return point.point, found
}

// SetHistogramDataPoint assigns the point to the identifier in the cache.
func (c *Cache) SetHistogramDataPoint(identifier uint64, point pmetric.HistogramDataPoint) {
	c.histogramLock.Lock()
	defer c.histogramLock.Unlock()
	if existing, ok := c.histogramCache[identifier]; ok {
		existing.used.Store(true)
		minimalHistogramDataPointCopyTo(point, existing.point)
		return
	}
	newPoint := pmetric.NewHistogramDataPoint()
	minimalHistogramDataPointCopyTo(point, newPoint)
	c.histogramCache[identifier] = usedHistogramPoint{newPoint, atomic.NewBool(true)}
}

// GetExponentialHistogramDataPoint retrieves the point associated with the identifier, and whether
// or not it was found.
func (c *Cache) GetExponentialHistogramDataPoint(identifier uint64) (pmetric.ExponentialHistogramDataPoint, bool) {
	c.exponentialHistogramLock.RLock()
	defer c.exponentialHistogramLock.RUnlock()
	point, found := c.exponentialHistogramCache[identifier]
	if found {
		point.used.Store(true)
	}
	return point.point, found
}

// SetExponentialHistogramDataPoint assigns the point to the identifier in the cache.
func (c *Cache) SetExponentialHistogramDataPoint(identifier uint64, point pmetric.ExponentialHistogramDataPoint) {
	c.exponentialHistogramLock.Lock()
	defer c.exponentialHistogramLock.Unlock()
	if existing, ok := c.exponentialHistogramCache[identifier]; ok {
		existing.used.Store(true)
		minimalExponentialHistogramDataPointCopyTo(point, existing.point)
		return
	}
	newPoint := pmetric.NewExponentialHistogramDataPoint()
	minimalExponentialHistogramDataPointCopyTo(point, newPoint)
	c.exponentialHistogramCache[identifier] = usedExponentialHistogramPoint{newPoint, atomic.NewBool(true)}
}

// gc garbage collects the cache after the ticker ticks.
func (c *Cache) gc(shutdown <-chan struct{}, tickerCh <-chan time.Time) bool {
	select {
	case <-shutdown:
		return false
	case <-tickerCh:
		// garbage collect the numberCache
		c.numberLock.Lock()
		for id, point := range c.numberCache {
			// for points that have been used, mark them as unused
			if point.used.Load() {
				point.used.Store(false)
			} else {
				// for points that have not been used, delete points
				delete(c.numberCache, id)
			}
		}
		c.numberLock.Unlock()
		// garbage collect the summaryCache
		c.summaryLock.Lock()
		for id, point := range c.summaryCache {
			// for points that have been used, mark them as unused
			if point.used.Load() {
				point.used.Store(false)
			} else {
				// for points that have not been used, delete points
				delete(c.summaryCache, id)
			}
		}
		c.summaryLock.Unlock()
		// garbage collect the histogramCache
		c.histogramLock.Lock()
		for id, point := range c.histogramCache {
			// for points that have been used, mark them as unused
			if point.used.Load() {
				point.used.Store(false)
			} else {
				// for points that have not been used, delete points
				delete(c.histogramCache, id)
			}
		}
		c.histogramLock.Unlock()
		// garbage collect the exponentialHistogramCache
		c.exponentialHistogramLock.Lock()
		for id, point := range c.exponentialHistogramCache {
			// for points that have been used, mark them as unused
			if point.used.Load() {
				point.used.Store(false)
			} else {
				// for points that have not been used, delete points
				delete(c.exponentialHistogramCache, id)
			}
		}
		c.exponentialHistogramLock.Unlock()
	}
	return true
}

// Use the same constants as Prometheus uses for hashing:
// https://github.com/prometheus/prometheus/blob/282fb1632ad62a82401a230f486538a72384faf0/model/labels/labels_common.go#L32
var (
	itemSep = []byte{'\xfe'} // Used between identifiers
	kvSep   = []byte{'\xff'} // Used between map keys and values
)

// Identifier returns the unique string identifier for a metric.
func Identifier(resource *monitoredrespb.MonitoredResource, extraLabels map[string]string, metric pmetric.Metric, attributes pcommon.Map) uint64 {
	h := fnv.New64()
	h.Write([]byte(resource.GetType()))
	h.Write(itemSep)
	h.Write([]byte(metric.Name()))
	h.Write(itemSep)
	attrs := make(map[string]string)
	attributes.Range(func(k string, v pcommon.Value) bool {
		attrs[k] = v.AsString()
		return true
	})
	hashOfMap(h, extraLabels)
	h.Write(itemSep)
	hashOfMap(h, attrs)
	h.Write(itemSep)
	hashOfMap(h, resource.GetLabels())
	return h.Sum64()
}

func hashOfMap(h hash.Hash64, m map[string]string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		h.Write([]byte(key))
		h.Write(kvSep)
		h.Write([]byte(m[key]))
		h.Write(kvSep)
	}
}

// minimalSummaryDataPointCopyTo is the same as CopyTo for SummaryDataPoint,
// but only copies values required for normalization.
func minimalSummaryDataPointCopyTo(src, dest pmetric.SummaryDataPoint) {
	// We do not copy attributes, start timestamp, flags, quantiles
	dest.SetTimestamp(src.Timestamp())
	dest.SetCount(src.Count())
	dest.SetSum(src.Sum())
}

// minimalHistogramDataPointCopyTo is the same as CopyTo for
// HistogramDataPoint, but only copies values required for normalization.
func minimalHistogramDataPointCopyTo(src, dest pmetric.HistogramDataPoint) {
	// We do not copy attributes, start timestamp, flags, exemplars, min, max
	dest.SetTimestamp(src.Timestamp())
	dest.SetCount(src.Count())
	src.BucketCounts().CopyTo(dest.BucketCounts())
	src.ExplicitBounds().CopyTo(dest.ExplicitBounds())
	if src.HasSum() {
		dest.SetSum(src.Sum())
	}
}

// minimalExponentialHistogramDataPointCopyTo is the same as CopyTo for
// ExponentialHistogramDataPoint, but only copies values required for normalization.
func minimalExponentialHistogramDataPointCopyTo(src, dest pmetric.ExponentialHistogramDataPoint) {
	// We do not copy attributes, start timestamp, flags, exemplars, min, max, zero threshold
	dest.SetTimestamp(src.Timestamp())
	dest.SetCount(src.Count())
	dest.SetZeroCount(src.ZeroCount())
	dest.SetScale(src.Scale())
	src.Positive().CopyTo(dest.Positive())
	src.Negative().CopyTo(dest.Negative())
	if src.HasSum() {
		dest.SetSum(src.Sum())
	}
	dest.SetZeroThreshold(src.ZeroThreshold())
}

// minimalNumberDataPointCopyTo is the same as CopyTo for NumberDataPoint,
// but only copies values required for normalization.
func minimalNumberDataPointCopyTo(src, dest pmetric.NumberDataPoint) {
	// We do not copy attributes, start timestamp, flags, exemplars
	dest.SetTimestamp(src.Timestamp())
	switch src.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		dest.SetDoubleValue(src.DoubleValue())
	case pmetric.NumberDataPointValueTypeInt:
		dest.SetIntValue(src.IntValue())
	}
}
