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
		numberCache:               make(map[string]usedNumberPoint),
		summaryCache:              make(map[string]usedSummaryPoint),
		histogramCache:            make(map[string]usedHistogramPoint),
		exponentialHistogramCache: make(map[string]usedExponentialHistogramPoint),
	}
	_, found := c.GetNumberDataPoint("bar")
	assert.False(t, found)
	setPoint := pmetric.NewNumberDataPoint()
	c.SetNumberDataPoint("bar", setPoint)
	point, found := c.GetNumberDataPoint("bar")
	assert.Equal(t, point, setPoint)
	assert.True(t, found)
}

func TestShutdown(t *testing.T) {
	shutdown := make(chan struct{})
	c := Cache{
		numberCache:               make(map[string]usedNumberPoint),
		summaryCache:              make(map[string]usedSummaryPoint),
		histogramCache:            make(map[string]usedHistogramPoint),
		exponentialHistogramCache: make(map[string]usedExponentialHistogramPoint),
	}
	close(shutdown)
	// gc should return after shutdown is closed
	cont := c.gc(shutdown, make(chan time.Time))
	assert.False(t, cont)
}

func TestGC(t *testing.T) {
	shutdown := make(chan struct{})
	c := Cache{
		numberCache:               make(map[string]usedNumberPoint),
		summaryCache:              make(map[string]usedSummaryPoint),
		histogramCache:            make(map[string]usedHistogramPoint),
		exponentialHistogramCache: make(map[string]usedExponentialHistogramPoint),
	}
	fakeTicker := make(chan time.Time)

	c.SetNumberDataPoint("bar", pmetric.NumberDataPoint{})

	// bar exists since we just set it
	usedPoint, found := c.numberCache["bar"]
	assert.True(t, usedPoint.used.Load())
	assert.True(t, found)

	// first gc tick marks bar stale
	go func() {
		fakeTicker <- time.Now()
	}()
	cont := c.gc(shutdown, fakeTicker)
	assert.True(t, cont)
	usedPoint, found = c.numberCache["bar"]
	assert.False(t, usedPoint.used.Load())
	assert.True(t, found)

	// second gc tick removes bar
	go func() {
		fakeTicker <- time.Now()
	}()
	cont = c.gc(shutdown, fakeTicker)
	assert.True(t, cont)
	_, found = c.numberCache["bar"]
	assert.False(t, found)
}

func TestGetPreventsGC(t *testing.T) {
	shutdown := make(chan struct{})
	c := Cache{
		numberCache:               make(map[string]usedNumberPoint),
		summaryCache:              make(map[string]usedSummaryPoint),
		histogramCache:            make(map[string]usedHistogramPoint),
		exponentialHistogramCache: make(map[string]usedExponentialHistogramPoint),
	}
	fakeTicker := make(chan time.Time)

	setPoint := pmetric.NewNumberDataPoint()
	c.SetNumberDataPoint("bar", setPoint)
	// bar exists since we just set it
	_, found := c.numberCache["bar"]
	assert.True(t, found)
	// first gc tick marks bar stale
	go func() {
		fakeTicker <- time.Now()
	}()
	cont := c.gc(shutdown, fakeTicker)
	assert.True(t, cont)
	// calling Get() marks it fresh again.
	_, found = c.GetNumberDataPoint("bar")
	assert.True(t, found)
	// second gc tick does not remove bar
	go func() {
		fakeTicker <- time.Now()
	}()
	cont = c.gc(shutdown, fakeTicker)
	assert.True(t, cont)
	_, found = c.numberCache["bar"]
	assert.True(t, found)
}

