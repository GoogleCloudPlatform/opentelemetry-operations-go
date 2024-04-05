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

const (
	bmsProjectIDEnv  = "BMS_PROJECT_ID"
	bmsRegionEnv     = "BMS_REGION"
	bmsInstanceIDEnv = "BMS_INSTANCE_ID"
)

// Use BMS_PROJECT_ID, BMS_REGION and BMS_INSTANCE_ID env vars as an indication that we are running on BMS.
func (d *Detector) onBMS() bool {
	projectID, projectIDExists := d.os.LookupEnv(bmsProjectIDEnv)
	region, regionExists := d.os.LookupEnv(bmsRegionEnv)
	instanceID, instanceIDExists := d.os.LookupEnv(bmsInstanceIDEnv)
	return projectIDExists && regionExists && instanceIDExists && projectID != "" && region != "" && instanceID != ""
}

// BMSInstanceID returns the instance ID from the BMS_INSTANCE_ID environment variable.
func (d *Detector) BMSInstanceID() (string, error) {
	if instanceID, found := d.os.LookupEnv(bmsInstanceIDEnv); found {
		return instanceID, nil
	}
	return "", errEnvVarNotFound
}

// BMSCloudRegion returns the region from the BMS_REGION environment variable.
func (d *Detector) BMSCloudRegion() (string, error) {
	if region, found := d.os.LookupEnv(bmsRegionEnv); found {
		return region, nil
	}
	return "", errEnvVarNotFound
}

// BMSProjectID returns the project ID from the BMS_PROJECT_ID environment variable.
func (d *Detector) BMSProjectID() (string, error) {
	if project, found := d.os.LookupEnv(bmsProjectIDEnv); found {
		return project, nil
	}
	return "", errEnvVarNotFound
}
