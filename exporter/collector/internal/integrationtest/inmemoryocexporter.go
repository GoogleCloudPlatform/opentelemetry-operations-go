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
	"log"
	"sort"
	"time"

	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/metric/metricexport"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
)

var (
	ocTypeToMetricKind = map[metricdata.Type]SelfObservabilityMetric_MetricKind{
		metricdata.TypeCumulativeInt64:        SelfObservabilityMetric_CUMULATIVE,
		metricdata.TypeCumulativeFloat64:      SelfObservabilityMetric_CUMULATIVE,
		metricdata.TypeGaugeInt64:             SelfObservabilityMetric_GAUGE,
		metricdata.TypeGaugeFloat64:           SelfObservabilityMetric_GAUGE,
		metricdata.TypeCumulativeDistribution: SelfObservabilityMetric_CUMULATIVE,
	}
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

func (i *InMemoryOCExporter) Proto(ctx context.Context) ([]*SelfObservabilityMetric, error) {
	// Hack to flush stats, see https://tinyurl.com/5hfcxzk2
	view.SetReportingPeriod(time.Minute)
	i.reader.ReadAndExport(i)
	var data []*metricdata.Metric

	ctx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case data = <-i.c:
	}

	selfObsMetrics := []*SelfObservabilityMetric{}
	for _, d := range data {
		for _, ts := range d.TimeSeries {
			labels := make(map[string]string, len(d.Descriptor.LabelKeys))
			for i := 0; i < len(d.Descriptor.LabelKeys); i++ {
				labels[d.Descriptor.LabelKeys[i].Key] = ts.LabelValues[i].Value
			}
			metricKind, ok := ocTypeToMetricKind[d.Descriptor.Type]
			if !ok {
				return nil, fmt.Errorf("Got unsupported OC metric type %v", d.Descriptor.Type)
			}

			for _, p := range ts.Points {
				selfObsMetric := &SelfObservabilityMetric{
					Name:       d.Descriptor.Name,
					MetricKind: metricKind,
					Labels:     labels,
				}

				switch value := p.Value.(type) {
				case int64:
					selfObsMetric.Value = &SelfObservabilityMetric_Int64Value{Int64Value: value}
				case float64:
					selfObsMetric.Value = &SelfObservabilityMetric_Float64Value{Float64Value: value}
				case *metricdata.Distribution:
					bucketCounts := []int64{}
					for _, bucket := range value.Buckets {
						bucketCounts = append(bucketCounts, bucket.Count)
					}
					selfObsMetric.Value = &SelfObservabilityMetric_HistogramValue{
						HistogramValue: &SelfObservabilityMetric_Histogram{
							BucketBounds: value.BucketOptions.Bounds,
							BucketCounts: bucketCounts,
							Count:        value.Count,
							Sum:          value.Sum,
						},
					}
				default:
					// Probably don't care about any others so leaving them out for now
					log.Printf("Can't handle OpenCensus metric data type %v, update the code", d.Descriptor.Type)
				}

				selfObsMetrics = append(selfObsMetrics, selfObsMetric)
			}
		}
	}
	sort.Slice(selfObsMetrics, func(i, j int) bool {
		if selfObsMetrics[i].Name == selfObsMetrics[j].Name {
			return fmt.Sprint(selfObsMetrics[i].Labels) < fmt.Sprint(selfObsMetrics[j].Labels)
		}
		return selfObsMetrics[i].Name < selfObsMetrics[j].Name
	})
	return selfObsMetrics, nil
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
	view.Unregister(ocgrpc.DefaultClientViews...)
	view.Register(collector.MetricViews()...)
	view.Register(ocgrpc.DefaultClientViews...)

	return &InMemoryOCExporter{
			c:      make(chan []*metricdata.Metric, 1),
			reader: metricexport.NewReader(),
		},
		nil
}
