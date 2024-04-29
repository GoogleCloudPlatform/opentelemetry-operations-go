// Copyright 2023 Google LLC
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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
)

// newFactory creates a factory for the GCP Auth extension.
func newFactory() extension.Factory {
	return extension.NewFactory(
		component.MustNewType("googleclientauth"),
		CreateDefaultConfig,
		CreateExtension,
		component.StabilityLevelAlpha,
	)
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := newFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestNewFactory(t *testing.T) {
	f := newFactory()
	assert.NotNil(t, f)
}

func TestCreateExtension(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds.json")
	ext, err := newFactory().CreateExtension(context.Background(), extension.CreateSettings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
}

func TestStart(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds.json")
	ext, err := newFactory().CreateExtension(context.Background(), extension.CreateSettings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
	err = ext.Start(context.Background(), nil)
	assert.NoError(t, err)
}

func TestStart_WithError(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/foo.json")
	ext, err := newFactory().CreateExtension(context.Background(), extension.CreateSettings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
	err = ext.Start(context.Background(), nil)
	assert.Error(t, err)
}
