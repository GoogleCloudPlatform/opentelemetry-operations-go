// Copyright 2026 OpenTelemetry Authors
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

package collector

// Ordered flushes with concurrent disjoint-series RPC lanes.
// The caller MUST serialize PushMetrics through a single FIFO dispatcher. This
// does not repair late input or series shared between different collector pods.
import (
	"context"
	"encoding/binary"
	"errors"
	"hash/fnv"
	"slices"
	"strings"
	"sync"

	monitoringpb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
)

// Encode the complete identity, not just a hash: collisions must never combine
// distinct series. Length prefixes make arbitrary label bytes unambiguous;
// sorted map keys make insertion order irrelevant. Values and timestamps are
// deliberately excluded. Unlike JSON this avoids reflection and a second copy.
func finalSeriesKey(project string, ts *monitoringpb.TimeSeries) string {
	var b strings.Builder
	b.Grow(256)
	var length [binary.MaxVarintLen64]byte
	appendString := func(s string) {
		n := binary.PutUvarint(length[:], uint64(len(s)))
		b.Write(length[:n])
		b.WriteString(s)
	}
	appendString(project)
	appendString(ts.GetMetric().GetType())
	appendString(ts.GetResource().GetType())
	for _, labels := range []map[string]string{ts.GetMetric().GetLabels(), ts.GetResource().GetLabels()} {
		n := binary.PutUvarint(length[:], uint64(len(labels)))
		b.Write(length[:n])
		keys := make([]string, 0, len(labels))
		for k := range labels {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			appendString(k)
			appendString(labels[k])
		}
	}
	return b.String()
}

// schemaKey includes descriptor-defining fields but not resource/label values.
// First writes can race during Google's automatic descriptor creation/update.
// Only a successful single-writer priming request permits parallel writes for
// a schema. The bounded cache avoids repeating that work on steady-state flushes.
func schemaKey(project string, ts *monitoringpb.TimeSeries) string {
	var b strings.Builder
	b.Grow(128)
	var length [binary.MaxVarintLen64]byte
	add := func(s string) {
		n := binary.PutUvarint(length[:], uint64(len(s)))
		b.Write(length[:n])
		b.WriteString(s)
	}
	add(project)
	add(ts.GetMetric().GetType())
	add(ts.GetMetricKind().String())
	add(ts.GetValueType().String())
	add(ts.GetUnit())
	keys := make([]string, 0, len(ts.GetMetric().GetLabels()))
	for k := range ts.GetMetric().GetLabels() {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		add(k)
	}
	return b.String()
}

func validateOrderedRPCConfig(cfg MetricConfig) error {
	if cfg.OrderedRPCWorkers < 0 || cfg.OrderedRPCWorkers > 64 {
		return errors.New("ordered RPC workers must be zero (disabled) or in [1,64]")
	}
	if cfg.OrderedRPCWorkers > 0 && cfg.WALConfig != nil {
		return errors.New("ordered RPC workers cannot be combined with WAL")
	}
	return nil
}

const maxOrderedSchemas = 4096

// orderedSchemasKnown must be called with orderedSchemasMu held.
func (me *MetricsExporter) orderedSchemasKnown(keys map[string]bool) bool {
	for key := range keys {
		if !me.orderedSchemas[key] {
			return false
		}
	}
	return true
}

// rememberOrderedSchemas keeps a bounded FIFO cache. Reaching capacity evicts
// only the oldest entries, rather than making every schema cold again. An
// eviction can reduce concurrency, but cannot weaken initialization ordering.
// The caller must hold orderedSchemasMu.
func (me *MetricsExporter) rememberOrderedSchemas(keys map[string]bool) {
	if me.orderedSchemas == nil {
		me.orderedSchemas = make(map[string]bool)
	}
	for key := range keys {
		if me.orderedSchemas[key] {
			continue
		}
		if len(me.orderedSchemaOrder) < maxOrderedSchemas {
			me.orderedSchemaOrder = append(me.orderedSchemaOrder, key)
		} else {
			delete(me.orderedSchemas, me.orderedSchemaOrder[me.orderedSchemaNext])
			me.orderedSchemaOrder[me.orderedSchemaNext] = key
			me.orderedSchemaNext = (me.orderedSchemaNext + 1) % maxOrderedSchemas
		}
		me.orderedSchemas[key] = true
	}
}

