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
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
	"google.golang.org/grpc/credentials/oauth"
)

func CreateExtension(ctx context.Context, set extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	config := cfg.(*Config)
	ca := &clientAuthenticator{
		config: config,
	}
	return ca, nil
}

// clientAuthenticator supplies credentials from an oauth.TokenSource.
type clientAuthenticator struct {
	*oauth.TokenSource
	config *Config
}

func (ca *clientAuthenticator) Start(ctx context.Context, _ component.Host) error {
	config := ca.config
	creds, err := google.FindDefaultCredentials(ctx, config.Scopes...)
	if err != nil {
		return err
	}
	if config.Project == "" {
		config.Project = creds.ProjectID
	}
	if config.Project == "" {
		return errors.New("no project set in config or found with application default credentials")
	}
	if config.QuotaProject == "" {
		config.QuotaProject = quotaProjectFromCreds(creds)
	}
	source, err := ca.newTokenSource(ctx, creds)
	if err != nil {
		return err
	}
	ca.TokenSource = &oauth.TokenSource{TokenSource: source}
	return nil
}

func (ca *clientAuthenticator) newTokenSource(ctx context.Context, creds *google.Credentials) (oauth2.TokenSource, error) {
	switch ca.config.TokenType {
	case idToken:
		return idtoken.NewTokenSource(
			ctx,
			ca.config.Audience,
			idtoken.WithCredentialsJSON(creds.JSON),
		)
	default:
		return creds.TokenSource, nil
	}
}

func (ca *clientAuthenticator) Shutdown(ctx context.Context) error {
	return nil
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
