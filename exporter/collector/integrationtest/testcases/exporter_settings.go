// Copyright 2025 Google LLC
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

package testcases

import (
	"fmt"
	"runtime"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
)

func NewTestExporterSettings(logger *zap.Logger, meterProvider metric.MeterProvider) exporter.Settings {
	return exporter.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger:        logger,
			MeterProvider: meterProvider,
		},
		BuildInfo: component.BuildInfo{
			Description: "GoogleCloudExporter Integration Test",
			Version:     collector.Version(),
		},
	}
}

func SetTestUserAgent(cfg *collector.Config, buildInfo component.BuildInfo) {
	collector.SetUserAgent(cfg, buildInfo)
	cfg.UserAgent = UserAgentRemoveRuntimeInfo(cfg.UserAgent)
}

func UserAgentRemoveRuntimeInfo(userAgent string) string {
	runtimeInfo := fmt.Sprintf(" (%s/%s)", runtime.GOOS, runtime.GOARCH)
	return strings.ReplaceAll(userAgent, runtimeInfo, "")
}
