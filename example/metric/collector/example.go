// Copyright 2023 Google LLC
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

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
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

type observedInt struct {
	mu sync.RWMutex
	i  int64
}

func (oi *observedInt) set(v int64) {
	oi.mu.Lock()
	defer oi.mu.Unlock()
	oi.i = v
}

func (oi *observedInt) get() int64 {
	oi.mu.RLock()
	defer oi.mu.RUnlock()
	return oi.i
}

func newObservedInt(v int64) *observedInt {
	return &observedInt{
		i: v,
	}
}

// Export OTLP metrtics using the otlpmetrichttp exporter.
// This example is used to demostrate how to use the collector approach to export metrics to Google Cloud.
func main() {
	ctx := context.Background()
	exp, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithInsecure())
	if err != nil {
		panic(err)
	}

	// initialize a MeterProvider with that periodically exports to the otlp exporter.
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)),
	)
	defer func() {
		if err = provider.Shutdown(ctx); err != nil {
			log.Fatalf("failed to shutdown meter provider: %v", err)
		}
	}()

	// Create a meter with which we will record metrics for our package.
	meter := provider.Meter("github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/collector/metric")

	// Register counter value
	counter, err := meter.Int64Counter("counter-a-collector")
	if err != nil {
		log.Fatalf("Failed to create counter: %v", err)
	}
	clabels := []attribute.KeyValue{attribute.Key("key").String("value")}
	counter.Add(ctx, 100, metric.WithAttributes(clabels...))

	histogram, err := meter.Float64Histogram("histogram-b-collector")
	if err != nil {
		log.Fatalf("Failed to create histogram: %v", err)
	}

	// Register observer value
	olabels := []attribute.KeyValue{
		attribute.String("foo", "Tokyo"),
		attribute.String("bar", "Sushi"),
	}
	of := newObservedFloat(12.34)

	gaugeObserver, err := meter.Float64ObservableGauge("observer-a-collector")
	if err != nil {
		log.Fatalf("failed to initialize instrument: %v", err)
	}
	_, err = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		v := of.get()
		o.ObserveFloat64(gaugeObserver, v, metric.WithAttributes(olabels...))
		return nil
	}, gaugeObserver)
	if err != nil {
		log.Fatalf("failed to register callback: %v", err)
	}

	oi := newObservedInt(3)
	// Export boolean as a custom type (Boolean types are not supported in OTLP)
	gaugeObserverBool, err := meter.Int64ObservableGauge("observer-boolean-collector", metric.WithUnit("{gcp.BOOL}"))
	if err != nil {
		log.Fatalf("failed to initialize instrument: %v", err)
	}
	_, err = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		v := oi.get()
		o.ObserveInt64(gaugeObserverBool, v, metric.WithAttributes(clabels...))
		return nil
	}, gaugeObserverBool)
	if err != nil {
		log.Fatalf("failed to register callback: %v", err)
	}

	// Add measurement once an every 10 second.
	timer := time.NewTicker(10 * time.Second)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for range timer.C {
		r := rng.Int63n(100)
		cv := 100 + r
		counter.Add(ctx, cv, metric.WithAttributes(clabels...))

		r2 := rng.Int63n(100)
		hv := float64(r2) / 20.0
		histogram.Record(ctx, hv, metric.WithAttributes(clabels...))
		ovf := 12.34 + hv
		of.set(ovf)

		r3 := rng.Int63n(2) // 0 or 1
		ovi := int64(r3)
		oi.set(ovi)

		log.Printf("Most recent data: counter %v, observer-float %v; observer-int %v; histogram %v", cv, ovf, ovi, hv)
	}
}
