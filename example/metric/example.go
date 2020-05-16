// Copyright 2020, Google Inc.
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

package main

import (
	"context"
	"log"
	"math/rand"
	"sync"
	"time"

	mexporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric"

	"go.opentelemetry.io/otel/api/kv"
	"go.opentelemetry.io/otel/api/metric"
	"go.opentelemetry.io/otel/sdk/metric/controller/push"
	"go.opentelemetry.io/otel/sdk/resource"
)

type observedFloat struct {
	mu sync.RWMutex
	f  float64
}

func (of *observedFloat) set(v float64) {
	of.mu.Lock()
	defer of.mu.Unlock()
	of.f = v
}

func (of *observedFloat) get() float64 {
	of.mu.RLock()
	defer of.mu.RUnlock()
	return of.f
}

func newObservedFloat(v float64) *observedFloat {
	return &observedFloat{
		f: v,
	}
}

func main() {
	// Initialization. In order to pass the credentials to the exporter,
	// prepare credential file following the instruction described in this doc.
	// https://pkg.go.dev/golang.org/x/oauth2/google?tab=doc#FindDefaultCredentials
	opts := []mexporter.Option{}
	resOpt := push.WithResource(
		resource.New(
			kv.String("instance_id", "abc123"),
			kv.String("application", "example-app"),
		),
	)
	pusher, err := mexporter.InstallNewPipeline(opts, resOpt)
	if err != nil {
		log.Fatalf("Failed to establish pipeline: %v", err)
	}
	defer pusher.Stop()

	// Start meter
	ctx := context.Background()
	meter := pusher.Meter("cloudmonitoring/example")

	// Register counter value
	counter := metric.Must(meter).NewInt64Counter("counter-a")
	clabels := []kv.KeyValue{kv.Key("key").String("value")}
	counter.Add(ctx, 100, clabels...)

	// Register observer value
	olabels := []kv.KeyValue{
		kv.String("foo", "Tokyo"),
		kv.String("bar", "Sushi"),
	}
	of := newObservedFloat(12.34)

	observerV := new(float64)
	callback := func(result metric.Float64ObserverResult) {
		v := of.get()
		result.Observe(v, olabels...)
	}

	metric.Must(meter).RegisterFloat64Observer("observer-a", callback)
	*observerV = of.get()

	// Add measurement once an every 10 second.
	timer := time.NewTicker(10 * time.Second)
	for range timer.C {
		rand.Seed(time.Now().UnixNano())

		r := rand.Int63n(100)
		cv := 100 + r
		counter.Add(ctx, cv, clabels...)

		r2 := rand.Int63n(10)
		ov := 12.34 + float64(r2)/20.0
		of.set(ov)
		log.Printf("Submitted data: counter %v, observer %v", cv, ov)
	}
}
