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
	"time"

	"go.opentelemetry.io/collector/model/pdata"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

const gcInterval = 20 * time.Minute

type Cache struct {
	numberCache               map[string]usedNumberPoint
	summaryCache              map[string]usedSummaryPoint
	histogramCache            map[string]usedHistogramPoint
	exponentialHistogramCache map[string]usedExponentialHistogramPoint
}

type usedNumberPoint struct {
	point *pdata.NumberDataPoint
	used  bool
}

type usedSummaryPoint struct {
	point *pdata.SummaryDataPoint
	used  bool
}

type usedHistogramPoint struct {
	point *pdata.HistogramDataPoint
	used  bool
}

type usedExponentialHistogramPoint struct {
	point *pdata.ExponentialHistogramDataPoint
	used  bool
}

// NewCache instantiates a cache and starts background processes
func NewCache(shutdown <-chan struct{}) Cache {
	c := Cache{
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
func (c Cache) GetNumberDataPoint(identifier string) (*pdata.NumberDataPoint, bool) {
	point, found := c.numberCache[identifier]
	if found {
		point.used = true
		c.numberCache[identifier] = point
	}
	return point.point, found
}

// SetNumberDataPoint assigns the point to the identifier in the cache
func (c Cache) SetNumberDataPoint(identifier string, point *pdata.NumberDataPoint) {
	c.numberCache[identifier] = usedNumberPoint{point, true}
}

// GetSummaryDataPoint retrieves the point associated with the identifier, and whether
// or not it was found
func (c Cache) GetSummaryDataPoint(identifier string) (*pdata.SummaryDataPoint, bool) {
	point, found := c.summaryCache[identifier]
	if found {
		point.used = true
		c.summaryCache[identifier] = point
	}
	return point.point, found
}

// SetSummaryDataPoint assigns the point to the identifier in the cache
func (c Cache) SetSummaryDataPoint(identifier string, point *pdata.SummaryDataPoint) {
	c.summaryCache[identifier] = usedSummaryPoint{point, true}
}

// GetHistogramDataPoint retrieves the point associated with the identifier, and whether
// or not it was found
func (c Cache) GetHistogramDataPoint(identifier string) (*pdata.HistogramDataPoint, bool) {
	point, found := c.histogramCache[identifier]
	if found {
		point.used = true
		c.histogramCache[identifier] = point
	}
	return point.point, found
}

// SetHistogramDataPoint assigns the point to the identifier in the cache
func (c Cache) SetHistogramDataPoint(identifier string, point *pdata.HistogramDataPoint) {
	c.histogramCache[identifier] = usedHistogramPoint{point, true}
}

// GetExponentialHistogramDataPoint retrieves the point associated with the identifier, and whether
// or not it was found
func (c Cache) GetExponentialHistogramDataPoint(identifier string) (*pdata.ExponentialHistogramDataPoint, bool) {
	point, found := c.exponentialHistogramCache[identifier]
	if found {
		point.used = true
		c.exponentialHistogramCache[identifier] = point
	}
	return point.point, found
}

// SetExponentialHistogramDataPoint assigns the point to the identifier in the cache
func (c Cache) SetExponentialHistogramDataPoint(identifier string, point *pdata.ExponentialHistogramDataPoint) {
	c.exponentialHistogramCache[identifier] = usedExponentialHistogramPoint{point, true}
}

// gc garbage collects the cache after the ticker ticks
func (c Cache) gc(shutdown <-chan struct{}, tickerCh <-chan time.Time) bool {
	select {
	case <-shutdown:
		return false
	case <-tickerCh:
		// garbage collect the numberCache
		for id, point := range c.numberCache {
			if point.used {
				// for points that have been used, mark them as unused
				point.used = false
				c.numberCache[id] = point
			} else {
				// for points that have not been used, delete points
				delete(c.numberCache, id)
			}
		}
		// garbage collect the summaryCache
		for id, point := range c.summaryCache {
			if point.used {
				// for points that have been used, mark them as unused
				point.used = false
				c.summaryCache[id] = point
			} else {
				// for points that have not been used, delete points
				delete(c.summaryCache, id)
			}
		}
		// garbage collect the histogramCache
		for id, point := range c.histogramCache {
			if point.used {
				// for points that have been used, mark them as unused
				point.used = false
				c.histogramCache[id] = point
			} else {
				// for points that have not been used, delete points
				delete(c.histogramCache, id)
			}
		}
		// garbage collect the exponentialHistogramCache
		for id, point := range c.exponentialHistogramCache {
			if point.used {
				// for points that have been used, mark them as unused
				point.used = false
				c.exponentialHistogramCache[id] = point
			} else {
				// for points that have not been used, delete points
				delete(c.exponentialHistogramCache, id)
			}
		}
	}
	return true
}

// Identifier returns the unique string identifier for a metric
func Identifier(resource *monitoredrespb.MonitoredResource, extraLabels map[string]string, metric pdata.Metric, attributes pdata.Map) string {
	var b strings.Builder

	// Resource identifiers
	if resource != nil {
		fmt.Fprintf(&b, "%v", resource.GetLabels())
	}

	// Instrumentation library labels and additional resource labels
	fmt.Fprintf(&b, " - %v", extraLabels)

	// Metric identifiers
	fmt.Fprintf(&b, " - %s -", metric.Name())
	attributes.Sort().Range(func(k string, v pdata.Value) bool {
		fmt.Fprintf(&b, " %s=%s", k, v.AsString())
		return true
	})
	return b.String()
}
