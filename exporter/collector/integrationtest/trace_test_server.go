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

package integrationtest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock"
)

func NewTraceTestExporter(
	ctx context.Context,
	t testing.TB,
	s *cloudmock.TracesTestServer,
	cfg collector.Config,
) *collector.TraceExporter {

	cfg.TraceConfig.ClientConfig.Endpoint = s.Endpoint
	cfg.TraceConfig.ClientConfig.UseInsecure = true
	cfg.ProjectID = "fakeprojectid"

	exporter, err := collector.NewGoogleCloudTracesExporter(
		ctx,
		cfg,
		"latest",
		collector.DefaultTimeout,
	)
	require.NoError(t, err)
	t.Logf("Collector TracesTestServer exporter started, pointing at %v", cfg.TraceConfig.ClientConfig.Endpoint)
	return exporter
}
