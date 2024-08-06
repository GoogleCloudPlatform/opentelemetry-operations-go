// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"context"
	"os"
	"testing"

	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestSyncExporterNoDeadlock(t *testing.T) {
	ctx := context.Background()
	exporter, err := texporter.New(
		texporter.WithProjectID(os.Getenv("PROJECT_ID")))
	assert.NoError(t, err)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	ctx, span := tp.Tracer("test").Start(ctx, "span")
	// This will hang if the export itself creates a span.
	span.End()
	assert.NoError(t, tp.Shutdown(ctx))
}
