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
	"errors"
	"log"
	"log/slog"
	"runtime"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	otelsdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

const (
	name = "github.com/GoogleCloudPlatform/opentelemetry-operations-go/example/log/slogbridge/main"
)

func main() {
	ctx := context.Background()

	// Create OpenTelemetry Log exporter(s)
	// OTLP log exporter to export structuted logs to an OTLP endpoint
	otlpLogExporter, err := otlploghttp.New(ctx)
	if err != nil {
		log.Fatalf("failed to initialize OTLP log exporter")
	}
	// stdout log exporter to export structured logs to standard out
	stdoutLogExporter, err := stdoutlog.New()
	if err != nil {
		log.Fatalf("failed to initialize stdout log exporter")
	}

	// Add resource attributes using GCP resource detector
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
		log.Fatalf("resource.New: %v", err)
	}

	// Setup OpenTelemetry logger provider.
	// The logger provider is setup to export logs to both OTLP endpoint
	// and standard out.
	loggerProvider := otelsdklog.NewLoggerProvider(
		otelsdklog.WithProcessor(otelsdklog.NewBatchProcessor(otlpLogExporter)),
		otelsdklog.WithProcessor(otelsdklog.NewBatchProcessor(stdoutLogExporter)),
		otelsdklog.WithResource(res),
	)

	// Esnure provider shutdown to flush all logs before exit
	defer func() {
		if err = loggerProvider.Shutdown(ctx); err != nil {
			log.Println(err)
		}
	}()

	// emit structured logs to OTLP and standard out
	logger := otelslog.NewLogger(name,
		otelslog.WithLoggerProvider(loggerProvider),
		otelslog.WithSource(true),
		otelslog.WithVersion("v0.1.0"),
		otelslog.WithAttributes(
			attribute.String("fixed_label", "fixed_value"),
		))
	generateLogs(ctx, logger)
}

func generateLogs(ctx context.Context, logger *slog.Logger) {
	logger.DebugContext(ctx, "Sample debug application log")
	logger.WarnContext(ctx, "Sample warning log from application")
	logger.InfoContext(ctx, "Sample info application log")
	logger.InfoContext(ctx, "Sample log with key-value", "OS Name: ", runtime.GOOS)
	logger.InfoContext(ctx, "Sample log with stronlgy-typed contextual attributes", slog.String("os.arch", runtime.GOARCH))
	logger.ErrorContext(ctx, "Sample error log from application")
	// Logs with levels defined in OpenTelemetry Logging which are not built-in slog.
	//
	// Subtracting 9 from an OpenTelemetry level in the DEBUG, INFO, WARN and ERROR ranges
	// converts it to the corresponding slog Level range.
	// SeverityNumber range for OpenTelemetry Logs can be found at:
	// https://opentelemetry.io/docs/specs/otel/logs/data-model/#field-severitynumber
	// 21 is the starting level for FATAL Range in OpenTelemetry Logging
	logger.Log(ctx, 21-9, "Sample 'FATAL' level OTel log, shows as CRITICAL in Google Cloud")
	// 1 is the starting level for TRACE Range in OpenTelemetry Logging
	logger.Log(ctx, 1-9, "Sample 'TRACE' level OTel log, shows as DEBUG in Google Cloud")
}
