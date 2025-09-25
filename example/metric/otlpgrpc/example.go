// Copyright 2024 Google LLC
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

	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/oauth"
)

func main() {
	ctx := context.Background()

	creds, err := oauth.NewApplicationDefault(ctx)
	if err != nil {
		panic(err)
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
		panic(err)
	}

	// Set endpoint with OTEL_EXPORTER_OTLP_ENDPOINT or OTEL_EXPORTER_OTLP_METRICS_ENDPOINT
	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithDialOption(grpc.WithPerRPCCredentials(creds)))
	if err != nil {
		panic(err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
	)

	defer func() {
		if err = meterProvider.Shutdown(ctx); err != nil {
			log.Println(err)
		}
	}()

	meter := meterProvider.Meter("github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/metric/otlpgrpc")

	// Register counter value
	counter, err := meter.Int64Counter("counter-a")
	if err != nil {
		log.Fatalf("Failed to create counter: %v", err)
	}
	clabels := []attribute.KeyValue{attribute.Key("key").String("value")}
	counter.Add(ctx, 100, metric.WithAttributes(clabels...))
}
