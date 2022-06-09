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
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/atomic"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

const gcInterval = 20 * time.Minute

type Cache struct {
	numberCache               map[string]usedNumberPoint
	summaryCache              map[string]usedSummaryPoint
	histogramCache            map[string]usedHistogramPoint
	exponentialHistogramCache map[string]usedExponentialHistogramPoint
	numberLock                sync.RWMutex
	summaryLock               sync.RWMutex
	histogramLock             sync.RWMutex
	exponentialHistogramLock  sync.RWMutex
}

type usedNumberPoint struct {
	point *pmetric.NumberDataPoint
	used  *atomic.Bool
}

type usedSummaryPoint struct {
	point *pmetric.SummaryDataPoint
	used  *atomic.Bool
}

type usedHistogramPoint struct {
	point *pmetric.HistogramDataPoint
	used  *atomic.Bool
}

type usedExponentialHistogramPoint struct {
	point *pmetric.ExponentialHistogramDataPoint
	used  *atomic.Bool
}

// NewCache instantiates a cache and starts background processes
func NewCache(shutdown <-chan struct{}) *Cache {
	c := &Cache{
		numberCache:               make(map[string]usedNumberPoint),
		summaryCache:              make(map[string]usedSummaryPoint),
		histogramCache:            make(map[string]usedHistogramPoint),
		exponentialHistogramCache: make(map[string]usedExponentialHistogramPoint),
	}
	go func() {
		ticker := time.NewTicker(gcInterval)
		for c.gc(shutdown, ticker.C) {
		}
	}()
	return c
}

// GetNumberDataPoint retrieves the point associated with the identifier, and whether
// or not it was found
func (c *Cache) GetNumberDataPoint(identifier string) (*pmetric.NumberDataPoint, bool) {
	c.numberLock.RLock()
	defer c.numberLock.RUnlock()
	point, found := c.numberCache[identifier]
	if found {
		point.used.Store(true)
		c.numberCache[identifier] = point
	}
	return point.point, found
}

// SetNumberDataPoint assigns the point to the identifier in the cache
func (c *Cache) SetNumberDataPoint(identifier string, point *pmetric.NumberDataPoint) {
	c.numberLock.Lock()
	defer c.numberLock.Unlock()
	c.numberCache[identifier] = usedNumberPoint{point, atomic.NewBool(true)}
}

// GetSummaryDataPoint retrieves the point associated with the identifier, and whether
// or not it was found
func (c *Cache) GetSummaryDataPoint(identifier string) (*pmetric.SummaryDataPoint, bool) {
	c.summaryLock.RLock()
	defer c.summaryLock.RUnlock()
	point, found := c.summaryCache[identifier]
	if found {
		point.used.Store(true)
		c.summaryCache[identifier] = point
	}
	return point.point, found
}

// SetSummaryDataPoint assigns the point to the identifier in the cache
func (c *Cache) SetSummaryDataPoint(identifier string, point *pmetric.SummaryDataPoint) {
	c.summaryLock.Lock()
	defer c.summaryLock.Unlock()
	c.summaryCache[identifier] = usedSummaryPoint{point, atomic.NewBool(true)}
}

// GetHistogramDataPoint retrieves the point associated with the identifier, and whether
// or not it was found
func (c *Cache) GetHistogramDataPoint(identifier string) (*pmetric.HistogramDataPoint, bool) {
	c.histogramLock.RLock()
	defer c.histogramLock.RUnlock()
	point, found := c.histogramCache[identifier]
	if found {
		point.used.Store(true)
		c.histogramCache[identifier] = point
	}
	return point.point, found
}

// SetHistogramDataPoint assigns the point to the identifier in the cache
func (c *Cache) SetHistogramDataPoint(identifier string, point *pmetric.HistogramDataPoint) {
	c.histogramLock.Lock()
	defer c.histogramLock.Unlock()
	c.histogramCache[identifier] = usedHistogramPoint{point, atomic.NewBool(true)}
}

// GetExponentialHistogramDataPoint retrieves the point associated with the identifier, and whether
// or not it was found
func (c *Cache) GetExponentialHistogramDataPoint(identifier string) (*pmetric.ExponentialHistogramDataPoint, bool) {
	c.exponentialHistogramLock.RLock()
	defer c.exponentialHistogramLock.RUnlock()
	point, found := c.exponentialHistogramCache[identifier]
	if found {
		point.used.Store(true)
		c.exponentialHistogramCache[identifier] = point
	}
	return point.point, found
}

// SetExponentialHistogramDataPoint assigns the point to the identifier in the cache
func (c *Cache) SetExponentialHistogramDataPoint(identifier string, point *pmetric.ExponentialHistogramDataPoint) {
	c.exponentialHistogramLock.Lock()
	defer c.exponentialHistogramLock.Unlock()
	c.exponentialHistogramCache[identifier] = usedExponentialHistogramPoint{point, atomic.NewBool(true)}
}

// gc garbage collects the cache after the ticker ticks
func (c *Cache) gc(shutdown <-chan struct{}, tickerCh <-chan time.Time) bool {
	select {
	case <-shutdown:
		return false
	case <-tickerCh:
		// garbage collect the numberCache
		c.numberLock.Lock()
		for id, point := range c.numberCache {
			// If used is true, swap it to false. Otherwise, delete the point.
			if !point.used.CAS(true, false) {
				// for points that have not been used, delete points
				delete(c.numberCache, id)
			}
		}
		c.numberLock.Unlock()
		// garbage collect the summaryCache
		c.summaryLock.Lock()
		for id, point := range c.summaryCache {
			// If used is true, swap it to false. Otherwise, delete the point.
			if !point.used.CAS(true, false) {
				// for points that have not been used, delete points
				delete(c.summaryCache, id)
			}
		}
		c.summaryLock.Unlock()
		// garbage collect the histogramCache
		c.histogramLock.Lock()
		for id, point := range c.histogramCache {
			// If used is true, swap it to false. Otherwise, delete the point.
			if !point.used.CAS(true, false) {
				// for points that have not been used, delete points
				delete(c.histogramCache, id)
			}
		}
		c.histogramLock.Unlock()
		// garbage collect the exponentialHistogramCache
		c.exponentialHistogramLock.Lock()
		for id, point := range c.exponentialHistogramCache {
			// If used is true, swap it to false. Otherwise, delete the point.
			if !point.used.CAS(true, false) {
				// for points that have not been used, delete points
				delete(c.exponentialHistogramCache, id)
			}
		}
		c.exponentialHistogramLock.Unlock()
	}
	return true
}

// Identifier returns the unique string identifier for a metric
func Identifier(resource *monitoredrespb.MonitoredResource, extraLabels map[string]string, metric pmetric.Metric, attributes pcommon.Map) string {
	var b strings.Builder

	// Resource identifiers
	if resource != nil {
		fmt.Fprintf(&b, "%v", resource.GetLabels())
	}

	// Instrumentation library labels and additional resource labels
	fmt.Fprintf(&b, " - %v", extraLabels)

	// Metric identifiers
	fmt.Fprintf(&b, " - %s -", metric.Name())
	attributes.Sort().Range(func(k string, v pcommon.Value) bool {
		fmt.Fprintf(&b, " %s=%s", k, v.AsString())
		return true
	})
	return b.String()
}
