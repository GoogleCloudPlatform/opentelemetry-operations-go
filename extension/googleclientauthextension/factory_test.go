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
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
)

type mockIDTokenSource struct {
	token string
}

func (ts *mockIDTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: ts.token,
	}, nil
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateExtension(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds.json")
	ext, err := CreateExtension(context.Background(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
}

func TestStart(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds.json")
	ext, err := CreateExtension(context.Background(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
	err = ext.Start(context.Background(), nil)
	assert.NoError(t, err)
}

func TestStart_WithError(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/foo.json")
	ext, err := CreateExtension(context.Background(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
	err = ext.Start(context.Background(), nil)
	assert.Error(t, err)
}

func TestStart_WithNoProjectError(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	ext, err := CreateExtension(context.Background(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
	err = ext.Start(context.Background(), nil)
	assert.Error(t, err)
}

func TestStart_WithProjectFromEnvVar(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds_no_project.json")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	ext, err := CreateExtension(context.Background(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
	err = ext.Start(context.Background(), nil)
	assert.NoError(t, err)
}

func TestStart_WithProjectOverride(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds.json")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "my-overridden-project")
	ext, err := CreateExtension(context.Background(), extension.Settings{}, CreateDefaultConfig())
	assert.NotNil(t, ext)
	assert.NoError(t, err)
	err = ext.Start(context.Background(), nil)
	assert.NoError(t, err)
	// verify the value of the overridden project id
	ca, ok := ext.(*clientAuthenticator)
	if !ok {
		t.Fatalf("Returned extension is not of type *clientAuthenticator. Got: %T", ext)
	}
	assert.Equal(t, ca.config.Project, "my-overridden-project")
}

func TestStart_idtoken(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/fake_creds.json")
	ca := &clientAuthenticator{
		config: &Config{
			Project:   "my-project",
			Scopes:    defaultScopes,
			TokenType: idToken,
			Audience:  "my-audience",
		},
		newIDTokenSource: func(ctx context.Context, audience string, opts ...idtoken.ClientOption) (oauth2.TokenSource, error) {
			// opts should have idtoken.WithCredentialsJSON
			assert.Len(t, opts, 1)

			return &mockIDTokenSource{token: "dummy token"}, nil
		},
	}
	err := ca.Start(context.Background(), nil)
	assert.NoError(t, err)

	token, err := ca.Token()
	assert.NoError(t, err)
	assert.Equal(t, "dummy token", token.AccessToken)
}

func Test_newTokenSource_idtokenWithoutCredsJSON(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	ca := &clientAuthenticator{
		config: &Config{
			Project:   "my-project",
			Scopes:    defaultScopes,
			TokenType: idToken,
			Audience:  "my-audience",
		},
		newIDTokenSource: func(ctx context.Context, audience string, opts ...idtoken.ClientOption) (oauth2.TokenSource, error) {
			// opts should have no item
			assert.Len(t, opts, 0)

			return &mockIDTokenSource{token: "dummy token"}, nil
		},
	}
	ts, err := ca.newTokenSource(context.Background(), &google.Credentials{
		ProjectID: "my-project",
	})
	assert.NotNil(t, ts)
	assert.NoError(t, err)

	token, err := ts.Token()
	assert.NoError(t, err)
	assert.Equal(t, "dummy token", token.AccessToken)
}
