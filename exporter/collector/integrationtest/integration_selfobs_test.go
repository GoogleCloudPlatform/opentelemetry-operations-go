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

//go:build integrationtest
// +build integrationtest

package integrationtest

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"

	mexporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric"
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	exporter, err := mexporter.New(mexporter.WithProjectID(os.Getenv("PROJECT_ID")))
	if err != nil {
		log.Fatalf("Failed to create self-obs exporter: %v", err)
	}
	version4Uuid, err := uuid.NewRandom()
	if err != nil {
		log.Fatalf("Failed to create uuid for resource: %v", err)
	}
	res, err := resource.New(
		ctx,
		resource.WithTelemetrySDK(),
		resource.WithFromEnv(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("integrationtest"),
			semconv.ServiceInstanceIDKey.String(version4Uuid.String()),
		),
	)
	if err != nil {
		log.Fatalf("Failed to create self-obs resource: %v", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
	)
	defer mp.Shutdown(ctx)
	otel.SetMeterProvider(mp)
	m.Run()
}
