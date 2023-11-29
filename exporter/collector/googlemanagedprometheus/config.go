// Copyright 2023 Google LLC
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

// Config provides configuration options specific to the GMP translation.
// It is meant to be embedded in the googlemanagedprometheus configuration.
type Config struct {
	// AddMetricSuffixes controls whether suffixes are added to metric names. Defaults to true.
	AddMetricSuffixes bool `mapstructure:"add_metric_suffixes"`
	// ExtraMetricsConfig configures the target_info and otel_scope_info metrics.
	ExtraMetricsConfig ExtraMetricsConfig `mapstructure:"extra_metrics_config"`
}

// ExtraMetricsConfig controls the inclusion of additional metrics.
type ExtraMetricsConfig struct {
	// Add `target_info` metric based on the resource. On by default.
	EnableTargetInfo bool `mapstructure:"enable_target_info"`
	// Add `otel_scope_info` metric and `scope_name`/`scope_version` attributes to all other metrics. On by default.
	EnableScopeInfo bool `mapstructure:"enable_scope_info"`
}

func DefaultConfig() Config {
	return Config{
		AddMetricSuffixes: true,
		ExtraMetricsConfig: ExtraMetricsConfig{
			EnableTargetInfo: true,
			EnableScopeInfo:  true,
		},
	}
}

// Validate checks if the exporter configuration is valid.
func (cfg *Config) Validate() error {
	return nil
}
