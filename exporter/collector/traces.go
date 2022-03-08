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
	"strings"
	"time"

	traceapi "cloud.google.com/go/trace/apiv2"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

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

func setProjectFromADC(ctx context.Context, cfg *Config, scopes []string) error {
	if cfg.ProjectID == "" {
		creds, err := google.FindDefaultCredentials(ctx, scopes...)
		if err != nil {
			return fmt.Errorf("error finding default application credentials: %v", err)
		}
		if creds.ProjectID == "" {
			return errors.New("no project found with application default credentials")
		}
		cfg.ProjectID = creds.ProjectID
	}
	return nil
}

func generateClientOptions(cfg *ClientConfig, userAgent string) ([]option.ClientOption, error) {
	var copts []option.ClientOption
	// option.WithUserAgent is used by the Trace exporter, but not the Metric exporter (see comment below)
	if userAgent != "" {
		copts = append(copts, option.WithUserAgent(userAgent))
	}
	if cfg.Endpoint != "" {
		if cfg.UseInsecure {
			// option.WithGRPCConn option takes precedent over all other supplied options so the
			// following user agent will be used by both exporters if we reach this branch
			dialOpts := []grpc.DialOption{
				grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			}
			if userAgent != "" {
				dialOpts = append(dialOpts, grpc.WithUserAgent(userAgent))
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

func NewGoogleCloudTracesExporter(ctx context.Context, cfg Config, version string, timeout time.Duration) (*TraceExporter, error) {
	view.Register(ocgrpc.DefaultClientViews...)
	setVersionInUserAgent(&cfg, version)
	setProjectFromADC(ctx, &cfg, traceapi.DefaultAuthScopes())

	topts := []cloudtrace.Option{
		cloudtrace.WithProjectID(cfg.ProjectID),
		cloudtrace.WithTimeout(timeout),
	}
	if cfg.TraceConfig.AttributeKeyMappings != nil {
		topts = append(topts, cloudtrace.WithAttributeKeyMapping(mappingFuncFromAKM(cfg.TraceConfig.AttributeKeyMappings)))
	}

	copts, err := generateClientOptions(&cfg.TraceConfig.ClientConfig, cfg.UserAgent)
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

func mappingFuncFromAKM(akm []AttributeKeyMapping) func(attribute.Key) attribute.Key {
	// convert list to map for easy lookups
	mapFromConfig := map[string]string{}
	for _, mapping := range akm {
		mapFromConfig[mapping.Key] = mapping.Replacement
	}
	return func(input attribute.Key) attribute.Key {
		// if a replacement was specified in the config, use it.
		if replacement, ok := mapFromConfig[string(input)]; ok {
			return attribute.Key(replacement)
		}
		// otherwise, leave the attribute as-is
		return attribute.Key(input)
	}
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
