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

func (d *detector) onCloudRun() bool {
	_, found := d.os.LookupEnv("K_CONFIGURATION")
	return found
}

func (d *detector) onCloudFunctions() bool {
	_, found := d.os.LookupEnv("FUNCTION_TARGET")
	return found
}

// faasAttributes detects attributes that are available in the Cloud Run
// (https://cloud.google.com/run/docs/reference/container-contract) and
// Cloud Functions (https://cloud.google.com/functions/docs/configuring/env-var#runtime_environment_variables_set_automatically)
// environments.  This includes: FAAS name, FAAS version, FAAS ID, and cloud region
func (d *detector) faasAttributes() (attributes []attribute.KeyValue, errs []string) {
	if serviceName, found := d.os.LookupEnv("K_SERVICE"); !found {
		errs = append(errs, "envvar K_SERVICE not found.")
	} else {
		attributes = append(attributes, semconv.FaaSNameKey.String(serviceName))
	}
	if serviceVersion, found := d.os.LookupEnv("K_REVISION"); !found {
		errs = append(errs, "envvar K_REVISION not found.")
	} else {
		attributes = append(attributes, semconv.FaaSVersionKey.String(serviceVersion))
	}
	if instanceID, err := d.metadata.InstanceID(); err != nil {
		errs = append(errs, err.Error())
	} else if instanceID != "" {
		attributes = append(attributes, semconv.FaaSIDKey.String(instanceID))
	}
	if region, err := d.faasCloudRegion(); err != nil {
		errs = append(errs, err.Error())
	} else if region != "" {
		attributes = append(attributes, semconv.CloudRegionKey.String(region))
	}
	return
}

// faasCloudRegion detects region from the metadata server.  It is in the
// format /projects/123/regions/r.
// https://cloud.google.com/run/docs/reference/container-contract#metadata-server
func (d *detector) faasCloudRegion() (string, error) {
	region, err := d.metadata.Get("instance/region")
	if err != nil {
		return "", err
	}
	return region[strings.LastIndex(region, "/")+1:], nil
}
