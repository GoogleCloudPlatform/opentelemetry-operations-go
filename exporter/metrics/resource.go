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
	"sync"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metrics/monitoredresource/gcp"

	"go.opentelemetry.io/otel/sdk/resource"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
)

// Resource labels that are generally internal to the exporter.
// Consider exposing these labels and a type identifier in the future to allow
// for customization.
const (
	operationsProjectID = "go.opentelemetry.io/exporter/operations/project_id"
	// operationsLocation  = "go.opentelemetry.io/exporter/operations/location"
	// operationsClusterName          = "go.opentelemetry.io/exporter/operations/cluster_name"
	operationsGenericTaskNamespace = "go.opentelemetry.io/exporter/operations/generic_task/namespace"
	operationsGenericTaskJob       = "go.opentelemetry.io/exporter/operations/generic_task/job"
	operationsGenericTaskID        = "go.opentelemetry.io/exporter/operations/generic_task/task_id"
)

var (
	// autodetectFunc returns a monitored resource that is autodetected.
	// from the cloud environment at runtime.
	autodetectFunc func() gcp.Interface

	// autodetectOnce is used to lazy initialize autodetectedLabels.
	autodetectOnce *sync.Once
	// autodetectedLabels stores all the labels from the autodetected monitored resource
	// with a possible additional label for the GCP "location".
	autodetectedLabels map[string]string
)

func init() {
	autodetectFunc = gcp.Autodetect
	// monitoredresource.Autodetect only makes calls to the metadata APIs once
	// and caches the results
	autodetectOnce = new(sync.Once)
}

// getAutodetectedLabels returns all the labels from the Monitored Resource detected
// from the environment by calling monitoredresource.Autodetect. If a "zone" label is detected,
// a "location" label is added with the same value to account for differences between
// Legacy Stackdriver and Stackdriver Kubernetes Engine Monitoring,
// see https://cloud.google.com/monitoring/kubernetes-engine/migration.
func getAutodetectedLabels() map[string]string {
	autodetectOnce.Do(func() {
		autodetectedLabels = map[string]string{}
		if mr := autodetectFunc(); mr != nil {
			_, labels := mr.MonitoredResource()
			// accept "zone" value for "location" because values for location can be a zone
			// or region, see https://cloud.google.com/docs/geography-and-regions
			if _, ok := labels["zone"]; ok {
				labels["location"] = labels["zone"]
			}

			autodetectedLabels = labels
		}
	})

	return autodetectedLabels
}

func defaultMapResource(res *resource.Resource) *monitoredrespb.MonitoredResource {
	result := &monitoredrespb.MonitoredResource{
		Type: "global",
	}

	if res == nil || res.Len() == 0 {
		return result
	}

	// TODO: [ymotongpoo] add default labeles based on some attributes or labels.
	// c.f. https://github.com/census-ecosystem/opencensus-go-exporter-stackdriver/blob/v0.13.1/resource.go#L70-L134
	for _, kv := range res.Attributes() {
		result.Labels[string(kv.Key)] = kv.Value.AsString()
	}

	return result
}
