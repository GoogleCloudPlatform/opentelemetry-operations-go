// Copyright 2021 Google LLC
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

package trace_test

import (
	"log"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	cloudtrace "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
)

// For most programs, it's enough to install a tracing pipeline globally.
func Example_globalPipeline() {
	_, shutdown, err := cloudtrace.InstallNewPipeline(nil)
	if err != nil {
		log.Fatalf("unable to set up tracing: %v", err)
	}
	defer shutdown()

	// use global tracer, for example...
	_ = otelhttp.NewHandler(nil, "op")
}

// In some cases (e.g. multi-tenancy), it can be useful to have multiple tracers
// defined in the program and pick one explicitly.
func Example_explicitPipeline() {
	tp, shutdown, err := cloudtrace.NewExportPipeline(nil)
	if err != nil {
		log.Fatalf("unable to set up tracing: %v", err)
	}
	defer shutdown()

	// use tp explicitly, for example...
	_ = otelhttp.NewHandler(nil, "op", otelhttp.WithTracerProvider(tp))
}
