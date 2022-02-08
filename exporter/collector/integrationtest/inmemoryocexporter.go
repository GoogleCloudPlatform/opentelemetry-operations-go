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
	"time"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/metric/metricexport"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
)

// OC stats/metrics exporter used to capture self observability metrics
type InMemoryOCExporter struct {
	testServer          *MetricsTestServer
	reader              *metricexport.Reader
	stackdriverExporter *stackdriver.Exporter
}

func getViews() []*view.View {
	views := []*view.View{}
	views = append(views, collector.MetricViews()...)
	views = append(views, ocgrpc.DefaultClientViews...)
	return views
}

func (i *InMemoryOCExporter) Proto(ctx context.Context) (*SelfObservabilityMetric, error) {
	// Hack to flush stats, see https://tinyurl.com/5hfcxzk2
	view.SetReportingPeriod(time.Minute)
	i.reader.ReadAndExport(i.stackdriverExporter)
	ctx, cancel := context.WithTimeout(ctx, time.Millisecond*500)
	defer cancel()

	done := make(chan struct{}, 1)
	go func() {
		i.stackdriverExporter.Flush()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
	}

	return &SelfObservabilityMetric{
			CreateTimeSeriesRequests:       i.testServer.CreateTimeSeriesRequests(),
			CreateMetricDescriptorRequests: i.testServer.CreateMetricDescriptorRequests(),
		},
		nil
}

// Shutdown unregisters the global OpenCensus views to reset state for the next test
func (i *InMemoryOCExporter) Shutdown(ctx context.Context) error {
	i.stackdriverExporter.StopMetricsExporter()
	err := i.stackdriverExporter.Close()
	i.testServer.Shutdown()

	view.Unregister(getViews()...)
	return err
}

// NewInMemoryOCViewExporter creates a new in memory OC exporter for testing. Be sure to defer
// a call to Shutdown().
func NewInMemoryOCViewExporter() (*InMemoryOCExporter, error) {
	// Reset our views in case any tests ran before this
	views := getViews()
	view.Unregister(views...)
	view.Register(views...)

	testServer, err := NewMetricTestServer()
	if err != nil {
		return nil, err
	}
	go testServer.Serve()
	conn, err := grpc.Dial(testServer.Endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	clientOpts := []option.ClientOption{option.WithGRPCConn(conn)}

	stackdriverExporter, err := stackdriver.NewExporter(stackdriver.Options{
		DefaultMonitoringLabels: &stackdriver.Labels{},
		ProjectID:               "myproject",
		MonitoringClientOptions: clientOpts,
		TraceClientOptions:      clientOpts,
	})
	if err != nil {
		return nil, err
	}

	return &InMemoryOCExporter{
			testServer:          testServer,
			stackdriverExporter: stackdriverExporter,
			reader:              metricexport.NewReader(),
		},
		nil
}
