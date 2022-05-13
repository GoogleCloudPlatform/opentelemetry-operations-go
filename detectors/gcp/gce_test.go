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

func TestGCEHostType(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		Attributes: map[string]string{machineTypeMetadataAttr: "n1-standard1"},
	}, &FakeOSProvider{})
	hostType, err := d.GCEHostType()
	assert.NoError(t, err)
	assert.Equal(t, hostType, "n1-standard1")
}

func TestGCEHostTypeErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		Err: fmt.Errorf("fake error"),
	}, &FakeOSProvider{})
	hostType, err := d.GCEHostType()
	assert.Error(t, err)
	assert.Equal(t, hostType, "")
}

func TestGCEHostID(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		FakeInstanceID: "my-instance-id-123",
	}, &FakeOSProvider{})
	instanceID, err := d.GCEHostID()
	assert.NoError(t, err)
	assert.Equal(t, instanceID, "my-instance-id-123")
}

func TestGCEHostIDErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		Err: fmt.Errorf("fake error"),
	}, &FakeOSProvider{})
	instanceID, err := d.GCEHostID()
	assert.Error(t, err)
	assert.Equal(t, instanceID, "")
}

func TestGCEHostName(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		FakeInstanceName: "my-host-123",
	}, &FakeOSProvider{})
	hostName, err := d.GCEHostName()
	assert.NoError(t, err)
	assert.Equal(t, hostName, "my-host-123")
}

func TestGCEHostNameErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		Err: fmt.Errorf("fake error"),
	}, &FakeOSProvider{})
	hostName, err := d.GCEHostName()
	assert.Error(t, err)
	assert.Equal(t, hostName, "")
}

func TestGCEAvaiabilityZoneAndRegion(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		FakeZone: "us-central1-c",
	}, &FakeOSProvider{})
	zone, region, err := d.GCEAvaiabilityZoneAndRegion()
	assert.NoError(t, err)
	assert.Equal(t, zone, "us-central1-c")
	assert.Equal(t, region, "us-central1")
}

func TestGCEAvaiabilityZoneAndRegionMalformedZone(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		FakeZone: "us-central1",
	}, &FakeOSProvider{})
	zone, region, err := d.GCEAvaiabilityZoneAndRegion()
	assert.Error(t, err)
	assert.Equal(t, zone, "")
	assert.Equal(t, region, "")
}

func TestGCEAvaiabilityZoneAndRegionNoZone(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		FakeZone: "",
	}, &FakeOSProvider{})
	zone, region, err := d.GCEAvaiabilityZoneAndRegion()
	assert.Error(t, err)
	assert.Equal(t, zone, "")
	assert.Equal(t, region, "")
}

func TestGCEAvaiabilityZoneAndRegionErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		Err: fmt.Errorf("fake error"),
	}, &FakeOSProvider{})
	zone, region, err := d.GCEAvaiabilityZoneAndRegion()
	assert.Error(t, err)
	assert.Equal(t, zone, "")
	assert.Equal(t, region, "")
}
