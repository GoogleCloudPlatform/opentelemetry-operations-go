// Copyright 2019 OpenTelemetry Authors
// Copyright 2025 Google LLC
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
	"flag"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

var keepRunning = flag.Bool("keepRunning", false, "Set to true for generating spans at a fixed rate indefinitely. Default is false.")

func initTracer() (func(), error) {
	ctx := context.Background()

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint("0.0.0.0:4317"))
	if err != nil {
		panic(err)
	}

	res, _ := resource.New(
		ctx,
		// Keep the default detectors
		// Additional resource attributes can be added from the collector side
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("collector-trace-example"),
		),
	)

	tp := sdktrace.NewTracerProvider(
		// For this example code we use sdktrace.AlwaysSample sampler to sample all traces.
		// In a production application, use sdktrace.TraceIDRatioBased with a desired probability.
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter))

	otel.SetTracerProvider(tp)
	return func() {
		err := tp.Shutdown(context.Background())
		if err != nil {
			fmt.Printf("error shutting down trace provider: %+v", err)
		}
	}, nil
}

func generateTestSpan(ctx context.Context, tr trace.Tracer, description string) {
	fmt.Println("starting span...")
	_, span := tr.Start(ctx, description, trace.WithAttributes(semconv.PeerServiceKey.String("ExampleService")))
	defer span.End()
	defer fmt.Println("ending span.")

	time.Sleep(3 * time.Second)
}

func generateSpansAtFixedRate(ctx context.Context, tr trace.Tracer) {
	fmt.Println("Generating 1 test span every 10 seconds indefinitely")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for tick := range ticker.C {
		go generateTestSpan(ctx, tr, fmt.Sprintf("span-%s", tick))
	}
	fmt.Println("Span generation complete.")
}

func main() {
	flag.Parse()
	shutdown, err := initTracer()
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown()
	tr := otel.Tracer("cloudtrace/example/client")

	ctx := context.Background()
	if *keepRunning {
		generateSpansAtFixedRate(ctx, tr)
	} else {
		generateTestSpan(ctx, tr, "test span")
	}
}
