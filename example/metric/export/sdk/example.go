// Copyright 2020 Google LLC
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
	"errors"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	mexporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric"

	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
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
	ctx := context.Background()
	// Initialization. In order to pass the credentials to the exporter,
	// prepare credential file following the instruction described in this doc.
	// https://pkg.go.dev/golang.org/x/oauth2/google?tab=doc#FindDefaultCredentials
	exporter, err := mexporter.New(mexporter.WithProjectID(os.Getenv("GOOGLE_PROJECT_ID")))
	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}

	res, err := resource.New(
		ctx,
		// Use the GCP resource detector to detect information about the GCP platform
		resource.WithDetectors(gcp.NewDetector()),
		// Keep the default detectors
		resource.WithTelemetrySDK(),
		// Add attributes from environment variables
		resource.WithFromEnv(),
		// Add your own custom attributes to identify your application
		resource.WithAttributes(
			semconv.ServiceNameKey.String("example-application"),
		),
	)
	if errors.Is(err, resource.ErrPartialResource) || errors.Is(err, resource.ErrSchemaURLConflict) {
		log.Println(err)
	} else if err != nil {
		log.Fatalf("resource.New: %v", err)
	}

	// initialize a MeterProvider with that periodically exports to the GCP exporter.
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(res),
	)
	defer func() {
		if err = provider.Shutdown(ctx); err != nil {
			log.Fatalf("failed to shutdown meter provider: %v", err)
		}
	}()

	// Create a meter with which we will record metrics for our package.
	meter := provider.Meter("github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/metric")

	// Register counter value
	counter, err := meter.Int64Counter("counter-a")
	if err != nil {
		log.Fatalf("Failed to create counter: %v", err)
	}
	clabels := []attribute.KeyValue{attribute.Key("key").String("value")}
	counter.Add(ctx, 100, metric.WithAttributes(clabels...))

	histogram, err := meter.Float64Histogram("histogram-b")
	if err != nil {
		log.Fatalf("Failed to create histogram: %v", err)
	}

	// Register observer value
	olabels := []attribute.KeyValue{
		attribute.String("foo", "Tokyo"),
		attribute.String("bar", "Sushi"),
	}
	of := newObservedFloat(12.34)

	gaugeObserver, err := meter.Float64ObservableGauge("observer-a")
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
		ov := 12.34 + hv
		of.set(ov)
		log.Printf("Most recent data: counter %v, observer %v; histogram %v", cv, ov, hv)
	}
}
