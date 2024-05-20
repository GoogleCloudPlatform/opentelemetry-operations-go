// Copyright 2024 Google LLC
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

package googleclientauthextension // import "github.com/GoogleCloudPlatform/opentelemetry-operations-go/extension/googleclientauthextension"

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate_ValidAccessToken(t *testing.T) {
	cfg := &Config{
		TokenFormat: "access_token",
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_ValidIDToken(t *testing.T) {
	cfg := &Config{
		TokenFormat: "id_token",
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_Invalid(t *testing.T) {
	cfg := &Config{
		TokenFormat: "invalid",
	}

	err := cfg.Validate()
	assert.Error(t, err)
}
