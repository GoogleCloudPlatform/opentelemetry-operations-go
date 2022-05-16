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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppEngineServiceName(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{
			gaeServiceEnv: "my-service",
		},
	})
	serviceName, err := d.AppEngineServiceName()
	assert.NoError(t, err)
	assert.Equal(t, serviceName, "my-service")
}

func TestAppEngineServiceNameErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{},
	})
	name, err := d.AppEngineServiceName()
	assert.Error(t, err)
	assert.Equal(t, name, "")
}

func TestAppEngineServiceVersion(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{
			gaeVersionEnv: "my-version",
		},
	})
	version, err := d.AppEngineServiceVersion()
	assert.NoError(t, err)
	assert.Equal(t, version, "my-version")
}

func TestAppEngineServiceVersionErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{},
	})
	version, err := d.AppEngineServiceVersion()
	assert.Error(t, err)
	assert.Equal(t, version, "")
}

func TestAppEngineServiceInstance(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{
			gaeInstanceEnv: "instance-123",
		},
	})
	instance, err := d.AppEngineServiceInstance()
	assert.NoError(t, err)
	assert.Equal(t, instance, "instance-123")
}

func TestAppEngineServiceInstanceErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{},
	})
	instance, err := d.AppEngineServiceInstance()
	assert.Error(t, err)
	assert.Equal(t, instance, "")
}