// exportOrderedRequest serializes requests that may initialize an unfamiliar
// descriptor. Waiting for that gate is cancelable. Cached requests do not wait
// for unrelated schema initialization. Points are neither moved nor replayed.
func (me *MetricsExporter) exportOrderedRequest(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	keys := make(map[string]bool)
	for _, ts := range req.TimeSeries {
		keys[schemaKey(req.Name, ts)] = true
	}
	me.orderedSchemasMu.Lock()
	known := me.orderedSchemasKnown(keys)
	if me.orderedSchemaInit == nil {
		me.orderedSchemaInit = make(chan struct{}, 1)
	}
	gate := me.orderedSchemaInit
	me.orderedSchemasMu.Unlock()
	if known {
		return me.export(ctx, req)
	}
	select {
	case gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		<-gate
		return err
	}
	// Another request may have initialized the same schema while we waited.
	me.orderedSchemasMu.Lock()
	known = me.orderedSchemasKnown(keys)
	me.orderedSchemasMu.Unlock()
	if known {
		<-gate
		return me.export(ctx, req)
	}
	defer func() { <-gate }()
	err := me.export(ctx, req)
	if err == nil {
		me.orderedSchemasMu.Lock()
		me.rememberOrderedSchemas(keys)
		me.orderedSchemasMu.Unlock()
	}
	return err
}

func (me *MetricsExporter) exportOrdered(ctx context.Context, projects map[string][]*monitoringpb.TimeSeries) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	workers := me.cfg.MetricConfig.OrderedRPCWorkers
	if workers < 1 || workers > 64 {
		return errors.New("ordered RPC worker count must be in [1,64]")
	}
	if me.wal != nil {
		return errors.New("ordered RPC workers do not support WAL")
	}
	lanes := make([][]*monitoringpb.CreateTimeSeriesRequest, workers)
	for project, series := range projects {
		pending := make([][]*monitoringpb.TimeSeries, workers)
		seen := make([]map[string]bool, workers)
		flush := func(i int) {
			if len(pending[i]) == 0 {
				return
			}
			lanes[i] = append(lanes[i], &monitoringpb.CreateTimeSeriesRequest{Name: projectName(project), TimeSeries: pending[i]})
			pending[i] = nil
			seen[i] = nil
		}
		for _, ts := range series {
			if err := ctx.Err(); err != nil {
				return err
			}
			key := finalSeriesKey(project, ts)
			h := fnv.New64a()
			_, _ = h.Write([]byte(key))
			lane := int(h.Sum64() % uint64(workers))
			// Do not silently deduplicate. Repeated identities get a later request in
			// the same sequential lane, never two entries in one Google request.
			if len(pending[lane]) == sendBatchSize || seen[lane][key] {
				flush(lane)
			}
			if seen[lane] == nil {
				seen[lane] = map[string]bool{}
			}
			seen[lane][key] = true
			pending[lane] = append(pending[lane], ts)
		}
		for i := range workers {
			flush(i)
		}
	}
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := range workers {
		if len(lanes[i]) == 0 {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, req := range lanes[i] {
				if err := ctx.Err(); err != nil {
					errs[i] = errors.Join(errs[i], err)
					return
				}
				// Preserve existing partial-failure accounting and continue independent
				// requests after permanent errors, as the upstream exporter does.
				errs[i] = errors.Join(errs[i], me.exportOrderedRequest(ctx, req))
			}
		}()
	}
	wg.Wait() // Flush barrier: the next snapshot cannot overtake these requests.
	return errors.Join(errs...)
}
