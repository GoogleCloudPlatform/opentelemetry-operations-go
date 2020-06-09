// Copyright 2020, Google Inc.
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
	"go.opentelemetry.io/otel/api/metric"
	apimetric "go.opentelemetry.io/otel/api/metric"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/array"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/lastvalue"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/sum"
)

type selectorCloudMonitoring struct{}

var _ export.AggregationSelector = selectorCloudMonitoring{}

// NewWithCloudNewWithCloudMonitoringDistribution return a simple aggregation selector
// that uses lastvalue, counter, array, and aggregator for three kinds of metric.
// NOTE: this selector is used to
func NewWithCloudMonitoringDistribution() export.AggregationSelector {
	return selectorCloudMonitoring{}
}

func (selectorCloudMonitoring) AggregatorFor(descriptor *apimetric.Descriptor) export.Aggregator {
	switch descriptor.MetricKind() {
	case metric.ValueObserverKind, metric.ValueRecorderKind:
		return lastvalue.New()
	case metric.CounterKind, metric.UpDownCounterKind:
		return array.New()
	default:
		return sum.New()
	}
}
