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
	"encoding/json"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/auth"
	"golang.org/x/oauth2/google"
	"google.golang.org/grpc/credentials/oauth"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/extension/googleclientauthextension/internal/metadata"
)

// NewFactory creates a factory for the GCP Auth extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createExtension(ctx context.Context, set extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	config := cfg.(*Config)
	creds, err := google.FindDefaultCredentials(ctx, config.Scopes...)
	if err != nil {
		return nil, err
	}
	if config.Project == "" {
		config.Project = creds.ProjectID
	}
	if config.Project == "" {
		return nil, errors.New("no project set in config or found with application default credentials")
	}
	if config.QuotaProject == "" {
		config.QuotaProject = quotaProjectFromCreds(creds)
	}

	ca := clientAuthenticator{
		TokenSource: oauth.TokenSource{TokenSource: creds.TokenSource},
		config:      config,
	}

	return auth.NewClient(
		auth.WithClientRoundTripper(ca.roundTripper),
		auth.WithClientPerRPCCredentials(ca.perRPCCredentials),
	), nil
}

// clientAuthenticator supplies credentials from an oauth.TokenSource.
type clientAuthenticator struct {
	oauth.TokenSource
	config *Config
}

// quotaProjectFromCreds retrieves quota project from the credentials file.
// Based on how the google go client gets quota project:
// https://github.com/googleapis/google-api-go-client/blob/113082d14d54f188d1b6c34c652e416592fc51b5/internal/creds.go#L159
func quotaProjectFromCreds(creds *google.Credentials) string {
	if creds == nil {
		return ""
	}
	var v struct {
		QuotaProject string `json:"quota_project_id"`
	}
	if err := json.Unmarshal(creds.JSON, &v); err != nil {
		return ""
	}
	return v.QuotaProject
}
