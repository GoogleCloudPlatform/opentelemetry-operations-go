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

func TestCloudPlatformAppEngineFlex(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{
			gaeServiceEnv: "foo",
		},
	})
	platform := d.CloudPlatform()
	assert.Equal(t, platform, AppEngineFlex)
}

func TestCloudPlatformAppEngineStandard(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{
			gaeServiceEnv: "foo",
			gaeEnv:        "standard",
		},
	})
	platform := d.CloudPlatform()
	assert.Equal(t, platform, AppEngineStandard)
}

func TestCloudPlatformGKE(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{
			k8sServiceHostEnv: "foo",
		},
	})
	platform := d.CloudPlatform()
	assert.Equal(t, platform, GKE)
}

func TestCloudPlatformGCE(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{},
	})
	platform := d.CloudPlatform()
	assert.Equal(t, platform, GCE)
}

func TestCloudPlatformCloudRun(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{
			cloudRunConfigurationEnv: "foo",
		},
	})
	platform := d.CloudPlatform()
	assert.Equal(t, platform, CloudRun)
}

func TestCloudPlatformCloudRunJobs(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{
			cloudRunJobsEnv: "foo",
		},
	})
	platform := d.CloudPlatform()
	assert.Equal(t, platform, CloudRun)
}

func TestCloudPlatformCloudFunctions(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{}, &FakeOSProvider{
		Vars: map[string]string{
			cloudFunctionsTargetEnv: "foo",
		},
	})
	platform := d.CloudPlatform()
	assert.Equal(t, platform, CloudFunctions)
}

func TestProjectID(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		Project: "my-project",
	}, &FakeOSProvider{})
	project, err := d.ProjectID()
	assert.NoError(t, err)
	assert.Equal(t, project, "my-project")
}

func TestProjectIDErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		Err: fmt.Errorf("fake error"),
	}, &FakeOSProvider{})
	project, err := d.ProjectID()
	assert.Error(t, err)
	assert.Equal(t, project, "")
}
