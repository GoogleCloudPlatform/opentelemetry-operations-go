// Copyright 2022 OpenTelemetry Authors
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

package config

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
)

const (
	typeStr = "googlecloud"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := newFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join("..", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Exporters), 2)

	r0 := cfg.Exporters[config.NewComponentID(typeStr)]
	assert.Equal(t, r0, factory.CreateDefaultConfig())

	r1 := cfg.Exporters[config.NewComponentIDWithName(typeStr, "customname")].(*testExporterConfig)
	assert.Equal(t, r1,
		&testExporterConfig{
			ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "customname")),
			Config: collector.Config{
				ProjectID: "my-project",
				UserAgent: "opentelemetry-collector-contrib {{version}}",
				TraceConfig: collector.TraceConfig{
					ClientConfig: collector.ClientConfig{
						Endpoint:    "test-trace-endpoint",
						UseInsecure: true,
					},
				},
				MetricConfig: collector.MetricConfig{
					ClientConfig: collector.ClientConfig{
						Endpoint:    "test-metric-endpoint",
						UseInsecure: true,
					},
					Prefix:                     "prefix",
					SkipCreateMetricDescriptor: true,
					KnownDomains: []string{
						"googleapis.com", "kubernetes.io", "istio.io", "knative.dev",
					},
					InstrumentationLibraryLabels:     true,
					CreateMetricDescriptorBufferSize: 10,
					ServiceResourceLabels:            true,
					ResourceMappings: []collector.ResourceMapping{
						{
							TargetType: "target-resource1",
							LabelMappings: []collector.LabelMapping{
								{
									SourceKey: "contrib.opencensus.io/exporter/googlecloud/project_id",
									TargetKey: "project_id",
								},
								{
									SourceKey: "source.label1",
									TargetKey: "target_label_1",
								},
							},
						},
						{
							TargetType: "target-resource2",
						},
					},
				},
			},
		})
}

func newFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		func() config.Exporter { return defaultConfig() },
	)
}

// testExporterConfig implements exporter.Config so we can test parsing of configuration
type testExporterConfig struct {
	config.ExporterSettings `mapstructure:",squash"`
	collector.Config        `mapstructure:",squash"`
}

func defaultConfig() *testExporterConfig {
	return &testExporterConfig{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		Config:           collector.DefaultConfig(),
	}
}
