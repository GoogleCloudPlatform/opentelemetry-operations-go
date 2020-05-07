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

package metrics

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"go.opentelemetry.io/otel/api/core"
	"go.opentelemetry.io/otel/sdk/resource"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

var domains = []string{"googleapis.com", "kubernetes.io", "istio.io", "knative.dev"}

var errNilMetricOrMetricDescriptor = errors.New("non-nil metric or metric descriptor")

func (me *metricsExporter) metricTypeFromProto(name string) string {
	prefix := me.o.MetricPrefix
	if me.o.GetMetricPrefix != nil {
		prefix = me.o.GetMetricPrefix(name)
	}
	if prefix != "" {
		name = path.Join(prefix, name)
	}
	if !hasDomain(name) {
		// Still needed because the name may or may not have a "/" at the beginning.
		name = path.Join(defaultDomain, name)
	}
	return name
}

// hasDomain checks if the metric name already has a domain in it.
func hasDomain(name string) bool {
	for _, domain := range domains {
		if strings.Contains(name, domain) {
			return true
		}
	}
	return false
}

// result is the product of transforming Records into monitoringpb.TimeSeries.
type result struct {
	Resource   resource.Resource
	Library    string
	TimeSeries *monitoringpb.TimeSeries
	Err        error
}

func aggValueToMpbTypeValue(v core.Number, kind core.NumberKind) (*monitoringpb.TypedValue, error) {
	var err error
	var tval *monitoringpb.TypedValue
	switch kind {
	case core.Int64NumberKind:
		tval = &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_Int64Value{
				Int64Value: v.AsInt64(),
			},
		}
	case core.Float64NumberKind:
		tval = &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_DoubleValue{
				DoubleValue: v.AsFloat64(),
			},
		}
	case core.Uint64NumberKind:
		tval = &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_Int64Value{
				Int64Value: v.AsInt64(),
			},
		}
	default:
		err = fmt.Errorf("Unexpected type: %T", kind)
	}
	return tval, err
}
