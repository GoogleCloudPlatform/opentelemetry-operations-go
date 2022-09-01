// Copyright 2021 Google LLC
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

package integrationtest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock"
)

// NewMetricsTestExporter creates and starts a googlecloud exporter by updating the
// given cfg copy to point to the test server.
func NewMetricTestExporter(
	ctx context.Context,
	t testing.TB,
	m *cloudmock.MetricsTestServer,
	cfg collector.Config,
) *collector.MetricsExporter {
	cfg.MetricConfig.ClientConfig.Endpoint = m.Endpoint
	cfg.MetricConfig.ClientConfig.UseInsecure = true

	exporter, err := collector.NewGoogleCloudMetricsExporter(
		ctx,
		cfg,
		zap.NewNop(),
		"latest",
		collector.DefaultTimeout,
	)
	require.NoError(t, err)
	t.Logf("Collector MetricsTestServer exporter started, pointing at %v", cfg.MetricConfig.ClientConfig.Endpoint)
	return exporter
}
