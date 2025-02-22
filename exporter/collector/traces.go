// Copyright 2021 OpenTelemetry Authors
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

// Package collector contains the wrapper for OpenTelemetry-GoogleCloud
// exporter to be used in opentelemetry-collector.
package collector

import (
	"context"
	"errors"
	"fmt"
	"time"

	traceapi "cloud.google.com/go/trace/apiv2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
)

// TraceExporter is a wrapper struct of OT cloud trace exporter.
type TraceExporter struct {
	obs       selfObservability
	texporter *texporter.Exporter
	cfg       Config
	timeout   time.Duration
}

func (te *TraceExporter) Shutdown(ctx context.Context) error {
	if te.texporter != nil {
		return te.texporter.Shutdown(ctx)
	}
	return nil
}

func NewGoogleCloudTracesExporter(
	ctx context.Context,
	cfg Config,
	set exporter.Settings,
	timeout time.Duration,
) (*TraceExporter, error) {
	SetUserAgent(&cfg, set.BuildInfo)
	obs := selfObservability{
		log:           set.TelemetrySettings.Logger,
		meterProvider: set.TelemetrySettings.MeterProvider,
	}
	return &TraceExporter{cfg: cfg, timeout: timeout, obs: obs}, nil
}

func (te *TraceExporter) Start(ctx context.Context, _ component.Host) error {
	topts := []texporter.Option{
		texporter.WithProjectID(te.cfg.ProjectID),
		texporter.WithTimeout(te.timeout),
	}

	if te.cfg.DestinationProjectQuota {
		topts = append(topts, texporter.WithDestinationProjectQuota())
	}

	if te.cfg.TraceConfig.AttributeMappings != nil {
		topts = append(topts, texporter.WithAttributeMapping(mappingFuncFromAKM(te.cfg.TraceConfig.AttributeMappings)))
	}

	copts, err := generateClientOptions(ctx, &te.cfg.TraceConfig.ClientConfig, &te.cfg, traceapi.DefaultAuthScopes(), te.obs.meterProvider)
	if err != nil {
		return err
	}
	topts = append(topts, texporter.WithTraceClientOptions(copts))

	exp, err := texporter.New(topts...)
	if err != nil {
		return fmt.Errorf("error creating GoogleCloud Trace exporter: %w", err)
	}
	te.texporter = exp
	return nil
}

func mappingFuncFromAKM(akm []AttributeMapping) func(attribute.Key) attribute.Key {
	// convert list to map for easy lookups
	mapFromConfig := make(map[string]string, len(akm))
	for _, mapping := range akm {
		mapFromConfig[mapping.Key] = mapping.Replacement
	}
	return func(input attribute.Key) attribute.Key {
		// if a replacement was specified in the config, use it.
		if replacement, ok := mapFromConfig[string(input)]; ok {
			return attribute.Key(replacement)
		}
		// otherwise, leave the attribute as-is
		return input
	}
}

// PushTraces calls texporter.ExportSpan for each span in the given traces.
func (te *TraceExporter) PushTraces(ctx context.Context, td ptrace.Traces) error {
	if te.texporter == nil {
		return errors.New("not started")
	}
	resourceSpans := td.ResourceSpans()
	spans := make([]sdktrace.ReadOnlySpan, 0, td.SpanCount())
	for i := 0; i < resourceSpans.Len(); i++ {
		sd := pdataResourceSpansToOTSpanData(resourceSpans.At(i))
		spans = append(spans, sd...)
	}

	return te.texporter.ExportSpans(ctx, spans)
}
