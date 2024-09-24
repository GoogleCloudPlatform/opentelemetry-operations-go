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

import "cloud.google.com/go/compute/metadata"

func NewTestDetector(metadata *FakeMetadataProvider, os *FakeOSProvider) *Detector {
	return &Detector{metadata: metadata, os: os}
}

type FakeMetadataProvider struct {
	Err                  error
	Attributes           map[string]string
	InstanceAttributes   map[string]string
	FakeInstanceID       string
	FakeInstanceName     string
	FakeInstanceHostname string
	FakeZone             string
	Project              string
}

func (f *FakeMetadataProvider) ProjectID() (string, error) {
	if f.Err != nil {
		return "", f.Err
	}
	return f.Project, nil
}
func (f *FakeMetadataProvider) InstanceID() (string, error) {
	if f.Err != nil {
		return "", f.Err
	}
	return f.FakeInstanceID, nil
}
func (f *FakeMetadataProvider) Get(s string) (string, error) {
	if f.Err != nil {
		return "", f.Err
	}
	return f.Attributes[s], nil
}
func (f *FakeMetadataProvider) InstanceName() (string, error) {
	if f.Err != nil {
		return "", f.Err
	}
	return f.FakeInstanceName, nil
}
func (f *FakeMetadataProvider) Hostname() (string, error) {
	if f.Err != nil {
		return "", f.Err
	}
	return f.FakeInstanceHostname, nil
}
func (f *FakeMetadataProvider) Zone() (string, error) {
	if f.Err != nil {
		return "", f.Err
	}
	return f.FakeZone, nil
}
func (f *FakeMetadataProvider) InstanceAttributeValue(s string) (string, error) {
	if f.Err != nil {
		return "", f.Err
	}
	value, ok := f.InstanceAttributes[s]
	if ok {
		return value, nil
	}
	return "", metadata.NotDefinedError(s)
}

type FakeOSProvider struct {
	Vars map[string]string
}

func (f *FakeOSProvider) LookupEnv(env string) (string, bool) {
	v, ok := f.Vars[env]
	return v, ok
}
