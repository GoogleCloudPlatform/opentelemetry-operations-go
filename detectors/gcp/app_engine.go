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
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

func (d *detector) onAppEngine() bool {
	_, found := d.os.LookupEnv("GAE_SERVICE")
	return found
}

// appEngineAttributes detects attributes available in app-engine environments:
// https://cloud.google.com/appengine/docs/flexible/python/migrating#modules
// This includes: faas name, faas version, faas id, and cloud region
func (d *detector) appEngineAttributes() ([]attribute.KeyValue, []string) {
	attributes, errs := d.zoneAndRegionAttributes()
	if serviceName, found := d.os.LookupEnv("GAE_SERVICE"); !found {
		errs = append(errs, "envvar GAE_SERVICE not found.")
	} else {
		attributes = append(attributes, semconv.FaaSNameKey.String(serviceName))
	}
	if serviceVersion, found := d.os.LookupEnv("GAE_VERSION"); !found {
		errs = append(errs, "envvar GAE_VERSION contains empty string.")
	} else {
		attributes = append(attributes, semconv.FaaSVersionKey.String(serviceVersion))
	}
	if serviceInstance, found := d.os.LookupEnv("GAE_INSTANCE"); !found {
		errs = append(errs, "envvar GAE_INSTANCE contains empty string.")
	} else {
		attributes = append(attributes, semconv.FaaSIDKey.String(serviceInstance))
	}
	return attributes, errs
}
