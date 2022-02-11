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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestSetAndGet(t *testing.T) {
	c := make(Cache)
	c.Set("foo", nil)
	point, found := c.Get("foo")
	assert.Nil(t, point)
	assert.True(t, found)

	point, found = c.Get("bar")
	assert.Nil(t, point)
	assert.False(t, found)

	setPoint := pdata.NewNumberDataPoint()
	c.Set("bar", &setPoint)

	point, found = c.Get("bar")
	assert.Equal(t, point, &setPoint)
	assert.True(t, found)
}

func TestShutdown(t *testing.T) {
	shutdown := make(chan struct{})
	c := make(Cache)
	close(shutdown)
	// gc should return after shutdown is closed
	cont := c.gc(shutdown, make(chan time.Time))
	assert.False(t, cont)
}

func TestGC(t *testing.T) {
	shutdown := make(chan struct{})
	c := make(Cache)
	fakeTicker := make(chan time.Time)

	c.Set("bar", nil)

	// bar exists since we just set it
	usedPoint, found := c["bar"]
	assert.True(t, usedPoint.used)
	assert.True(t, found)

	// first gc tick marks bar stale
	go func() {
		fakeTicker <- time.Now()
	}()
	cont := c.gc(shutdown, fakeTicker)
	assert.True(t, cont)
	usedPoint, found = c["bar"]
	assert.False(t, usedPoint.used)
	assert.True(t, found)

	// second gc tick removes bar
	go func() {
		fakeTicker <- time.Now()
	}()
	cont = c.gc(shutdown, fakeTicker)
	assert.True(t, cont)
	_, found = c["bar"]
	assert.False(t, found)
}

func TestGetPreventsGC(t *testing.T) {
	shutdown := make(chan struct{})
	c := make(Cache)
	fakeTicker := make(chan time.Time)

	setPoint := pdata.NewNumberDataPoint()
	c.Set("bar", &setPoint)

	// bar exists since we just set it
	_, found := c["bar"]
	assert.True(t, found)

	// first gc tick marks bar stale
	go func() {
		fakeTicker <- time.Now()
	}()
	cont := c.gc(shutdown, fakeTicker)
	assert.True(t, cont)
	// calling Get() marks it fresh again.
	_, found = c.Get("bar")
	assert.True(t, found)

	// second gc tick does not remove bar
	go func() {
		fakeTicker <- time.Now()
	}()
	cont = c.gc(shutdown, fakeTicker)
	assert.True(t, cont)
	_, found = c["bar"]
	assert.True(t, found)
}
