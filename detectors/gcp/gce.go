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
	"strings"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

// gceAttributes are attributes that are available from the
// GCE metadata server: https://cloud.google.com/compute/docs/metadata/default-metadata-values#vm_instance_metadata
// This includes: machine type, zone, region, instance ID, and instance name
func (d *detector) gceAttributes() ([]attribute.KeyValue, []string) {
	attributes, errs := d.commonGCEAttributes()
	zoneAttributes, zoneErrs := d.zoneAndRegionAttributes()
	attributes = append(attributes, zoneAttributes...)
	errs = append(errs, zoneErrs...)
	if hostType, err := d.metadata.Get("instance/machine-type"); err != nil {
		errs = append(errs, err.Error())
	} else if hostType != "" {
		attributes = append(attributes, semconv.HostTypeKey.String(hostType))
	}
	return attributes, errs
}

// commonGCEAttributes are attributes that are available from the
// GCE metadata server: https://cloud.google.com/compute/docs/metadata/default-metadata-values#vm_instance_metadata
// and the GKE metadata server: https://cloud.google.com/kubernetes-engine/docs/concepts/workload-identity#instance_metadata
// This includes: instance ID, and instance name
func (d *detector) commonGCEAttributes() (attributes []attribute.KeyValue, errs []string) {
	if instanceID, err := d.metadata.InstanceID(); err != nil {
		errs = append(errs, err.Error())
	} else if instanceID != "" {
		attributes = append(attributes, semconv.HostIDKey.String(instanceID))
	}
	if name, err := d.metadata.InstanceName(); err != nil {
		errs = append(errs, err.Error())
	} else if name != "" {
		attributes = append(attributes, semconv.HostNameKey.String(name))
	}
	return
}

// zoneAndRegionAttributes are attributes that are available from some metadata servers
// This includes: zone, and region
func (d *detector) zoneAndRegionAttributes() (attributes []attribute.KeyValue, errs []string) {
	if zone, err := d.metadata.Zone(); err != nil {
		errs = append(errs, err.Error())
	} else if zone != "" {
		attributes = append(attributes, semconv.CloudAvailabilityZoneKey.String(zone))
		splitArr := strings.SplitN(zone, "-", 3)
		if len(splitArr) == 3 {
			attributes = append(attributes, semconv.CloudRegionKey.String(strings.Join(splitArr[0:2], "-")))
		}
	}
	return attributes, errs
}
