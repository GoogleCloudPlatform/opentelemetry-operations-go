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
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/compute/metadata"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

// NewDetector returns a resource.Detector which performs OpenTelemetry resource
// detection for GKE, Cloud Run, Cloud Functions, App Engine, and Compute Engine.
func NewDetector() resource.Detector {
	return &detector{metadata: metadata.NewClient(nil), os: realOSProvider{}}
}

func (d *detector) Detect(ctx context.Context) (*resource.Resource, error) {
	if !metadata.OnGCE() {
		return nil, nil
	}
	attributes := []attribute.KeyValue{semconv.CloudProviderGCP}
	var errors []string

	// Detect project attributes on all platforms
	attrs, errs := d.projectAttributes()
	attributes = append(attributes, attrs...)
	errors = append(errors, errs...)

	// Detect different resources depending on which platform we are on
	switch {
	case d.onGKE():
		attributes = append(attributes, semconv.CloudPlatformGCPKubernetesEngine)
		attrs, errs = d.gkeAttributes()
		attributes = append(attributes, attrs...)
		errors = append(errors, errs...)
	case d.onCloudRun():
		attributes = append(attributes, semconv.CloudPlatformGCPCloudRun)
		attrs, errs = d.faasAttributes()
		attributes = append(attributes, attrs...)
		errors = append(errors, errs...)
	case d.onCloudFunctions():
		attributes = append(attributes, semconv.CloudPlatformGCPCloudFunctions)
		attrs, errs = d.faasAttributes()
		attributes = append(attributes, attrs...)
		errors = append(errors, errs...)
	case d.onAppEngine():
		attributes = append(attributes, semconv.CloudPlatformGCPAppEngine)
		attrs, errs = d.appEngineAttributes()
		attributes = append(attributes, attrs...)
		errors = append(errors, errs...)
	default:
		attributes = append(attributes, semconv.CloudPlatformGCPComputeEngine)
		attrs, errs = d.gceAttributes()
		attributes = append(attributes, attrs...)
		errors = append(errors, errs...)
	}
	var aggregatedErr error
	if len(errors) > 0 {
		aggregatedErr = fmt.Errorf("detecting GCP resources: %s", errors)
	}

	return resource.NewWithAttributes(semconv.SchemaURL, attributes...), aggregatedErr
}

func (d *detector) projectAttributes() (attributes []attribute.KeyValue, errs []string) {
	if projectID, err := d.metadata.ProjectID(); err != nil {
		errs = append(errs, err.Error())
	} else if projectID != "" {
		attributes = append(attributes, semconv.CloudAccountIDKey.String(projectID))
	}
	return
}

// detector collects resource information for all GCP platforms
type detector struct {
	metadata metadataProvider
	os       osProvider
}

// metadataProvider contains the subset of the metadata.Client functions used
// by this resource detector to allow testing with a fake implementation.
type metadataProvider interface {
	ProjectID() (string, error)
	InstanceID() (string, error)
	Get(string) (string, error)
	InstanceName() (string, error)
	Zone() (string, error)
	InstanceAttributeValue(string) (string, error)
}

// osProvider contains the subset of the os package functions used by
type osProvider interface {
	LookupEnv(string) (string, bool)
}

// realOSProvider uses the os package to lookup env vars
type realOSProvider struct{}

func (realOSProvider) LookupEnv(env string) (string, bool) {
	return os.LookupEnv(env)
}
