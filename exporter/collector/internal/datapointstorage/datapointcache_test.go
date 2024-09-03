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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

func TestSetAndGet(t *testing.T) {
	c := Cache{
		numberCache:               make(map[uint64]usedNumberPoint),
		summaryCache:              make(map[uint64]usedSummaryPoint),
		histogramCache:            make(map[uint64]usedHistogramPoint),
		exponentialHistogramCache: make(map[uint64]usedExponentialHistogramPoint),
	}
	id := uint64(12345)
	_, found := c.GetNumberDataPoint(id)
	assert.False(t, found)
	setPoint := pmetric.NewNumberDataPoint()
	c.SetNumberDataPoint(id, setPoint)
	point, found := c.GetNumberDataPoint(id)
	assert.Equal(t, point, setPoint)
	assert.True(t, found)
}

func TestShutdown(t *testing.T) {
	shutdown := make(chan struct{})
	c := Cache{
		numberCache:               make(map[uint64]usedNumberPoint),
		summaryCache:              make(map[uint64]usedSummaryPoint),
		histogramCache:            make(map[uint64]usedHistogramPoint),
		exponentialHistogramCache: make(map[uint64]usedExponentialHistogramPoint),
	}
	close(shutdown)
	// gc should return after shutdown is closed
	cont := c.gc(shutdown, make(chan time.Time))
	assert.False(t, cont)
}

func TestGC(t *testing.T) {
	shutdown := make(chan struct{})
	c := Cache{
		numberCache:               make(map[uint64]usedNumberPoint),
		summaryCache:              make(map[uint64]usedSummaryPoint),
		histogramCache:            make(map[uint64]usedHistogramPoint),
		exponentialHistogramCache: make(map[uint64]usedExponentialHistogramPoint),
	}
	fakeTicker := make(chan time.Time)
	id := uint64(12345)

	c.SetNumberDataPoint(id, pmetric.NumberDataPoint{})

	// bar exists since we just set it
	usedPoint, found := c.numberCache[id]
	assert.True(t, usedPoint.used.Load())
	assert.True(t, found)

	// first gc tick marks bar stale
	go func() {
		fakeTicker <- time.Now()
	}()
	cont := c.gc(shutdown, fakeTicker)
	assert.True(t, cont)
	usedPoint, found = c.numberCache[id]
	assert.False(t, usedPoint.used.Load())
	assert.True(t, found)

	// second gc tick removes bar
	go func() {
		fakeTicker <- time.Now()
	}()
	cont = c.gc(shutdown, fakeTicker)
	assert.True(t, cont)
	_, found = c.numberCache[id]
	assert.False(t, found)
}

func TestGetPreventsGC(t *testing.T) {
	id := uint64(12345)
	shutdown := make(chan struct{})
	c := Cache{
		numberCache:               make(map[uint64]usedNumberPoint),
		summaryCache:              make(map[uint64]usedSummaryPoint),
		histogramCache:            make(map[uint64]usedHistogramPoint),
		exponentialHistogramCache: make(map[uint64]usedExponentialHistogramPoint),
	}
	fakeTicker := make(chan time.Time)

	setPoint := pmetric.NewNumberDataPoint()
	c.SetNumberDataPoint(id, setPoint)
	// bar exists since we just set it
	_, found := c.numberCache[id]
	assert.True(t, found)
	// first gc tick marks bar stale
	go func() {
		fakeTicker <- time.Now()
	}()
	cont := c.gc(shutdown, fakeTicker)
	assert.True(t, cont)
	// calling Get() marks it fresh again.
	_, found = c.GetNumberDataPoint(id)
	assert.True(t, found)
	// second gc tick does not remove bar
	go func() {
		fakeTicker <- time.Now()
	}()
	cont = c.gc(shutdown, fakeTicker)
	assert.True(t, cont)
	_, found = c.numberCache[id]
	assert.True(t, found)
}

func TestConcurrentNumber(t *testing.T) {
	id := uint64(12345)
	c := Cache{
		numberCache:               make(map[uint64]usedNumberPoint),
		summaryCache:              make(map[uint64]usedSummaryPoint),
		histogramCache:            make(map[uint64]usedHistogramPoint),
		exponentialHistogramCache: make(map[uint64]usedExponentialHistogramPoint),
	}
	setPoint := pmetric.NewNumberDataPoint()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			c.SetNumberDataPoint(id, setPoint)
			point, found := c.GetNumberDataPoint(id)
			assert.Equal(t, point, setPoint)
			assert.True(t, found)
			wg.Done()
		}()
	}
	wg.Add(1)
	go func() {
		dontShutdown := make(chan struct{})
		fakeTick := make(chan time.Time)
		go func() { fakeTick <- time.Now() }()
		c.gc(dontShutdown, fakeTick)
		wg.Done()
	}()
	wg.Wait()
}

