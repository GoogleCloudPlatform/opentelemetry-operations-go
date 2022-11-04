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
	"fmt"
	"time"

	traceapi "cloud.google.com/go/trace/apiv2"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	cloudtrace "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
)

// TraceExporter is a wrapper struct of OT cloud trace exporter
type TraceExporter struct {
	texporter *cloudtrace.Exporter
}

func (te *TraceExporter) Shutdown(ctx context.Context) error {
	return te.texporter.Shutdown(ctx)
}

func NewGoogleCloudTracesExporter(ctx context.Context, cfg Config, version string, timeout time.Duration) (*TraceExporter, error) {
	view.Register(ocgrpc.DefaultClientViews...)
	setVersionInUserAgent(&cfg, version)

	topts := []cloudtrace.Option{
		cloudtrace.WithProjectID(cfg.ProjectID),
		cloudtrace.WithTimeout(timeout),
	}
	if cfg.TraceConfig.AttributeMappings != nil {
		topts = append(topts, cloudtrace.WithAttributeMapping(mappingFuncFromAKM(cfg.TraceConfig.AttributeMappings)))
	}

	copts, err := generateClientOptions(ctx, &cfg.TraceConfig.ClientConfig, &cfg, traceapi.DefaultAuthScopes())
	if err != nil {
		return nil, err
	}
	topts = append(topts, cloudtrace.WithTraceClientOptions(copts))

	exp, err := cloudtrace.New(topts...)
	if err != nil {
		return nil, fmt.Errorf("error creating GoogleCloud Trace exporter: %w", err)
	}

	return &TraceExporter{texporter: exp}, nil
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

// PushTraces calls texporter.ExportSpan for each span in the given traces
func (te *TraceExporter) PushTraces(ctx context.Context, td ptrace.Traces) error {
	resourceSpans := td.ResourceSpans()
	spans := make([]sdktrace.ReadOnlySpan, 0, td.SpanCount())
	for i := 0; i < resourceSpans.Len(); i++ {
		sd := pdataResourceSpansToOTSpanData(resourceSpans.At(i))
		spans = append(spans, sd...)
	}

	return te.texporter.ExportSpans(ctx, spans)
}
