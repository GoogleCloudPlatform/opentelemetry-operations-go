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

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	monitoringpb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"go.opentelemetry.io/otel/metric/noop"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	resourcepb "google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func TestOrderedIdentity(t *testing.T) {
	ts := &monitoringpb.TimeSeries{Metric: &metricpb.Metric{Type: "m", Labels: map[string]string{"a": "bc", "d": "e"}}, Resource: &resourcepb.MonitoredResource{Type: "r", Labels: map[string]string{"f": "g"}}}
	key := finalSeriesKey("p", ts)
	for name, mutate := range map[string]func(*monitoringpb.TimeSeries){
		"metric type":    func(s *monitoringpb.TimeSeries) { s.Metric.Type += "x" },
		"resource type":  func(s *monitoringpb.TimeSeries) { s.Resource.Type += "x" },
		"metric value":   func(s *monitoringpb.TimeSeries) { s.Metric.Labels["a"] += "x" },
		"resource value": func(s *monitoringpb.TimeSeries) { s.Resource.Labels["f"] += "x" },
		"boundary":       func(s *monitoringpb.TimeSeries) { delete(s.Metric.Labels, "a"); s.Metric.Labels["ab"] = "c" },
		"empty label":    func(s *monitoringpb.TimeSeries) { s.Metric.Labels[""] = "" },
		"nul":            func(s *monitoringpb.TimeSeries) { s.Metric.Labels["a"] = "b\x00c" },
		"label domains": func(s *monitoringpb.TimeSeries) {
			s.Metric.Labels, s.Resource.Labels = s.Resource.Labels, s.Metric.Labels
		},
	} {
		t.Run(name, func(t *testing.T) {
			s := proto.Clone(ts).(*monitoringpb.TimeSeries)
			mutate(s)
			if finalSeriesKey("p", s) == key {
				t.Fatal("distinct identities collapsed")
			}
		})
	}
	if finalSeriesKey("p2", ts) == key {
		t.Fatal("project ignored")
	}
	same := proto.Clone(ts).(*monitoringpb.TimeSeries)
	same.Metric.Labels = map[string]string{"d": "e", "a": "bc"}
	same.Points = []*monitoringpb.Point{{}}
	if finalSeriesKey("p", same) != key {
		t.Fatal("map order or points changed identity")
	}
	same.Metric.Labels = nil
	empty := proto.Clone(same).(*monitoringpb.TimeSeries)
	empty.Metric.Labels = map[string]string{}
	if finalSeriesKey("p", same) != finalSeriesKey("p", empty) {
		t.Fatal("empty label sets differ")
	}
}

// The ungated control reproduces Google's cold-descriptor failure without
// network timing. The gated path keeps first writes serial, then resumes RPC
// concurrency once schema creation has succeeded.
func TestOrderedColdSchema(t *testing.T) {
	for _, gated := range []bool{false, true} {
		t.Run(fmt.Sprint(gated), func(t *testing.T) {
			counter, _ := noop.NewMeterProvider().Meter("test").Int64Counter("points")
			var mu sync.Mutex
			initialized, creating := false, false
			var rejected, active, peak int
			me := &MetricsExporter{orderedSchemas: make(map[string]bool), pointsExportedCounter: counter}
			me.exportFunc = func(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest) error {
				mu.Lock()
				if !initialized && creating {
					rejected++
					mu.Unlock()
					return status.Error(codes.Aborted, "Errors during metric descriptor creation")
				}
				cold := !initialized
				if cold {
					creating = true
				}
				active++
				if active > peak {
					peak = active
				}
				mu.Unlock()
				time.Sleep(30 * time.Millisecond)
				mu.Lock()
				if cold {
					initialized = true
					creating = false
				}
				active--
				mu.Unlock()
				return nil
			}
			ts := &monitoringpb.TimeSeries{Metric: &metricpb.Metric{Type: "m", Labels: map[string]string{"id": "0"}}, Resource: &resourcepb.MonitoredResource{Type: "r"}}
			for wave := 0; wave < 2; wave++ {
				var wg sync.WaitGroup
				for i := 0; i < 4; i++ {
					wg.Go(func() {
						req := &monitoringpb.CreateTimeSeriesRequest{Name: "projects/fake", TimeSeries: []*monitoringpb.TimeSeries{ts}}
						if gated {
							_ = me.exportOrderedRequest(context.Background(), req)
						} else {
							_ = me.export(context.Background(), req)
						}
					})
				}
				wg.Wait()
			}
			if gated && rejected != 0 {
				t.Fatalf("cold requests raced: %d", rejected)
			}
			if !gated && rejected == 0 {
				t.Fatal("negative control did not reproduce cold race")
			}
			if peak != 4 {
				t.Fatalf("warm concurrency = %d, want 4", peak)
			}
		})
	}
}
