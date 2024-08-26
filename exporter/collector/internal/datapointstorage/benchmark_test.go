// Copyright 2024 Google LLC
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

package datapointstorage

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

func BenchmarkIdentifier(b *testing.B) {
	metric := pmetric.NewMetric()
	metric.SetName("custom.googleapis.com/test.metric")
	attrs := pcommon.NewMap()
	attrs.PutStr("string", "strval")
	attrs.PutBool("bool", true)
	attrs.PutInt("int", 123)
	attrs.PutInt("int1", 123)
	attrs.PutInt("int2", 123)
	attrs.PutInt("int3", 123)
	attrs.PutInt("int4", 123)
	attrs.PutInt("int5", 123)
	monitoredResource := &monitoredrespb.MonitoredResource{
		Type: "k8s_container",
		Labels: map[string]string{
			"location":  "us-central1-b",
			"project":   "project-foo",
			"cluster":   "cluster-foo",
			"pod":       "pod-foo",
			"namespace": "namespace-foo",
			"container": "container-foo",
		},
	}
	extraLabels := map[string]string{
		"foo":   "bar",
		"hello": "world",
		"ding":  "dong",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Identifier(monitoredResource, extraLabels, metric, attrs)
	}
}
