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
