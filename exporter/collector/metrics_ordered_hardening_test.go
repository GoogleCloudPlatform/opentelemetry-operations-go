// Copyright 2026 OpenTelemetry Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collector

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	monitoringpb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"go.opentelemetry.io/otel/metric/noop"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	resourcepb "google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func orderedTestExporter() *MetricsExporter {
	counter, _ := noop.NewMeterProvider().Meter("test").Int64Counter("points")
	cfg := DefaultConfig()
	cfg.MetricConfig.OrderedRPCWorkers = 4
	return &MetricsExporter{cfg: cfg, orderedSchemas: make(map[string]bool), pointsExportedCounter: counter}
}
func orderedTestSeries(name, id string) *monitoringpb.TimeSeries {
	return &monitoringpb.TimeSeries{Metric: &metricpb.Metric{Type: name, Labels: map[string]string{"id": id}}, Resource: &resourcepb.MonitoredResource{Type: "prometheus_target"}}
}

func TestOrderedColdWaitCancellation(t *testing.T) {
	me := orderedTestExporter()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	me.exportFunc = func(context.Context, *monitoringpb.CreateTimeSeriesRequest) error {
		once.Do(func() { close(entered) })
		<-release
		return nil
	}
	req := &monitoringpb.CreateTimeSeriesRequest{Name: "projects/test", TimeSeries: []*monitoringpb.TimeSeries{orderedTestSeries("m", "0")}}
	first := make(chan error, 1)
	go func() { first <- me.exportOrderedRequest(context.Background(), req) }()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	second := make(chan error, 1)
	go func() { second <- me.exportOrderedRequest(ctx, req) }()
	// Cancel while a different request holds the cold-schema initialization gate.
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case err := <-second:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("got %v, want cancellation", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("canceled request waited for unrelated RPC")
		close(release)
		<-first
		<-second
		return
	}
	close(release)
	<-first
}

func TestOrderedCancellationStopsRemainingRequests(t *testing.T) {
	me := orderedTestExporter()
	started := make(chan struct{})
	var once sync.Once
	var calls atomic.Int64
	me.exportFunc = func(ctx context.Context, _ *monitoringpb.CreateTimeSeriesRequest) error {
		calls.Add(1)
		once.Do(func() { close(started) })
		<-ctx.Done()
		return ctx.Err()
	}
	series := make([]*monitoringpb.TimeSeries, 1000)
	for i := range series {
		series[i] = orderedTestSeries("m", fmt.Sprint(i))
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- me.exportOrdered(ctx, map[string][]*monitoringpb.TimeSeries{"test": series}) }()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled flush did not finish")
	}
	if calls.Load() != 1 {
		t.Errorf("started %d RPCs, want only the already-running RPC", calls.Load())
	}
}

func TestOrderedSchemaCacheRetainsWarmConcurrency(t *testing.T) {
	me := orderedTestExporter()
	var active, peak atomic.Int64
	me.exportFunc = func(context.Context, *monitoringpb.CreateTimeSeriesRequest) error {
		n := active.Add(1)
		for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
		}
		time.Sleep(2 * time.Millisecond)
		active.Add(-1)
		return nil
	}
	for _, generation := range []string{"initial", "replacement"} {
		series := make([]*monitoringpb.TimeSeries, 4100)
		for i := range series {
			series[i] = orderedTestSeries(fmt.Sprintf("metric_%s_%d", generation, i), "0")
		}
		for pass := 0; pass < 2; pass++ {
			peak.Store(0)
			if err := me.exportOrdered(context.Background(), map[string][]*monitoringpb.TimeSeries{"test": series}); err != nil {
				t.Fatal(err)
			}
			if len(me.orderedSchemas) > 4096 {
				t.Fatalf("schema cache unbounded: %d", len(me.orderedSchemas))
			}
			if pass == 1 && peak.Load() < 2 {
				t.Errorf("%s warm pass serialized every RPC at cache capacity", generation)
			}
		}
	}
}

func TestOrderedConfigValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		workers int
		wal     bool
		invalid bool
	}{
		{name: "disabled"}, {name: "one", workers: 1}, {name: "four", workers: 4}, {name: "maximum", workers: 64},
		{name: "negative", workers: -1, invalid: true}, {name: "too many", workers: 65, invalid: true},
		{name: "WAL remains allowed when disabled", wal: true}, {name: "ordered WAL rejected", workers: 4, wal: true, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.MetricConfig.OrderedRPCWorkers = tc.workers
			if tc.wal {
				cfg.MetricConfig.WALConfig = &WALConfig{}
			}
			err := ValidateConfig(cfg)
			if (err != nil) != tc.invalid {
				t.Fatalf("validation = %v, want invalid=%v", err, tc.invalid)
			}
		})
	}
}

func TestOrderedRequestsPreserveSeriesOrder(t *testing.T) {
	me := orderedTestExporter()
	var mu sync.Mutex
	latest := map[string]int64{}
	points := 0
	me.exportFunc = func(_ context.Context, req *monitoringpb.CreateTimeSeriesRequest) error {
		mu.Lock()
		defer mu.Unlock()
		if len(req.TimeSeries) > 200 {
			t.Errorf("request has %d series", len(req.TimeSeries))
		}
		seen := map[string]bool{}
		for _, ts := range req.TimeSeries {
			key := finalSeriesKey(req.Name, ts)
			if seen[key] {
				t.Error("duplicate final identity in a request")
			}
			seen[key] = true
			stamp := ts.Points[0].Interval.EndTime.Seconds
			if stamp <= latest[key] {
				t.Errorf("timestamp %d overtook %d", latest[key], stamp)
			}
			latest[key] = stamp
			points++
		}
		return nil
	}
	for flush := 0; flush < 2; flush++ {
		series := make([]*monitoringpb.TimeSeries, 0, 1802)
		// Two samples per identity in one flush must become sequential requests.
		for sample := 0; sample < 2; sample++ {
			for i := 0; i < 901; i++ {
				ts := orderedTestSeries("m", fmt.Sprint(i))
				ts.Points = []*monitoringpb.Point{{Interval: &monitoringpb.TimeInterval{EndTime: timestamppb.New(time.Unix(int64(1+flush*2+sample), 0))}}}
				series = append(series, ts)
			}
		}
		if err := me.exportOrdered(context.Background(), map[string][]*monitoringpb.TimeSeries{"test": series}); err != nil {
			t.Fatal(err)
		}
	}
	if points != 3604 || len(latest) != 901 {
		t.Fatalf("points=%d series=%d", points, len(latest))
	}
	for _, stamp := range latest {
		if stamp != 4 {
			t.Fatalf("latest timestamp %d, want 4", stamp)
		}
	}
}
