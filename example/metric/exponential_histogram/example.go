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
	"os"
	"os/signal"
	"syscall"
	"time"

	mexporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric"
	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/gonum/stat/distuv"

	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	api "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
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

	clabels := []attribute.KeyValue{attribute.Key("key").String("value")}
	explicitBuckets := make([]float64, 35)
	for i := 0; i < 35; i++ {
		explicitBuckets[i] = float64(i*10 + 10)
	}

	view_a := sdkmetric.NewView(
		sdkmetric.Instrument{
			Name: "latency",
		},
		sdkmetric.Stream{
			Name: "latency_a",
			Aggregation: sdkmetric.AggregationBase2ExponentialHistogram{
				MaxSize:  160,
				MaxScale: 20,
			},
		},
	)

	view_b := sdkmetric.NewView(
		sdkmetric.Instrument{
			Name: "latency",
		},
		sdkmetric.Stream{
			Name: "latency_b",
			Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: explicitBuckets,
			},
		},
	)

	// initialize a MeterProvider with that periodically exports to the GCP exporter.
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithView(view_a),
		sdkmetric.WithView(view_b),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				exporter,
				sdkmetric.WithInterval(10*time.Second),
			),
		),
		sdkmetric.WithResource(res),
	)
	defer func() {
		if err = provider.Shutdown(ctx); err != nil {
			log.Fatalf("failed to shutdown meter provider: %v", err)
		}
	}()

	// Create a meter with which we will record metrics for our package.
	meter := provider.Meter("github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/metric")

	_, err = meter.Float64ObservableGauge(
		"latency",
		metric.WithDescription(
			"Latency",
		),
		metric.WithUnit("ms"),
		metric.WithFloat64Callback(func(_ context.Context, o metric.Float64Observer) error {

			points := 1000

			// Create a log normal distribution to simulate latency
			// from server response.
			dist := distuv.LogNormal{
				Mu:    4,
				Sigma: .5,
			}

			data := make([]float64, points)

			// Draw some random values from the log normal distribution
			for i := range data {
				data[i] = dist.Rand()

				// Gauge metric observation
				o.Observe(data[i], api.WithAttributes(clabels...))
			}
			mean, std := stat.MeanStdDev(data, nil)
			log.Printf("Sent : #points %d , mean %v, sdv %v", points, mean, std)
			return nil
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create gauge: %v", err)
	}

	// Wait for interrupt
	var stopChan = make(chan os.Signal, 2)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	<-stopChan
}