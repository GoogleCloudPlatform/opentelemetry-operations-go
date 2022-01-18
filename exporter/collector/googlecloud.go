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
	"strings"
	"time"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/model/pdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/api/option"
	"google.golang.org/grpc"

	cloudtrace "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
)

// TraceExporter is a wrapper struct of OT cloud trace exporter
type TraceExporter struct {
	texporter *cloudtrace.Exporter
}

func (te *TraceExporter) Shutdown(ctx context.Context) error {
	return te.texporter.Shutdown(ctx)
}

func setVersionInUserAgent(cfg *Config, version string) {
	cfg.UserAgent = strings.ReplaceAll(cfg.UserAgent, "{{version}}", version)
}

func generateClientOptions(cfg *Config) ([]option.ClientOption, error) {
	var copts []option.ClientOption
	// option.WithUserAgent is used by the Trace exporter, but not the Metric exporter (see comment below)
	if cfg.UserAgent != "" {
		copts = append(copts, option.WithUserAgent(cfg.UserAgent))
	}
	if cfg.Endpoint != "" {
		if cfg.UseInsecure {
			// option.WithGRPCConn option takes precedent over all other supplied options so the
			// following user agent will be used by both exporters if we reach this branch
			dialOpts := []grpc.DialOption{
				grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
				grpc.WithInsecure(),
			}
			if cfg.UserAgent != "" {
				dialOpts = append(dialOpts, grpc.WithUserAgent(cfg.UserAgent))
			}
			conn, err := grpc.Dial(cfg.Endpoint, dialOpts...)
			if err != nil {
				return nil, fmt.Errorf("cannot configure grpc conn: %w", err)
			}
			copts = append(copts, option.WithGRPCConn(conn))
		} else {
			copts = append(copts, option.WithEndpoint(cfg.Endpoint))
		}
	}
	if cfg.GetClientOptions != nil {
		copts = append(copts, cfg.GetClientOptions()...)
	}
	return copts, nil
}

func NewGoogleCloudTracesExporter(cfg Config, version string, timeout time.Duration) (*TraceExporter, error) {
	view.Register(ocgrpc.DefaultClientViews...)
	setVersionInUserAgent(&cfg, version)

	topts := []cloudtrace.Option{
		cloudtrace.WithProjectID(cfg.ProjectID),
		cloudtrace.WithTimeout(timeout),
	}

	copts, err := generateClientOptions(&cfg)
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

// PushTraces calls texporter.ExportSpan for each span in the given traces
func (te *TraceExporter) PushTraces(ctx context.Context, td pdata.Traces) error {
	resourceSpans := td.ResourceSpans()
	spans := make([]sdktrace.ReadOnlySpan, 0, td.SpanCount())
	for i := 0; i < resourceSpans.Len(); i++ {
		sd := pdataResourceSpansToOTSpanData(resourceSpans.At(i))
		spans = append(spans, sd...)
	}

	return te.texporter.ExportSpans(ctx, spans)
}
