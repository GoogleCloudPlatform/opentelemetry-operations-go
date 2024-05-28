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
	"errors"

	"go.opentelemetry.io/collector/component"
)

const (
	// accessToken indicates OAuth 2.0 access token (https://cloud.google.com/docs/authentication/token-types#access)
	accessToken = "access_token"

	// idToken indicates Google-signed ID-token (https://cloud.google.com/docs/authentication/token-types#id)
	idToken = "id_token"
)

var tokenTypes = map[string]struct{}{
	accessToken: {},
	idToken:     {},
}

// Config stores the configuration for GCP Client Credentials.
type Config struct {
	// Project is the project telemetry is sent to if the gcp.project.id
	// resource attribute is not set. If unspecified, this is determined using
	// application default credentials.
	Project string `mapstructure:"project"`

	// QuotaProject specifies a project for quota and billing purposes. The
	// caller must have serviceusage.services.use permission on the project.
	//
	// For more information please read:
	// https://cloud.google.com/apis/docs/system-parameters
	QuotaProject string `mapstructure:"quota_project"`

	// TokenType specifies which type of token will be generated.
	// default: access_token
	TokenType string `mapstructure:"token_type,omitempty"`

	// Audience specifies the audience claim used for generating ID token.
	Audience string `mapstructure:"audience,omitempty"`

	// Scope specifies optional requested permissions.
	// See https://datatracker.ietf.org/doc/html/rfc6749#section-3.3
	Scopes []string `mapstructure:"scopes,omitempty"`

	// TODO: Support impersonation, similar to what exists in the googlecloud collector exporter.
}

var _ component.Config = (*Config)(nil)

// Validate checks if the extension configuration is valid.
func (cfg *Config) Validate() error {
	if _, ok := tokenTypes[cfg.TokenType]; !ok {
		return errors.New("invalid token_type")
	}

	if cfg.TokenType == idToken && cfg.Audience == "" {
		return errors.New("audience must be specified when using the id_token token_type")
	}

	return nil
}

// defaultScopes are the scopes required for writing logs, metrics, and traces.
var defaultScopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/logging.write",
	"https://www.googleapis.com/auth/monitoring.write",
	"https://www.googleapis.com/auth/trace.append",
}

func CreateDefaultConfig() component.Config {
	return &Config{
		Scopes:    defaultScopes,
		TokenType: accessToken,
	}
}