func TestConcurrentSummary(t *testing.T) {
	id := uint64(12345)
	c := Cache{
		numberCache:               make(map[uint64]usedNumberPoint),
		summaryCache:              make(map[uint64]usedSummaryPoint),
		histogramCache:            make(map[uint64]usedHistogramPoint),
		exponentialHistogramCache: make(map[uint64]usedExponentialHistogramPoint),
	}
	setPoint := pmetric.NewSummaryDataPoint()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			c.SetSummaryDataPoint(id, setPoint)
			point, found := c.GetSummaryDataPoint(id)
			assert.Equal(t, point, setPoint)
			assert.True(t, found)
			wg.Done()
		}()
	}
	wg.Add(1)
	go func() {
		dontShutdown := make(chan struct{})
		fakeTick := make(chan time.Time)
		go func() { fakeTick <- time.Now() }()
		c.gc(dontShutdown, fakeTick)
		wg.Done()
	}()
	wg.Wait()
}

func TestConcurrentHistogram(t *testing.T) {
	id := uint64(12345)
	c := Cache{
		numberCache:               make(map[uint64]usedNumberPoint),
		summaryCache:              make(map[uint64]usedSummaryPoint),
		histogramCache:            make(map[uint64]usedHistogramPoint),
		exponentialHistogramCache: make(map[uint64]usedExponentialHistogramPoint),
	}
	setPoint := pmetric.NewHistogramDataPoint()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			c.SetHistogramDataPoint(id, setPoint)
			point, found := c.GetHistogramDataPoint(id)
			assert.Equal(t, point, setPoint)
			assert.True(t, found)
			wg.Done()
		}()
	}
	wg.Add(1)
	go func() {
		dontShutdown := make(chan struct{})
		fakeTick := make(chan time.Time)
		go func() { fakeTick <- time.Now() }()
		c.gc(dontShutdown, fakeTick)
		wg.Done()
	}()
	wg.Wait()
}

func TestConcurrentExponentialHistogram(t *testing.T) {
	id := uint64(12345)
	c := Cache{
		numberCache:               make(map[uint64]usedNumberPoint),
		summaryCache:              make(map[uint64]usedSummaryPoint),
		histogramCache:            make(map[uint64]usedHistogramPoint),
		exponentialHistogramCache: make(map[uint64]usedExponentialHistogramPoint),
	}
	setPoint := pmetric.NewExponentialHistogramDataPoint()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			c.SetExponentialHistogramDataPoint(id, setPoint)
			point, found := c.GetExponentialHistogramDataPoint(id)
			assert.Equal(t, point, setPoint)
			assert.True(t, found)
			wg.Done()
		}()
	}
	wg.Add(1)
	go func() {
		dontShutdown := make(chan struct{})
		fakeTick := make(chan time.Time)
		go func() { fakeTick <- time.Now() }()
		c.gc(dontShutdown, fakeTick)
		wg.Done()
	}()
	wg.Wait()
}

func TestIdentifier(t *testing.T) {
	metricWithName := pmetric.NewMetric()
	metricWithName.SetName("custom.googleapis.com/test.metric")
	dpWithAttributes := pmetric.NewNumberDataPoint()
	dpWithAttributes.Attributes().PutStr("foo", "bar")
	monitoredResource := &monitoredrespb.MonitoredResource{
		Type: "generic_task",
		Labels: map[string]string{
			"foo": "bar",
		},
	}
	extraLabels := map[string]string{
		"foo": "bar",
	}
	for _, tc := range []struct {
		resource    *monitoredrespb.MonitoredResource
		extraLabels map[string]string
		metric      pmetric.Metric
		labels      pcommon.Map
		desc        string
		want        uint64
	}{
		{
			desc:   "empty",
			want:   16303200508382005237,
			metric: pmetric.NewMetric(),
			labels: pmetric.NewNumberDataPoint().Attributes(),
		},
		{
			desc:   "with name",
			want:   9745949301560396956,
			metric: metricWithName,
			labels: pmetric.NewNumberDataPoint().Attributes(),
		},
		{
			desc:   "with attributes",
			want:   2247677448538243046,
			metric: pmetric.NewMetric(),
			labels: dpWithAttributes.Attributes(),
		},
		{
			desc:     "with resource",
			want:     4344837656395061793,
			resource: monitoredResource,
			metric:   pmetric.NewMetric(),
			labels:   pmetric.NewNumberDataPoint().Attributes(),
		},
		{
			desc:        "with extra labels",
			want:        4054966704305404600,
			metric:      pmetric.NewMetric(),
			labels:      pmetric.NewNumberDataPoint().Attributes(),
			extraLabels: extraLabels,
		},
		{
			desc:        "with all",
			want:        2679814683341169356,
			metric:      metricWithName,
			labels:      dpWithAttributes.Attributes(),
			extraLabels: extraLabels,
			resource:    monitoredResource,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			origLabels := pcommon.NewMap()
			tc.labels.CopyTo(origLabels)
			got := Identifier(tc.resource, tc.extraLabels, tc.metric, tc.labels)
			if tc.want != got {
				t.Errorf("Identifier() = %d; want %d", got, tc.want)
			}
			assert.Equal(t, origLabels, tc.labels) // Make sure the labels are not mutated
		})
	}
}
