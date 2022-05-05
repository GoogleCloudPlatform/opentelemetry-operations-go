// Copyright 2022 Google LLC
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
	"go.opentelemetry.io/collector/service/servicetest"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus"
)

const (
	gmpTypeStr = "googlemanagedprometheus"
)

func TestLoadGMPConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := newGMPFactory()
	factories.Exporters[gmpTypeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(path.Join("..", "testdata", "gmp_config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Exporters), 2)

	r0 := cfg.Exporters[config.NewComponentID(gmpTypeStr)].(*gmpTestExporterConfig)
	defaultConfig := factory.CreateDefaultConfig().(*gmpTestExporterConfig)
	assert.Equal(t, r0, defaultConfig)

	r1 := cfg.Exporters[config.NewComponentIDWithName(gmpTypeStr, "customname")].(*gmpTestExporterConfig)
	assert.Equal(t, r1,
		&gmpTestExporterConfig{
			ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(gmpTypeStr, "customname")),
			GMPConfig: googlemanagedprometheus.GMPConfig{
				ProjectID: "my-project",
				UserAgent: "opentelemetry-collector-contrib {{version}}",
				ClientConfig: collector.ClientConfig{
					Endpoint:    "test-metric-endpoint",
					UseInsecure: true,
				},
			},
		})
}

func newGMPFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		gmpTypeStr,
		func() config.Exporter { return defaultGMPConfig() },
	)
}

// gmpTestExporterConfig implements exporter.Config so we can test parsing of configuration
type gmpTestExporterConfig struct {
	config.ExporterSettings           `mapstructure:",squash"`
	googlemanagedprometheus.GMPConfig `mapstructure:",squash"`
}

func defaultGMPConfig() *gmpTestExporterConfig {
	return &gmpTestExporterConfig{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(gmpTypeStr)),
		GMPConfig: googlemanagedprometheus.GMPConfig{
			UserAgent: "opentelemetry-collector-contrib {{version}}",
		},
	}
}
