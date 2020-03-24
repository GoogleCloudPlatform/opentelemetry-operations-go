// Copyright 2020, OpenCensus Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aws

import (
	"testing"
)

func TestAWSEC2InstanceMonitoredResources(t *testing.T) {
	awsIdentityDoc := &awsIdentityDocument{
		"123456789012",
		"i-1234567890abcdef0",
		"us-west-2",
	}
	autoDetected := detectResourceType(awsIdentityDoc)

	if autoDetected == nil {
		t.Fatal("AWSEC2InstanceMonitoredResource nil")
	}
	resType, labels := autoDetected.MonitoredResource()
	if resType != "aws_ec2_instance" ||
		labels["instance_id"] != "i-1234567890abcdef0" ||
		labels["aws_account"] != "123456789012" ||
		labels["region"] != "aws:us-west-2" {
		t.Errorf("AWSEC2InstanceMonitoredResource Failed: %v", autoDetected)
	}
}
