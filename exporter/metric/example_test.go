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

package metric_test

import (
	"context"
	"log"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	mexporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric"
)

func ExampleNew() {
	exporter, err := mexporter.New()
	if err != nil {
		log.Printf("Failed to create exporter: %v", err)
		return
	}
	// initialize a MeterProvider with that periodically exports to the GCP exporter.
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
	)
	ctx := context.Background()
	defer func() {
		if err = provider.Shutdown(ctx); err != nil {
			log.Printf("Failed to shut down meter provider: %v", err)
		}
	}()

	// Start meter
	meter := provider.Meter("github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/metric")

	counter, err := meter.Int64Counter("counter-foo")
	if err != nil {
		log.Printf("Failed to create counter: %v", err)
		return
	}
	attrs := []attribute.KeyValue{
		attribute.Key("key").String("value"),
	}
	counter.Add(ctx, 123, metric.WithAttributes(attrs...))
}
