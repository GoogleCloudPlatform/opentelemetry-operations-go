// Copyright 2022 Google LLC
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

package gcp

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

func (d *detector) onGKE() bool {
	_, found := d.os.LookupEnv("KUBERNETES_SERVICE_HOST")
	return found
}

// gkeAttributes detects resource attributes available via the GKE Metadata server:
// https://cloud.google.com/kubernetes-engine/docs/concepts/workload-identity#instance_attributes
// This includes: cluster name, zone or region, instance ID, and instance name
func (d *detector) gkeAttributes() ([]attribute.KeyValue, []string) {
	attributes, errs := d.commonGCEAttributes()
	if clusterName, err := d.metadata.InstanceAttributeValue("cluster-name"); err != nil {
		errs = append(errs, err.Error())
	} else if clusterName != "" {
		attributes = append(attributes, semconv.K8SClusterNameKey.String(clusterName))
	}
	if clusterLocation, err := d.metadata.InstanceAttributeValue("cluster-location"); err != nil {
		errs = append(errs, err.Error())
	} else if clusterLocation != "" {
		switch strings.Count(clusterLocation, "-") {
		case 1:
			attributes = append(attributes, semconv.CloudRegionKey.String(clusterLocation))
		case 2:
			attributes = append(attributes, semconv.CloudAvailabilityZoneKey.String(clusterLocation))
		default:
			errs = append(errs, fmt.Sprintf("unrecognized format for cluster location: %v", clusterLocation))
		}
	}
	return attributes, errs
}
