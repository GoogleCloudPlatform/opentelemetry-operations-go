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

package collector

import (
	"context"
	"sync"
	"time"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/service/featuregate"
)

const (
	// The value of "type" key in configuration.
	typeStr                  = "googlecloud"
	defaultTimeout           = 12 * time.Second // Consistent with Cloud Monitoring's timeout
	pdataExporterFeatureGate = "exporter.googlecloud.OTLPDirect"
)

func init() {
	featuregate.Register(featuregate.Gate{
		ID:          pdataExporterFeatureGate,
		Description: "When enabled, the googlecloud exporter translates pdata directly to google cloud monitoring's types, rather than first translating to opencensus.",
		Enabled:     false,
	})
}

var once sync.Once

// NewFactory creates a factory for the googlecloud exporter
func NewFactory() component.ExporterFactory {
	// register view for self-observability
	once.Do(func() {
		view.Register(viewPointCount)
	})

	return exporterhelper.NewFactory(
		typeStr,
		func() config.Exporter { return createDefaultConfig() },
		exporterhelper.WithTraces(createTracesExporter),
		exporterhelper.WithMetrics(createMetricsExporter),
	)
}

// createDefaultConfig creates the default configuration for exporter.
func createDefaultConfig() *Config {
	cfg := &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		TimeoutSettings:  exporterhelper.TimeoutSettings{Timeout: defaultTimeout},
		RetrySettings:    exporterhelper.DefaultRetrySettings(),
		QueueSettings:    exporterhelper.DefaultQueueSettings(),
		UserAgent:        "opentelemetry-collector-contrib {{version}}",
	}
	if featuregate.IsEnabled(pdataExporterFeatureGate) {
		cfg.MetricConfig = MetricConfig{
			KnownDomains:                     domains,
			Prefix:                           "workload.googleapis.com",
			CreateMetricDescriptorBufferSize: 10,
		}
	}
	return cfg
}

// createTracesExporter creates a trace exporter based on this config.
func createTracesExporter(
	_ context.Context,
	params component.ExporterCreateSettings,
	cfg config.Exporter) (component.TracesExporter, error) {
	eCfg := cfg.(*Config)
	return newGoogleCloudTracesExporter(eCfg, params)
}

// createMetricsExporter creates a metrics exporter based on this config.
func createMetricsExporter(
	ctx context.Context,
	params component.ExporterCreateSettings,
	cfg config.Exporter) (component.MetricsExporter, error) {
	eCfg := cfg.(*Config)
	if !featuregate.IsEnabled(pdataExporterFeatureGate) {
		return newLegacyGoogleCloudMetricsExporter(ctx, eCfg, params)
	}
	return newGoogleCloudMetricsExporter(ctx, eCfg, params)
}
