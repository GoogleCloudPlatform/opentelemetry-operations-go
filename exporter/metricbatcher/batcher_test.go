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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type mockExporter struct {
	exportedMetrics []*metricdata.ResourceMetrics
	failOnBatch     int
	batchNum        int
}

func (m *mockExporter) Temporality(k metric.InstrumentKind) metricdata.Temporality {
	return metricdata.CumulativeTemporality
}

func (m *mockExporter) Aggregation(k metric.InstrumentKind) metric.Aggregation {
	return metric.DefaultAggregationSelector(k)
}

func (m *mockExporter) Export(ctx context.Context, rm *metricdata.ResourceMetrics) error {
	m.batchNum++
	if m.failOnBatch > 0 && m.batchNum == m.failOnBatch {
		return assert.AnError
	}
	m.exportedMetrics = append(m.exportedMetrics, rm)
	return nil
}

func (m *mockExporter) ForceFlush(ctx context.Context) error {
	return nil
}

func (m *mockExporter) Shutdown(ctx context.Context) error {
	return nil
}

func TestBatchingExporter(t *testing.T) {
	mock := &mockExporter{}
	exporter := New(mock, WithBatchSize(2))

	ctx := context.Background()

	rm := &metricdata.ResourceMetrics{
		ScopeMetrics: []metricdata.ScopeMetrics{
			{
				Metrics: []metricdata.Metrics{
					{
						Name: "metric1",
						Data: metricdata.Gauge[int64]{
							DataPoints: []metricdata.DataPoint[int64]{
								{Value: 1},
								{Value: 2},
								{Value: 3},
								{Value: 4},
								{Value: 5},
							},
						},
					},
				},
			},
		},
	}

	err := exporter.Export(ctx, rm)
	require.NoError(t, err)

	assert.Len(t, mock.exportedMetrics, 3)

	// First batch should have 2 points
	assert.Equal(t, 2, resourceMetricsDPC(mock.exportedMetrics[0]))
	// Second batch should have 2 points
	assert.Equal(t, 2, resourceMetricsDPC(mock.exportedMetrics[1]))
	// Third batch should have 1 point
	assert.Equal(t, 1, resourceMetricsDPC(mock.exportedMetrics[2]))
}

func TestBatchingExporter_PartialFailure(t *testing.T) {
	mock := &mockExporter{failOnBatch: 2} // fail on the second batch
	exporter := New(mock, WithBatchSize(2))

	ctx := context.Background()

	rm := &metricdata.ResourceMetrics{
		ScopeMetrics: []metricdata.ScopeMetrics{
			{
				Metrics: []metricdata.Metrics{
					{
						Name: "metric1",
						Data: metricdata.Gauge[int64]{
							DataPoints: []metricdata.DataPoint[int64]{
								{Value: 1},
								{Value: 2},
								{Value: 3},
								{Value: 4},
								{Value: 5},
							},
						},
					},
				},
			},
		},
	}

	err := exporter.Export(ctx, rm)
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)

	// Since batch 2 failed, only batch 1 and 3 should have been recorded.
	assert.Len(t, mock.exportedMetrics, 2)

	// First batch should have 2 points
	assert.Equal(t, 2, resourceMetricsDPC(mock.exportedMetrics[0]))
	// Third batch should have 1 point
	assert.Equal(t, 1, resourceMetricsDPC(mock.exportedMetrics[1]))
}
