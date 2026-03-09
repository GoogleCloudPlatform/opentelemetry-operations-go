// Copyright 2026 Google LLC
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

package metricbatcher

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// Exporter is a batching exporter.
type Exporter struct{
	exporter metric.Exporter
	size     int
}

// Option applies a configuration option value to an Exporter.
type Option interface {
	apply(*Exporter)
}

type optionFunc func(*Exporter)

func (fn optionFunc) apply(e *Exporter) {
	fn(e)
}

// WithBatchSize sets the max batch size.
func WithBatchSize(size int) Option {
	return optionFunc(func(e *Exporter) {
		e.size = size
	})
}

// New creates a new batching exporter.
func New(exporter metric.Exporter, opts ...Option) *Exporter {
	e := &Exporter{
		exporter: exporter,
		size:     200, // Size of batches we can send to Cloud Monitoring.
	}
	for _, opt := range opts {
		opt.apply(e)
	}
	return e
}

// Temporality returns the Temporality to use for an instrument kind.
func (e *Exporter) Temporality(k metric.InstrumentKind) metricdata.Temporality {
	return e.exporter.Temporality(k)
}

// Aggregation returns the Aggregation to use for an instrument kind.
func (e *Exporter) Aggregation(k metric.InstrumentKind) metric.Aggregation {
	return e.exporter.Aggregation(k)
}

// Export processes metric data.
func (e *Exporter) Export(ctx context.Context, rm *metricdata.ResourceMetrics) error {
	totalDataPoints := resourceMetricsDPC(rm)
	if totalDataPoints <= e.size || e.size == 0 {
		return e.exporter.Export(ctx, rm)
	}

	var err error
	batches := splitResourceMetrics(e.size, rm)
	for _, batch := range batches {
		if exportErr := e.exporter.Export(ctx, batch); exportErr != nil {
			err = errors.Join(err, exportErr)
		}
	}

	return err
}

// ForceFlush flushes any metric data held by an exporter.
func (e *Exporter) ForceFlush(ctx context.Context) error {
	return e.exporter.ForceFlush(ctx)
}

// Shutdown flushes all metric data held by an exporter.
func (e *Exporter) Shutdown(ctx context.Context) error {
	return e.exporter.Shutdown(ctx)
}
