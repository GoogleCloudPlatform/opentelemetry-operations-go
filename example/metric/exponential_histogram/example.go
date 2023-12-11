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
	"fmt"
	"log"
	"time"

	mexporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric"

	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel/attribute"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/gonum/stat/distuv"
)

func workloadMetricsPrefixFormatter(d metricdata.Metrics) string {
	return fmt.Sprintf("custom.googleapis.com/%s", d.Name)
}

func main() {
	ctx := context.Background()

	// Resource for GCP and SDK detectors
	res, err := resource.New(ctx,
		resource.WithDetectors(gcp.NewDetector()),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("example-application"),
		),
	)
	if err != nil {
		log.Fatalf("resource.New: %v", err)
	}

	// Create exporter pipeline
	exporter, err := mexporter.New(
		mexporter.WithMetricDescriptorTypeFormatter(workloadMetricsPrefixFormatter),
	)
	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}

	view := sdkmetric.NewView(
		sdkmetric.Instrument{
			Name:  "latency",
			Scope: instrumentation.Scope{Name: "http"},
		},
		sdkmetric.Stream{
			Aggregation: sdkmetric.AggregationBase2ExponentialHistogram{
				MaxSize:  160,
				MaxScale: 20,
			},
		},
	)

	// initialize a MeterProvider with that periodically exports to the GCP exporter.
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithView(view),
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

	clabels := []attribute.KeyValue{attribute.Key("key").String("value")}
	histogram, err := meter.Float64Histogram(
		"latency",
		api.WithDescription("description"),
		//api.WithExplicitBucketBoundaries(-10, -9, -8, -7, -6, -5, -4, -3, -2, -1, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10),
	)
	if err != nil {
		log.Fatalf("Failed to create histogram: %v", err)
	}

	// Add measurement once an every 10 second.
	points := 10000
	timer := time.NewTicker(10 * time.Second)
	for range timer.C {

		// Create a normal distribution
		dist := distuv.LogNormal{
			Mu:    4,
			Sigma: .5,
		}

		data := make([]float64, points)

		// Draw some random values from the standard normal distribution
		for i := range data {
			data[i] = dist.Rand()
			histogram.Record(ctx, data[i], api.WithAttributes(clabels...))
		}
		mean, std := stat.MeanStdDev(data, nil)
		log.Printf("Sent : #points %d , mean %v, sdv %v", points, mean, std)
	}
}