func TestConcurrentNumber(t *testing.T) {
	c := Cache{
		numberCache:               make(map[string]usedNumberPoint),
		summaryCache:              make(map[string]usedSummaryPoint),
		histogramCache:            make(map[string]usedHistogramPoint),
		exponentialHistogramCache: make(map[string]usedExponentialHistogramPoint),
	}
	setPoint := pmetric.NewNumberDataPoint()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			c.SetNumberDataPoint("bar", setPoint)
			point, found := c.GetNumberDataPoint("bar")
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
	c := Cache{
		numberCache:               make(map[string]usedNumberPoint),
		summaryCache:              make(map[string]usedSummaryPoint),
		histogramCache:            make(map[string]usedHistogramPoint),
		exponentialHistogramCache: make(map[string]usedExponentialHistogramPoint),
	}
	setPoint := pmetric.NewSummaryDataPoint()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			c.SetSummaryDataPoint("bar", setPoint)
			point, found := c.GetSummaryDataPoint("bar")
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
	c := Cache{
		numberCache:               make(map[string]usedNumberPoint),
		summaryCache:              make(map[string]usedSummaryPoint),
		histogramCache:            make(map[string]usedHistogramPoint),
		exponentialHistogramCache: make(map[string]usedExponentialHistogramPoint),
	}
	setPoint := pmetric.NewHistogramDataPoint()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			c.SetHistogramDataPoint("bar", setPoint)
			point, found := c.GetHistogramDataPoint("bar")
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
	c := Cache{
		numberCache:               make(map[string]usedNumberPoint),
		summaryCache:              make(map[string]usedSummaryPoint),
		histogramCache:            make(map[string]usedHistogramPoint),
		exponentialHistogramCache: make(map[string]usedExponentialHistogramPoint),
	}
	setPoint := pmetric.NewExponentialHistogramDataPoint()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			c.SetExponentialHistogramDataPoint("bar", setPoint)
			point, found := c.GetExponentialHistogramDataPoint("bar")
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
	dpWithAttributes.Attributes().PutStr("string", "strval")
	dpWithAttributes.Attributes().PutBool("bool", true)
	dpWithAttributes.Attributes().PutInt("int", 123)
	monitoredResource := &monitoredrespb.MonitoredResource{
		Type: "generic_task",
		Labels: map[string]string{
			"location": "us-central1-b",
			"project":  "project-foo",
		},
	}
	extraLabels := map[string]string{
		"foo":   "bar",
		"hello": "world",
	}
	for _, tc := range []struct {
		resource    *monitoredrespb.MonitoredResource
		extraLabels map[string]string
		metric      pmetric.Metric
		labels      pcommon.Map
		desc        string
		want        string
	}{
		{
			desc:   "empty",
			want:   " - map[] -  -",
			metric: pmetric.NewMetric(),
			labels: pmetric.NewNumberDataPoint().Attributes(),
		},
		{
			desc:   "with name",
			want:   " - map[] - custom.googleapis.com/test.metric -",
			metric: metricWithName,
			labels: pmetric.NewNumberDataPoint().Attributes(),
		},
		{
			desc:   "with attributes",
			want:   " - map[] -  - bool=true int=123 string=strval",
			metric: pmetric.NewMetric(),
			labels: dpWithAttributes.Attributes(),
		},
		{
			desc:     "with resource",
			want:     "map[location:us-central1-b project:project-foo] - map[] -  -",
			resource: monitoredResource,
			metric:   pmetric.NewMetric(),
			labels:   pmetric.NewNumberDataPoint().Attributes(),
		},
		{
			desc:        "with extra labels",
			want:        " - map[foo:bar hello:world] -  -",
			metric:      pmetric.NewMetric(),
			labels:      pmetric.NewNumberDataPoint().Attributes(),
			extraLabels: extraLabels,
		},
		{
			desc:        "with all",
			want:        "map[location:us-central1-b project:project-foo] - map[foo:bar hello:world] - custom.googleapis.com/test.metric - bool=true int=123 string=strval",
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
				t.Errorf("Identifier() = %q; want %q", got, tc.want)
			}
			assert.Equal(t, origLabels, tc.labels) // Make sure the labels are not mutated
		})
	}
}
