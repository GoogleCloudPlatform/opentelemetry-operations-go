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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFAASName(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{
			faasServiceEnv: "my-service",
		},
	})
	name, err := d.FAASName()
	assert.NoError(t, err)
	assert.Equal(t, name, "my-service")
}

func TestFAASNameErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{},
	})
	name, err := d.FAASName()
	assert.Error(t, err)
	assert.Equal(t, name, "")
}

func TestFAASVersion(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{
			faasRevisionEnv: "version-123",
		},
	})
	version, err := d.FAASVersion()
	assert.NoError(t, err)
	assert.Equal(t, version, "version-123")
}

func TestFAASVersionErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{},
	})
	version, err := d.FAASVersion()
	assert.Error(t, err)
	assert.Equal(t, version, "")
}

func TestFAASInstanceID(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		FakeInstanceID: "instance-id-123",
	}, &FakeOSProvider{})
	instance, err := d.FAASInstanceID()
	assert.NoError(t, err)
	assert.Equal(t, instance, "instance-id-123")
}

func TestFAASInstanceIDErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		Err: fmt.Errorf("fake error"),
	}, &FakeOSProvider{})
	instance, err := d.FAASInstanceID()
	assert.Error(t, err)
	assert.Equal(t, instance, "")
}

func TestFAASCloudRegion(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		Attributes: map[string]string{regionMetadataAttr: "/projects/123/regions/us-central1"},
	}, &FakeOSProvider{})
	instance, err := d.FAASCloudRegion()
	assert.NoError(t, err)
	assert.Equal(t, instance, "us-central1")
}

func TestFAASCloudRegionErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		Err: fmt.Errorf("fake error"),
	}, &FakeOSProvider{})
	instance, err := d.FAASCloudRegion()
	assert.Error(t, err)
	assert.Equal(t, instance, "")
}
