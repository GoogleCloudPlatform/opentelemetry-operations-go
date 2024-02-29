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
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/gonum/stat/distuv"

	mexporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric"

	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func workloadMetricsPrefixFormatter(d metricdata.Metrics) string {
	return fmt.Sprintf("workload.googleapis.com/%s", d.Name)
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
	defaultBuckets := []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 1000}
	linearBuckets := make([]float64, 35)
	for i := 0; i < 35; i++ {
		linearBuckets[i] = float64(i*10 + 10)
	}

	viewLatencyA := sdkmetric.NewView(
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

	viewLatencyB := sdkmetric.NewView(
		sdkmetric.Instrument{
			Name: "latency",
		},
		sdkmetric.Stream{
			Name: "latency_b",
			Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: linearBuckets,
			},
		},
	)

	viewLatencyC := sdkmetric.NewView(
		sdkmetric.Instrument{
			Name: "latency",
		},
		sdkmetric.Stream{
			Name: "latency_c",
			Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: defaultBuckets,
			},
		},
	)

	viewLatencyShiftedA := sdkmetric.NewView(
		sdkmetric.Instrument{
			Name: "latency_shifted",
		},
		sdkmetric.Stream{
			Name: "latency_shifted_a",
			Aggregation: sdkmetric.AggregationBase2ExponentialHistogram{
				MaxSize:  160,
				MaxScale: 20,
			},
		},
	)

	viewLatencyShiftedB := sdkmetric.NewView(
		sdkmetric.Instrument{
			Name: "latency_shifted",
		},
		sdkmetric.Stream{
			Name: "latency_shifted_b",
			Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: linearBuckets,
			},
		},
	)

	viewLatencyShiftedC := sdkmetric.NewView(
		sdkmetric.Instrument{
			Name: "latency_shifted",
		},
		sdkmetric.Stream{
			Name: "latency_shifted_c",
			Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: defaultBuckets,
			},
		},
	)

	viewLatencyMultimodalA := sdkmetric.NewView(
		sdkmetric.Instrument{
			Name: "latency_multimodal",
		},
		sdkmetric.Stream{
			Name: "latency_multimodal_a",
			Aggregation: sdkmetric.AggregationBase2ExponentialHistogram{
				MaxSize:  160,
				MaxScale: 20,
			},
		},
	)

	viewLatencyMultimodalB := sdkmetric.NewView(
		sdkmetric.Instrument{
			Name: "latency_multimodal",
		},
		sdkmetric.Stream{
			Name: "latency_multimodal_b",
			Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: linearBuckets,
			},
		},
	)

	viewLatencyMultimodalC := sdkmetric.NewView(
		sdkmetric.Instrument{
			Name: "latency_multimodal",
		},
		sdkmetric.Stream{
			Name: "latency_multimodal_c",
			Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: defaultBuckets,
			},
		},
	)

	// initialize a MeterProvider with that periodically exports to the GCP exporter.
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithView(viewLatencyA),
		sdkmetric.WithView(viewLatencyB),
		sdkmetric.WithView(viewLatencyC),
		sdkmetric.WithView(viewLatencyShiftedA),
		sdkmetric.WithView(viewLatencyShiftedB),
		sdkmetric.WithView(viewLatencyShiftedC),
		sdkmetric.WithView(viewLatencyMultimodalA),
		sdkmetric.WithView(viewLatencyMultimodalB),
		sdkmetric.WithView(viewLatencyMultimodalC),
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
		metric.WithUnit("s"),
		metric.WithFloat64Callback(func(_ context.Context, o metric.Float64Observer) error {
			points := 1000

			// Create a log normal distribution to simulate latency
			// from server response.
			dist := distuv.LogNormal{
				Mu:    3.5,
				Sigma: .5,
			}

			data := make([]float64, points)

			// Draw some random values from the log normal distribution
			for i := range data {
				data[i] = dist.Rand()

				// Gauge metric observation
				o.Observe(data[i], metric.WithAttributes(clabels...))
			}
			mean, std := stat.MeanStdDev(data, nil)
			log.Printf("Sent Latency Data (Original Distribution): #points %d , mean %v, sdv %v", points, mean, std)
			return nil
		}),
	)

	_, err = meter.Float64ObservableGauge(
		"latency_shifted",
		metric.WithDescription(
			"Latency Shifted",
		),
		metric.WithUnit("s"),
		metric.WithFloat64Callback(func(_ context.Context, o metric.Float64Observer) error {
			points := 1000

			// Create a log normal distribution to simulate latency
			// from server response.
			dist := distuv.LogNormal{
				Mu:    5.5,
				Sigma: .5,
			}

			data := make([]float64, points)

			// Draw some random values from the log normal distribution
			for i := range data {
				data[i] = dist.Rand()

				// Gauge metric observation
				o.Observe(data[i], metric.WithAttributes(clabels...))
			}
			mean, std := stat.MeanStdDev(data, nil)
			log.Printf("Sent Latency Data (Shifted Distribution): #points %d , mean %v, sdv %v", points, mean, std)
			return nil
		}),
	)

	_, err = meter.Float64ObservableGauge(
		"latency_multimodal",
		metric.WithDescription(
			"Latency Multimodal",
		),
		metric.WithUnit("s"),
		metric.WithFloat64Callback(func(_ context.Context, o metric.Float64Observer) error {
			points := 1000

			// Create a multimodal normal
			dist1 := distuv.LogNormal{
				Mu:    3.5,
				Sigma: .5,
			}
			dist2 := distuv.LogNormal{
				Mu:    5.5,
				Sigma: .5,
			}

			data := make([]float64, points)

			// Draw some random values from the log normal distribution
			for i := range data {
				if rand.Float64() < .5 {
					data[i] = dist1.Rand()
				} else {
					data[i] = dist2.Rand()
				}

				// Gauge metric observation
				o.Observe(data[i], metric.WithAttributes(clabels...))
			}
			mean, std := stat.MeanStdDev(data, nil)
			log.Printf("Sent Latency Data (Multimodal Distribution): #points %d , mean %v, sdv %v", points, mean, std)
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
