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
	"fmt"
	"sort"
	"time"

	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/metric/metricexport"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
)

var _ metricexport.Exporter = (*InMemoryOCExporter)(nil)

// OC stats/metrics exporter used to capture self observability metrics
type InMemoryOCExporter struct {
	c      chan []*metricdata.Metric
	reader *metricexport.Reader
}

func (i *InMemoryOCExporter) ExportMetrics(ctx context.Context, data []*metricdata.Metric) error {
	i.c <- data
	return nil
}

func (i *InMemoryOCExporter) Proto() []*SelfObservabilityMetric {
	// Hack to flush stats, see https://tinyurl.com/5hfcxzk2
	view.SetReportingPeriod(time.Minute)
	i.reader.ReadAndExport(i)
	var data []*metricdata.Metric

	select {
	case <-time.NewTimer(time.Millisecond * 100).C:
		return nil
	case data = <-i.c:
	}

	selfObsMetrics := []*SelfObservabilityMetric{}
	for _, d := range data {
		for _, ts := range d.TimeSeries {
			labels := make(map[string]string, len(d.Descriptor.LabelKeys))
			for i := 0; i < len(d.Descriptor.LabelKeys); i++ {
				labels[d.Descriptor.LabelKeys[i].Key] = ts.LabelValues[i].Value
			}

			for _, p := range ts.Points {
				selfObsMetrics = append(selfObsMetrics, &SelfObservabilityMetric{
					Name:   d.Descriptor.Name,
					Val:    fmt.Sprint(p.Value),
					Labels: labels,
				})
			}
		}
	}
	sort.Slice(selfObsMetrics, func(i, j int) bool {
		return selfObsMetrics[i].Name < selfObsMetrics[j].Name
	})
	return selfObsMetrics
}

// Shutdown unregisters the global OpenCensus views to reset state for the next test
func (i *InMemoryOCExporter) Shutdown(ctx context.Context) error {
	view.Unregister(collector.MetricViews()...)
	view.Unregister(ocgrpc.DefaultClientViews...)
	return nil
}

// NewInMemoryOCViewExporter creates a new in memory OC exporter for testing. Be sure to defer
// a call to Shutdown().
func NewInMemoryOCViewExporter() (*InMemoryOCExporter, error) {
	// Reset our views in case any tests ran before this
	view.Unregister(collector.MetricViews()...)
	view.Register(collector.MetricViews()...)
	view.Unregister(ocgrpc.DefaultClientViews...)
	// TODO: Register ocgrpc.DefaultClientViews to test them

	return &InMemoryOCExporter{
			c:      make(chan []*metricdata.Metric, 1),
			reader: metricexport.NewReader(),
		},
		nil
}
