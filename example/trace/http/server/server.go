// Copyright 2019, OpenTelemetry Authors
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
	"io"
	"log"
	"net/http"
	"os"

	otelhttp "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/label"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	cloudtrace "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
)

func initTracer() func() {
	projectID := os.Getenv("PROJECT_ID")

	// Create Google Cloud Trace exporter to be able to retrieve
	// the collected spans.
	_, flush, err := cloudtrace.InstallNewPipeline(
		[]cloudtrace.Option{cloudtrace.WithProjectID(projectID)},
		// For this example code we use sdktrace.AlwaysSample sampler to sample all traces.
		// In a production application, use sdktrace.ProbabilitySampler with a desired probability.
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
	)
	if err != nil {
		log.Fatal(err)
	}
	return flush
}

func main() {
	flush := initTracer()
	defer flush()

	helloHandler := func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(label.String("server", "handling this..."))

		_, _ = io.WriteString(w, "Hello, world!\n")
	}
	otelHandler := otelhttp.NewHandler(http.HandlerFunc(helloHandler), "Hello")
	http.Handle("/hello", otelHandler)
	err := http.ListenAndServe(":7777", nil)
	if err != nil {
		panic(err)
	}
}
