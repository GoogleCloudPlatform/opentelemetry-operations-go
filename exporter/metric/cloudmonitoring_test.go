// Copyright 2025 Google LLC
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

package metric

import (
	"fmt"
	"testing"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
)

func TestNew(t *testing.T) {
	cl := &monitoring.MetricClient{}
	tcs := []struct {
		desc               string
		opts               []Option
		verifyExporterFunc func(*metricExporter) error
	}{
		{
			desc: "WithMetricClient sets the client",
			opts: []Option{WithMonitoringClient(cl)},
			verifyExporterFunc: func(e *metricExporter) error {
				if e.client != cl {
					return fmt.Errorf("client mismatch, got = %p, want = %p", e.client, cl)
				}
				return nil
			},
		},
		{
			desc: "WithMetricClient overrides WithMetricClientOptions",
			opts: []Option{WithMonitoringClient(cl), WithMonitoringClientOptions()},
			verifyExporterFunc: func(e *metricExporter) error {
				if e.client != cl {
					return fmt.Errorf("client mismatch, got = %p, want = %p", e.client, cl)
				}
				return nil
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			opts := append(tc.opts, WithProjectID("some-project-id"))
			e, err := New(opts...)
			if err != nil {
				t.Fatalf("New(): %v", err)
			}
			exp, ok := e.(*metricExporter)
			if !ok {
				t.Fatal("unexpected type mismatch")
			}
			if err := tc.verifyExporterFunc(exp); err != nil {
				t.Fatalf("failed to verify exporter: %v", err)
			}
		})
	}
}
