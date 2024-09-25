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
	"strings"

	"cloud.google.com/go/compute/metadata"
)

func NewTestDetector(metadataProvider *FakeMetadataProvider, os *FakeOSProvider) *Detector {
	return &Detector{metadata: metadataProvider, os: os}
}

type FakeMetadataProvider struct {
	Err      error
	Metadata map[string]string
}

func (f *FakeMetadataProvider) Get(s string) (string, error) {
	if f.Err != nil {
		return "", f.Err
	}
	value, ok := f.Metadata[s]
	if ok {
		return value, nil
	}
	return "", metadata.NotDefinedError(s)
}

// These keys are all documented at https://cloud.google.com/compute/docs/metadata/predefined-metadata-keys
func (f *FakeMetadataProvider) ProjectID() (string, error) {
	return f.Get("project/project-id")
}
func (f *FakeMetadataProvider) InstanceID() (string, error) {
	return f.Get("instance/id")
}
func (f *FakeMetadataProvider) InstanceName() (string, error) {
	return f.Get("instance/name")
}
func (f *FakeMetadataProvider) Hostname() (string, error) {
	return f.Get("instance/hostname")
}
func (f *FakeMetadataProvider) Zone() (string, error) {
	value, err := f.Get("instance/zone")
	if err != nil {
		return value, err
	}
	// https://github.com/googleapis/google-cloud-go/blob/35d1f813f755c88d6869d993796c5ddf8ac3eea3/compute/metadata/metadata.go#L663
	return value[strings.LastIndex(value, "/")+1:], nil
}
func (f *FakeMetadataProvider) InstanceAttributeValue(s string) (string, error) {
	return f.Get(fmt.Sprintf("instance/attributes/%s", s))
}

const (
	fakeRegion              = "us-central1"
	fakeZone                = fakeRegion + "-a"
	fakeProjectID           = "my-project"
	fakeProjectNumber       = "1234567890"
	fakeInstanceID          = "1234567891"
	fakeInstanceName        = "my-instance"
	fakeInstanceHostname    = fakeInstanceName + "." + fakeZone + ".c." + fakeProjectID + ".internal"
	fakeInstanceMachineType = "n1-standard1"
)

// fmp constructs a FakeMetadatProvider that responds with the basic GCE metadata fields.
// To add more metadata, pass key-value pairs as arguments.
func fmp(keyValues ...string) *FakeMetadataProvider {
	out := map[string]string{
		"project/project-id":         fakeProjectID,
		"project/numeric-project-id": fakeProjectNumber,
		"instance/id":                fakeInstanceID,
		"instance/name":              fakeInstanceName,
		"instance/hostname":          fakeInstanceHostname,
		"instance/machine-type":      fakeInstanceMachineType,
		"instance/zone":              fmt.Sprintf("projects/%s/zones/%s", fakeProjectNumber, fakeZone),
	}
	for i := 0; i < len(keyValues); i += 2 {
		out[keyValues[i]] = keyValues[i+1]
	}
	return &FakeMetadataProvider{
		Metadata: out,
	}
}

type FakeOSProvider struct {
	Vars map[string]string
}

func (f *FakeOSProvider) LookupEnv(env string) (string, bool) {
	v, ok := f.Vars[env]
	return v, ok
}
