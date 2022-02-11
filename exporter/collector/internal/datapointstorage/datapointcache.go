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

type Cache map[string]usedPoint

type usedPoint struct {
	point *pdata.NumberDataPoint
	used  bool
}

// New instantiates a cache and starts background processes
func NewCache(shutdown <-chan struct{}) Cache {
	c := make(Cache)
	go func() {
		ticker := time.NewTicker(gcInterval)
		for c.gc(shutdown, ticker.C) {
		}
	}()
	return c
}

// Get retrieves the point associated with the identifier, and whether
// or not it was found
func (c Cache) Get(identifier string) (*pdata.NumberDataPoint, bool) {
	point, found := c[identifier]
	if found {
		point.used = true
		c[identifier] = point
	}
	return point.point, found
}

// Set assigns the point to the identifier in the cache
func (c Cache) Set(identifier string, point *pdata.NumberDataPoint) {
	c[identifier] = usedPoint{point, true}
}

// gc garbage collects the cache after the ticker ticks
func (c Cache) gc(shutdown <-chan struct{}, tickerCh <-chan time.Time) bool {
	select {
	case <-shutdown:
		return false
	case <-tickerCh:
		// garbage collect the cache
		for id, point := range c {
			if point.used {
				// for points that have been used, mark them as unused
				point.used = false
				c[id] = point
			} else {
				// for points that have not been used, delete points
				delete(c, id)
			}
		}
	}
	return true
}

// Identifier returns the unique string identifier for a metric
func Identifier(resource *monitoredrespb.MonitoredResource, metric pdata.Metric, labels pdata.AttributeMap) string {
	var b strings.Builder

	// Resource identifiers
	fmt.Fprintf(&b, "%v", resource.GetLabels())

	// Metric identifiers
	fmt.Fprintf(&b, " - %s", metric.Name())
	labels.Sort().Range(func(k string, v pdata.AttributeValue) bool {
		fmt.Fprintf(&b, " %s=%s", k, v.AsString())
		return true
	})
	return b.String()
}
