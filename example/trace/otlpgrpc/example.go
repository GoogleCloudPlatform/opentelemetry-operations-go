// Copyright 2019 OpenTelemetry Authors
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

// [START opentelemetry_otlp_grpc_imports].
import (
	"context"
	// [START_EXCLUDE silent].
	"flag"
	"fmt"
	"log"
	"time"

	// [END_EXCLUDE].
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	// [START_EXCLUDE].
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"

	// [START opentelemetry_otlp_grpc_auth_imports].
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/oauth"
	// [END opentelemetry_otlp_grpc_auth_imports].
	// [END_EXCLUDE].
)

// [END opentelemetry_otlp_grpc_imports]

var keepRunning = flag.Bool("keepRunning", false, "Set to true for generating spans at a fixed rate indefinitely. Default is false.")

func initTracer() (func(), error) {
	// [START opentelemetry_otlp_grpc_init]
	// Initializes OpenTelemetry with OTLP exporters
	ctx := context.Background()

	// Configure gRPC client with Google Application Default Credentials
	creds, err := oauth.NewApplicationDefault(ctx)
	if err != nil {
		panic(err)
	}

	// set OTEL_RESOURCE_ATTRIBUTES="gcp.project_id=<project_id>"
	// set endpoint with OTEL_EXPORTER_OTLP_ENDPOINT=https://<endpoint>
	// set OTEL_EXPORTER_OTLP_HEADERS="x-goog-user-project=<project_id>"
	// Configure exporter to use the credentials
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithDialOption(grpc.WithPerRPCCredentials(creds)))
	if err != nil {
		panic(err)
	}

	tp := sdktrace.NewTracerProvider(
		// For this example code we use sdktrace.AlwaysSample sampler to sample all traces.
		// In a production application, use sdktrace.TraceIDRatioBased with a desired probability.
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter))

	otel.SetTracerProvider(tp)
	// [END opentelemetry_otlp_grpc_init]
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
