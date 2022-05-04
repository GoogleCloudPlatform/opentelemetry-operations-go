// Copyright 2022 Google LLC
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

package googlemanagedprometheus

import (
	"fmt"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
)

// Config defines configuration for Google Cloud Managed Service for Prometheus exporter.
type Config struct {
	config.ExporterSettings `mapstructure:",squash"`
	GMPConfig               `mapstructure:",squash"`

	// Timeout for all API calls. If not set, defaults to 12 seconds.
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`
}

// GMPConfig is a subset of the collector config.
type GMPConfig struct {
	ProjectID    string                 `mapstructure:"project"`
	UserAgent    string                 `mapstructure:"user_agent"`
	ClientConfig collector.ClientConfig `mapstructure:",squash"`
}

func (cfg *Config) Validate() error {
	if err := cfg.ExporterSettings.Validate(); err != nil {
		return fmt.Errorf("exporter settings are invalid :%w", err)
	}
	if err := collector.ValidateConfig(cfg.toCollectorConfig()); err != nil {
		return fmt.Errorf("exporter settings are invalid :%w", err)
	}
	return nil
}
