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
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/api/core"
	"go.opentelemetry.io/otel/api/label"
	export "go.opentelemetry.io/otel/sdk/export/metric"
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

// checkpointSet transforms all records contained in a checkpoint into
// monitoringpb.TimeSeries in batch.
func (me *metricsExporter) checkPointSet(ctx context.Context, cps export.CheckpointSet, numWorkers uint) ([]*monitoringpb.TimeSeries, error) {
	records, errch := source(ctx, cps)

	// Start workers to transform records.
	transformed := make(chan result)
	var wg sync.WaitGroup
	wg.Add(int(numWorkers))
	for i := uint(0); i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			transformer(ctx, records, transformed)
		}()
	}
	go func() {
		wg.Wait()
		close(transformed)
	}()

	tss, err := sink(ctx, transformed)
	if err != nil {
		return nil, err
	}

	if err := <-errch; err != nil {
		return nil, err
	}
	return tss, nil
}

// source starts a goroutine that sends each one of the Records yielded by
// the CheckpointSet on the returned chan. Any error encoutered will be sent
// on the returned error chan after seeding is complete.
func source(ctx context.Context, cps export.CheckpointSet) (<-chan export.Record, <-chan error) {
	errch := make(chan error, 1)
	out := make(chan export.Record)
	go func() {
		defer close(out)
		errch <- cps.ForEach(func(r export.Record) error {
			select {
			case <-ctx.Done():
				return errContextCanceled
			case out <- r:
			}
			return nil
		})
	}()
	return out, errch
}

// transformer transforms records read from the passed in chan into
// monitoringpb.TimeSeries which are sent on the out chan.
func transformer(ctx context.Context, in <-chan export.Record, out chan<- result) {
	for r := range in {
		ts, err := recordToTs(r)
		if err == nil && ts == nil {
			continue
		}
		res := result{
			Resource:   r.Descriptor().Resource(),
			Library:    r.Descriptor().LibraryName(),
			TimeSeries: ts,
			Err:        err,
		}
		select {
		case <-ctx.Done():
			return
		case out <- res:
		}
	}
}

func sink(ctx context.Context, in <-chan result) ([]*monitoringpb.TimeSeries, error) {
	var errStrings []string

	grouped := make(map[label.Distinct][]*monitoringpb.TimeSeries)
	for res := range in {
		if res.Err != nil {
			errStrings = append(errStrings, res.Err.Error())
			continue
		}

		rID := res.Resource.Equivalent()

	}
	return nil, nil
}

func recordToTs(r export.Record) (*monitoringpb.TimeSeries, error) {
	// TODO: [ymotongpoo] implement here
	return nil, nil
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
