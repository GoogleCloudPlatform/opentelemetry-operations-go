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

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func initTracer(projectID string) (func(), error) {
	ctx := context.Background()

	creds, err := google.FindDefaultCredentials(ctx)
	if err != nil {
		panic(err)
	}

	// set OTEL_RESOURCE_ATTRIBUTES="gcp.project_id=<project_id>"
	// set endpoint with OTEL_EXPORTER_OTLP_ENDPOINT=https://<endpoint>
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithHTTPClient(oauth2.NewClient(ctx, creds.TokenSource)),
		otlptracehttp.WithHeaders(map[string]string{
			"x-goog-user-project": projectID,
		}))
	if err != nil {
		panic(err)
	}

	tp := sdktrace.NewTracerProvider(
		// For this example code we use sdktrace.AlwaysSample sampler to sample all traces.
		// In a production application, use sdktrace.ProbabilitySampler with a desired probability.
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

func main() {
	projectID := os.Getenv("GCLOUD_PROJECT")
	shutdown, err := initTracer(projectID)
	if err != nil {
		log.Fatal(err)
	}
	defer shutdown()
	tr := otel.Tracer("cloudtrace/example/client")

	ctx := context.Background()
	fmt.Println("starting span...")
	_, span := tr.Start(ctx, "test span", trace.WithAttributes(semconv.PeerServiceKey.String("ExampleService")))
	defer span.End()
	defer fmt.Println("ending span.")

	time.Sleep(3 * time.Second)
}
