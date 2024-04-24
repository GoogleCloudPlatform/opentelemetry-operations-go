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

package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBareMetalSolutionInstanceID(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{
			bmsInstanceIDEnv: "my-host-123",
		},
	})
	instanceID, err := d.BareMetalSolutionInstanceID()
	assert.NoError(t, err)
	assert.Equal(t, instanceID, "my-host-123")
}

func TestBareMetalSolutionInstanceIDErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{},
	})
	instanceID, err := d.BareMetalSolutionInstanceID()
	assert.Error(t, err)
	assert.Equal(t, instanceID, "")
}

func TestBareMetalSolutionCloudRegion(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{
			bmsRegionEnv: "us-central1",
		},
	})
	region, err := d.BareMetalSolutionCloudRegion()
	assert.NoError(t, err)
	assert.Equal(t, region, "us-central1")
}

func TestBareMetalSolutionCloudRegionErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{},
	})
	region, err := d.BareMetalSolutionCloudRegion()
	assert.Error(t, err)
	assert.Equal(t, region, "")
}

func TestBareMetalSolutionProjectID(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{
			bmsProjectIDEnv: "my-test-project",
		},
	})
	projectID, err := d.BareMetalSolutionProjectID()
	assert.NoError(t, err)
	assert.Equal(t, projectID, "my-test-project")
}

func TestBareMetalSolutionProjectIDErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{},
	})
	projectID, err := d.BareMetalSolutionProjectID()
	assert.Error(t, err)
	assert.Equal(t, projectID, "")
}
