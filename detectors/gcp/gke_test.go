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

func TestGKEHostID(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		FakeInstanceID: "my-instance-id-123",
	}, &FakeOSProvider{})
	instanceID, err := d.GKEHostID()
	assert.NoError(t, err)
	assert.Equal(t, instanceID, "my-instance-id-123")
}

func TestGKEHostIDErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		Err: fmt.Errorf("fake error"),
	}, &FakeOSProvider{})
	instanceID, err := d.GKEHostID()
	assert.Error(t, err)
	assert.Equal(t, instanceID, "")
}

func TestGKEHostName(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		FakeInstanceName: "my-host-123",
	}, &FakeOSProvider{})
	hostName, err := d.GKEHostName()
	assert.NoError(t, err)
	assert.Equal(t, hostName, "my-host-123")
}

func TestGKEHostNameErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		Err: fmt.Errorf("fake error"),
	}, &FakeOSProvider{})
	hostName, err := d.GKEHostName()
	assert.Error(t, err)
	assert.Equal(t, hostName, "")
}

func TestGKEClusterName(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		InstanceAttributes: map[string]string{clusterNameMetadataAttr: "my-cluster"},
	}, &FakeOSProvider{})
	clusterName, err := d.GKEClusterName()
	assert.NoError(t, err)
	assert.Equal(t, clusterName, "my-cluster")
}

func TestGKEClusterNameErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		Err: fmt.Errorf("fake error"),
	}, &FakeOSProvider{})
	clusterName, err := d.GKEClusterName()
	assert.Error(t, err)
	assert.Equal(t, clusterName, "")
}

func TestGKEAvaiabilityZoneOrRegionZonal(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		InstanceAttributes: map[string]string{clusterLocationMetadataAttr: "us-central1-c"},
	}, &FakeOSProvider{})
	location, zoneOrRegion, err := d.GKEAvaiabilityZoneOrRegion()
	assert.NoError(t, err)
	assert.Equal(t, zoneOrRegion, Zone)
	assert.Equal(t, location, "us-central1-c")
}

func TestGKEAvaiabilityZoneOrRegionRegional(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		InstanceAttributes: map[string]string{clusterLocationMetadataAttr: "us-central1"},
	}, &FakeOSProvider{})
	location, zoneOrRegion, err := d.GKEAvaiabilityZoneOrRegion()
	assert.NoError(t, err)
	assert.Equal(t, zoneOrRegion, Region)
	assert.Equal(t, location, "us-central1")
}

func TestGKEAvaiabilityZoneOrRegionMalformed(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		InstanceAttributes: map[string]string{clusterLocationMetadataAttr: "uscentral1c"},
	}, &FakeOSProvider{})
	location, zoneOrRegion, err := d.GKEAvaiabilityZoneOrRegion()
	assert.Error(t, err)
	assert.Equal(t, zoneOrRegion, UndefinedLocation)
	assert.Equal(t, location, "")
}

func TestGKEAvaiabilityZoneOrRegionErr(t *testing.T) {
	d := NewTestDetector(&FakeMetadataProvider{
		Err: fmt.Errorf("fake error"),
	}, &FakeOSProvider{})
	location, zoneOrRegion, err := d.GKEAvaiabilityZoneOrRegion()
	assert.Error(t, err)
	assert.Equal(t, zoneOrRegion, UndefinedLocation)
	assert.Equal(t, location, "")
}
