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
	"context"
	"fmt"
	"reflect"
	"testing"

	"go.opentelemetry.io/otel/api/kv"
	"go.opentelemetry.io/otel/api/metric"
	apimetric "go.opentelemetry.io/otel/api/metric"
	"go.opentelemetry.io/otel/exporters/metric/test"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/lastvalue"
	aggtest "go.opentelemetry.io/otel/sdk/metric/aggregator/test"
	"go.opentelemetry.io/otel/sdk/resource"

	googlemetricpb "google.golang.org/genproto/googleapis/api/metric"
)

var (
	formatter = func(d *apimetric.Descriptor) string {
		return fmt.Sprintf("test.googleapis.com/%s", d.Name())
	}
)

func TestDescToMetricType(t *testing.T) {
	inMe := []*metricExporter{
		{
			o: &options{},
		},
		{
			o: &options{
				MetricDescriptorTypeFormatter: formatter,
			},
		},
	}

	inDesc := []apimetric.Descriptor{
		apimetric.NewDescriptor("testing", apimetric.ValueRecorderKind, apimetric.Float64NumberKind),
		apimetric.NewDescriptor("test/of/path", apimetric.ValueRecorderKind, apimetric.Float64NumberKind),
	}

	wants := []string{
		"custom.googleapis.com/opentelemetry/testing",
		"test.googleapis.com/test/of/path",
	}

	for i, w := range wants {
		out := inMe[i].descToMetricType(&inDesc[i])
		if out != w {
			t.Errorf("expected: %v, actual: %v", w, out)
		}
	}
}

func TestRecordToMpb(t *testing.T) {
	res := &resource.Resource{}
	cps := test.NewCheckpointSet(res)
	ctx := context.Background()

	desc := apimetric.NewDescriptor("testing", apimetric.ValueRecorderKind, apimetric.Float64NumberKind)

	lvagg := lastvalue.New()
	aggtest.CheckedUpdate(t, lvagg, metric.NewFloat64Number(12.34), &desc)
	lvagg.Checkpoint(ctx, &desc)
	cps.Add(&desc, lvagg, kv.String("a", "A"), kv.String("b", "B"))

	md := &googlemetricpb.MetricDescriptor{
		Name:        desc.Name(),
		Type:        fmt.Sprintf(cloudMonitoringMetricDescriptorNameFormat, desc.Name()),
		MetricKind:  googlemetricpb.MetricDescriptor_GAUGE,
		ValueType:   googlemetricpb.MetricDescriptor_DOUBLE,
		Description: "test",
	}

	mdkey := key{
		name:        md.Name,
		libraryname: "",
	}
	me := &metricExporter{
		o: &options{},
		mdCache: map[key]*googlemetricpb.MetricDescriptor{
			mdkey: md,
		},
	}

	want := &googlemetricpb.Metric{
		Type: md.Type,
		Labels: map[string]string{
			"a": "A",
			"b": "B",
		},
	}

	aggError := cps.ForEach(func(r export.Record) error {
		out := me.recordToMpb(&r)
		if !reflect.DeepEqual(want, out) {
			return fmt.Errorf("expected: %v, actual: %v", want, out)
		}
		return nil
	})
	if aggError != nil {
		t.Errorf("%v", aggError)
	}
}


func TestResourceToMonitoredResourcepb(t *testing.T) {
	resList := []*resource.Resource{
	// correct k8s resources
	resource.New(
		kv.String("type", "k8s"),
		kv.String("cloud.zone", "us-central1-a"),
		kv.String("k8s.cluster.name", "opentelemetry-cluster"),
		kv.String("k8s.namespace.name", "default"),
		kv.String("k8s.pod.name", "opentelemetry-pod-autoconf"),
		kv.String("k8s.deployment.name", "opentelemetry"),		
	), 
	// nonexisting resource types
	resource.New(
		kv.String("type", "none"),
		kv.String("cloud.zone", "us-central1-a"),
		kv.String("k8s.cluster.name", "opentelemetry-cluster"),
		kv.String("k8s.namespace.name", "default"),
		kv.String("k8s.pod.name", "opentelemetry-pod-autoconf"),
		kv.String("k8s.deployment.name", "opentelemetry"),		
	),
	// missing resource fields
	resource.New(
		kv.String("type", "k8s"),
		kv.String("cloud.zone", "us-central1-a"),
		kv.String("k8s.cluster.name", "opentelemetry-cluster"),
		kv.String("k8s.namespace.name", "default"),
		kv.String("k8s.pod.name", "opentelemetry-pod-autoconf"),
	)}

	expectedTypes := []string{"k8s_container","global", "global"}

	expectedLabels := []map[string]string {
		map[string]string{
			"project_id":     "",
			"location": "us-central1-a",
			"cluster_name": "opentelemetry-cluster",
			"namespace_name": "default",
			"pod_name": "opentelemetry-pod-autoconf",
			"container_name": "opentelemetry",
		},
		map[string]string{
			"project_id":     "",
		},
		map[string]string{
			"project_id":     "",
		},
	}

	desc := apimetric.NewDescriptor("testing", apimetric.ValueRecorderKind, apimetric.Float64NumberKind)


	
	md := &googlemetricpb.MetricDescriptor{
		Name:        desc.Name(),
		Type:        fmt.Sprintf(cloudMonitoringMetricDescriptorNameFormat, desc.Name()),
		MetricKind:  googlemetricpb.MetricDescriptor_GAUGE,
		ValueType:   googlemetricpb.MetricDescriptor_DOUBLE,
		Description: "test",
	}	

	mdkey := key{
		name:        md.Name,
		libraryname: "",
	}	

	me := &metricExporter{
		o: &options{},
		mdCache: map[key]*googlemetricpb.MetricDescriptor{
			mdkey: md,
		},
	}

	for index := range resList {
		outputMonitoredRes := me.resourceToMonitoredResourcepb(resList[index])
		if !reflect.DeepEqual(outputMonitoredRes.GetLabels(), expectedLabels[index]) {
			t.Errorf("expected: %v, actual: %v", expectedLabels[index], outputMonitoredRes.GetLabels())
		}
		if outputMonitoredRes.GetType() != expectedTypes[index] {
			t.Errorf("expected: %v, actual: %v", expectedTypes[index], outputMonitoredRes.GetType())
		}		
	}
}