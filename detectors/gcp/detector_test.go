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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

func TestDetect(t *testing.T) {
	// Set this before all tests to ensure metadata.onGCE() returns true
	err := os.Setenv("GCE_METADATA_HOST", "169.254.169.254")
	assert.NoError(t, err)

	for _, tc := range []struct {
		desc             string
		metadata         metadataProvider
		os               osProvider
		expectErr        bool
		expectedResource *resource.Resource
	}{
		{
			desc: "zonal GKE cluster",
			metadata: &fakeMetadataProvider{
				project:      "my-project",
				instanceID:   "1472385723456792345",
				instanceName: "my-gke-node-1234",
				instanceAttributes: map[string]string{
					"cluster-name":     "my-cluster",
					"cluster-location": "us-central1-c",
				},
			},
			os: &fakeOSProvider{vars: map[string]string{"KUBERNETES_SERVICE_HOST": "foo"}},
			expectedResource: resource.NewWithAttributes(semconv.SchemaURL,
				semconv.CloudProviderGCP,
				semconv.CloudAccountIDKey.String("my-project"),
				semconv.CloudPlatformGCPKubernetesEngine,
				semconv.K8SClusterNameKey.String("my-cluster"),
				semconv.CloudAvailabilityZoneKey.String("us-central1-c"),
				semconv.HostIDKey.String("1472385723456792345"),
				semconv.HostNameKey.String("my-gke-node-1234"),
			),
		},
		{
			desc: "regional GKE cluster",
			metadata: &fakeMetadataProvider{
				project:      "my-project",
				instanceID:   "1472385723456792345",
				instanceName: "my-gke-node-1234",
				instanceAttributes: map[string]string{
					"cluster-name":     "my-cluster",
					"cluster-location": "us-central1",
				},
			},
			os: &fakeOSProvider{vars: map[string]string{"KUBERNETES_SERVICE_HOST": "foo"}},
			expectedResource: resource.NewWithAttributes(semconv.SchemaURL,
				semconv.CloudProviderGCP,
				semconv.CloudAccountIDKey.String("my-project"),
				semconv.CloudPlatformGCPKubernetesEngine,
				semconv.K8SClusterNameKey.String("my-cluster"),
				semconv.CloudRegionKey.String("us-central1"),
				semconv.HostIDKey.String("1472385723456792345"),
				semconv.HostNameKey.String("my-gke-node-1234"),
			),
		},
		{
			desc: "GCE",
			metadata: &fakeMetadataProvider{
				project:      "my-project",
				instanceID:   "1472385723456792345",
				instanceName: "my-gke-node-1234",
				attributes: map[string]string{
					"instance/machine-type": "n1-standard1",
				},
				zone: "us-central1-c",
			},
			os: &fakeOSProvider{vars: map[string]string{}},
			expectedResource: resource.NewWithAttributes(semconv.SchemaURL,
				semconv.CloudProviderGCP,
				semconv.CloudAccountIDKey.String("my-project"),
				semconv.CloudPlatformGCPComputeEngine,
				semconv.HostIDKey.String("1472385723456792345"),
				semconv.HostNameKey.String("my-gke-node-1234"),
				semconv.HostTypeKey.String("n1-standard1"),
				semconv.CloudRegionKey.String("us-central1"),
				semconv.CloudAvailabilityZoneKey.String("us-central1-c"),
			),
		},
		{
			desc: "Cloud Run",
			metadata: &fakeMetadataProvider{
				project:    "my-project",
				instanceID: "1472385723456792345",
				attributes: map[string]string{
					"instance/region": "/projects/123/regions/us-central1",
				},
			},
			os: &fakeOSProvider{vars: map[string]string{
				"K_CONFIGURATION": "foo",
				"K_SERVICE":       "my-service",
				"K_REVISION":      "123456",
			}},
			expectedResource: resource.NewWithAttributes(semconv.SchemaURL,
				semconv.CloudProviderGCP,
				semconv.CloudAccountIDKey.String("my-project"),
				semconv.CloudPlatformGCPCloudRun,
				semconv.CloudRegionKey.String("us-central1"),
				semconv.FaaSNameKey.String("my-service"),
				semconv.FaaSVersionKey.String("123456"),
				semconv.FaaSIDKey.String("1472385723456792345"),
			),
		},
		{
			desc: "Cloud Functions",
			metadata: &fakeMetadataProvider{
				project:    "my-project",
				instanceID: "1472385723456792345",
				attributes: map[string]string{
					"instance/region": "/projects/123/regions/us-central1",
				},
			},
			os: &fakeOSProvider{vars: map[string]string{
				"FUNCTION_TARGET": "foo",
				"K_SERVICE":       "my-service",
				"K_REVISION":      "123456",
			}},
			expectedResource: resource.NewWithAttributes(semconv.SchemaURL,
				semconv.CloudProviderGCP,
				semconv.CloudAccountIDKey.String("my-project"),
				semconv.CloudPlatformGCPCloudFunctions,
				semconv.CloudRegionKey.String("us-central1"),
				semconv.FaaSNameKey.String("my-service"),
				semconv.FaaSVersionKey.String("123456"),
				semconv.FaaSIDKey.String("1472385723456792345"),
			),
		},
		{
			desc: "App Engine",
			metadata: &fakeMetadataProvider{
				project: "my-project",
				zone:    "us-central1-c",
			},
			os: &fakeOSProvider{vars: map[string]string{
				"GAE_SERVICE":  "my-service",
				"GAE_VERSION":  "123456",
				"GAE_INSTANCE": "1472385723456792345",
			}},
			expectedResource: resource.NewWithAttributes(semconv.SchemaURL,
				semconv.CloudProviderGCP,
				semconv.CloudAccountIDKey.String("my-project"),
				semconv.CloudPlatformGCPAppEngine,
				semconv.CloudRegionKey.String("us-central1"),
				semconv.CloudAvailabilityZoneKey.String("us-central1-c"),
				semconv.FaaSNameKey.String("my-service"),
				semconv.FaaSVersionKey.String("123456"),
				semconv.FaaSIDKey.String("1472385723456792345"),
			),
		},
		{
			desc: "metadata error on GCE",
			metadata: &fakeMetadataProvider{
				err: fmt.Errorf("Failed to get metadata"),
			},
			os:        &fakeOSProvider{vars: map[string]string{}},
			expectErr: true,
			expectedResource: resource.NewWithAttributes(semconv.SchemaURL,
				semconv.CloudProviderGCP,
				semconv.CloudPlatformGCPComputeEngine,
			),
		},
		{
			desc: "metadata error on GKE",
			metadata: &fakeMetadataProvider{
				err: fmt.Errorf("Failed to get metadata"),
			},
			os:        &fakeOSProvider{vars: map[string]string{"KUBERNETES_SERVICE_HOST": "foo"}},
			expectErr: true,
			expectedResource: resource.NewWithAttributes(semconv.SchemaURL,
				semconv.CloudProviderGCP,
				semconv.CloudPlatformGCPKubernetesEngine,
			),
		},
		{
			desc: "metadata error on Cloud Run",
			metadata: &fakeMetadataProvider{
				err: fmt.Errorf("Failed to get metadata"),
			},
			os: &fakeOSProvider{vars: map[string]string{
				"K_CONFIGURATION": "foo",
				"K_SERVICE":       "my-service",
				"K_REVISION":      "123456",
			}},
			expectErr: true,
			expectedResource: resource.NewWithAttributes(semconv.SchemaURL,
				semconv.CloudProviderGCP,
				semconv.CloudPlatformGCPCloudRun,
				semconv.FaaSNameKey.String("my-service"),
				semconv.FaaSVersionKey.String("123456"),
			),
		},
		{
			desc: "metadata error on Cloud Functions",
			metadata: &fakeMetadataProvider{
				err: fmt.Errorf("Failed to get metadata"),
			},
			os: &fakeOSProvider{vars: map[string]string{
				"FUNCTION_TARGET": "foo",
				"K_SERVICE":       "my-service",
				"K_REVISION":      "123456",
			}},
			expectErr: true,
			expectedResource: resource.NewWithAttributes(semconv.SchemaURL,
				semconv.CloudProviderGCP,
				semconv.CloudPlatformGCPCloudFunctions,
				semconv.FaaSNameKey.String("my-service"),
				semconv.FaaSVersionKey.String("123456"),
			),
		},
		{
			desc: "metadata error on App Engine",
			metadata: &fakeMetadataProvider{
				err: fmt.Errorf("Failed to get metadata"),
			},
			os: &fakeOSProvider{vars: map[string]string{
				"GAE_SERVICE":  "my-service",
				"GAE_VERSION":  "123456",
				"GAE_INSTANCE": "1472385723456792345",
			}},
			expectErr: true,
			expectedResource: resource.NewWithAttributes(semconv.SchemaURL,
				semconv.CloudProviderGCP,
				semconv.CloudPlatformGCPAppEngine,
				semconv.FaaSNameKey.String("my-service"),
				semconv.FaaSVersionKey.String("123456"),
				semconv.FaaSIDKey.String("1472385723456792345"),
			),
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			d := &detector{metadata: tc.metadata, os: tc.os}

			res, err := d.Detect(context.TODO())
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedResource, res, "Resource object returned is incorrect")
		})
	}
}

type fakeMetadataProvider struct {
	project            string
	instanceID         string
	instanceName       string
	zone               string
	attributes         map[string]string
	instanceAttributes map[string]string
	err                error
}

func (f *fakeMetadataProvider) ProjectID() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.project, nil
}
func (f *fakeMetadataProvider) InstanceID() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.instanceID, nil
}
func (f *fakeMetadataProvider) Get(s string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.attributes[s], nil
}
func (f *fakeMetadataProvider) InstanceName() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.instanceName, nil
}
func (f *fakeMetadataProvider) Zone() (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.zone, nil
}
func (f *fakeMetadataProvider) InstanceAttributeValue(s string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.instanceAttributes[s], nil
}

type fakeOSProvider struct {
	vars map[string]string
}

func (f *fakeOSProvider) LookupEnv(env string) (string, bool) {
	v, ok := f.vars[env]
	return v, ok
}
