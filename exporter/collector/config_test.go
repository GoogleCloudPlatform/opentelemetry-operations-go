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

package collector

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/grpc/credentials"
	"gopkg.in/yaml.v3"
)

func TestValidateConfig(t *testing.T) {
	for _, tc := range []struct {
		desc        string
		input       Config
		expectedErr bool
	}{
		{
			desc:  "Empty",
			input: Config{},
		},
		{
			desc:  "Default",
			input: DefaultConfig(),
		},
		{
			desc: "Duplicate attribute keys",
			input: Config{
				TraceConfig: TraceConfig{
					AttributeMappings: []AttributeMapping{
						{
							Key:         "foo",
							Replacement: "bar",
						},
						{
							Key:         "foo",
							Replacement: "baz",
						},
					},
				},
			},
			expectedErr: true,
		},
		{
			desc: "Duplicate attribute replacements",
			input: Config{
				TraceConfig: TraceConfig{
					AttributeMappings: []AttributeMapping{
						{
							Key:         "key1",
							Replacement: "same",
						},
						{
							Key:         "key2",
							Replacement: "same",
						},
					},
				},
			},
			expectedErr: true,
		},
		{
			desc: "Invalid resource filter regex",
			input: Config{
				MetricConfig: MetricConfig{
					ResourceFilters: []ResourceFilter{{Regex: "*"}},
				},
			},
			expectedErr: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			err := ValidateConfig(tc.input)
			if (err != nil && !tc.expectedErr) || (err == nil && tc.expectedErr) {
				t.Errorf("ValidateConfig(%v) = %v; want no error", tc.input, err)
			}
		})
	}
}

func TestMarshal(t *testing.T) {
	config := DefaultConfig()

	cm := confmap.New()
	err := cm.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}

	_, err = yaml.Marshal(cm.ToStringMap())
	if err != nil {
		t.Fatal(err)
	}
}

// These tests are not super useful for functionality, but they
// exist as an assertion of how we expect the user agent
// to be constructed from build info to alert if somebody
// changes it unknowingly.
func TestBuildInfoUserAgentFallback(t *testing.T) {
	config := DefaultConfig()
	SetUserAgent(&config, component.BuildInfo{
		Description: "GoogleCloudExporter Tests",
		Version:     Version(),
	})
	expectedUserAgent := fmt.Sprintf(
		"GoogleCloudExporter Tests/%s (%s/%s)",
		Version(),
		runtime.GOOS,
		runtime.GOARCH,
	)
	if config.UserAgent != expectedUserAgent {
		t.Fatalf("expected user agent to be %s, was %s", expectedUserAgent, config.UserAgent)
	}
}

type mockHost struct {
	component.Host
	extensions map[component.ID]component.Component
}

func (m *mockHost) GetExtensions() map[component.ID]component.Component {
	return m.extensions
}

type mockComponent struct {
	component.StartFunc
	component.ShutdownFunc
}

type mockClientOptionProvider struct {
	mockComponent
	opts []option.ClientOption
}

func (m *mockClientOptionProvider) ClientOptions() []option.ClientOption {
	return m.opts
}


type mockContextTokenSourceProvider struct {
	mockComponent
	ts func(context.Context) (*oauth2.Token, error)
}

func (m *mockContextTokenSourceProvider) Token(ctx context.Context) (*oauth2.Token, error) {
	return m.ts(ctx)
}

type mockDirectTokenSource struct {
	mockComponent
	token *oauth2.Token
	err   error
}

func (m *mockDirectTokenSource) Token() (*oauth2.Token, error) {
	return m.token, m.err
}

type mockGRPCClient struct {
	mockComponent
	creds credentials.PerRPCCredentials
	err   error
}

func (m *mockGRPCClient) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return m.creds, m.err
}

type mockUnrecognized struct {
	mockComponent
}

type fakeCredentials struct {
	credentials.PerRPCCredentials
}

func TestGetAuthenticatorClientOptions(t *testing.T) {
	extID := component.MustNewID("testauth")
	invalidID := "/testauth"

	tests := []struct {
		desc      string
		host      component.Host
		authName  string
		wantCount int
		wantErr   string
	}{
		{
			desc:     "Empty auth name",
			host:     &mockHost{},
			authName: "",
		},
		{
			desc:     "Nil host",
			host:     nil,
			authName: "testauth",
		},
		{
			desc:     "Invalid component ID format",
			host:     &mockHost{},
			authName: invalidID,
			wantErr:  "invalid authenticator extension ID",
		},
		{
			desc:     "Extension not found",
			host:     &mockHost{extensions: map[component.ID]component.Component{}},
			authName: "testauth",
			wantErr:  `authenticator extension "testauth" (looking for ID "testauth") not found`,
		},
		{
			desc: "clientOptionProvider resolution",
			host: &mockHost{
				extensions: map[component.ID]component.Component{
					extID: &mockClientOptionProvider{
						opts: []option.ClientOption{option.WithAPIKey("fake-api-key")},
					},
				},
			},
			authName:  "testauth",
			wantCount: 1,
		},
		{
			desc: "contextTokenSourceProvider resolution",
			host: &mockHost{
				extensions: map[component.ID]component.Component{
					extID: &mockContextTokenSourceProvider{
						ts: func(ctx context.Context) (*oauth2.Token, error) {
							return &oauth2.Token{AccessToken: "token2"}, nil
						},
					},
				},
			},
			authName:  "testauth",
			wantCount: 1,
		},
		{
			desc: "Direct oauth2.TokenSource resolution",
			host: &mockHost{
				extensions: map[component.ID]component.Component{
					extID: &mockDirectTokenSource{
						token: &oauth2.Token{AccessToken: "token3"},
					},
				},
			},
			authName:  "testauth",
			wantCount: 1,
		},
		{
			desc: "extensionauth.GRPCClient resolution",
			host: &mockHost{
				extensions: map[component.ID]component.Component{
					extID: &mockGRPCClient{
						creds: fakeCredentials{},
					},
				},
			},
			authName:  "testauth",
			wantCount: 1,
		},
		{
			desc: "GRPCClient resolution error path",
			host: &mockHost{
				extensions: map[component.ID]component.Component{
					extID: &mockGRPCClient{
						err: errors.New("grpc failure"),
					},
				},
			},
			authName: "testauth",
			wantErr:  "failed to produce PerRPCCredentials: grpc failure",
		},
		{
			desc: "Unrecognized interface error path",
			host: &mockHost{
				extensions: map[component.ID]component.Component{
					extID: &mockUnrecognized{},
				},
			},
			authName: "testauth",
			wantErr:  "does not implement a recognized authentication interface",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			opts, err := getAuthenticatorClientOptions(tt.host, tt.authName)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(opts) != tt.wantCount {
				t.Fatalf("expected %d options, got %d", tt.wantCount, len(opts))
			}
		})
	}
}

func TestGoogleApiContextSourceWrapper(t *testing.T) {
	expectedToken := &oauth2.Token{AccessToken: "wrapped-token-value"}
	expectedErr := errors.New("token retrieval error")

	tests := []struct {
		desc      string
		ts        contextTokenSourceProvider
		wantToken *oauth2.Token
		wantErr   error
	}{
		{
			desc: "successful token retrieval",
			ts: &mockContextTokenSourceProvider{
				ts: func(ctx context.Context) (*oauth2.Token, error) {
					return expectedToken, nil
				},
			},
			wantToken: expectedToken,
		},
		{
			desc: "failed token retrieval",
			ts: &mockContextTokenSourceProvider{
				ts: func(ctx context.Context) (*oauth2.Token, error) {
					return nil, expectedErr
				},
			},
			wantErr: expectedErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			w := &googleApiContextSourceWrapper{ts: tt.ts}
			tok, err := w.Token()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
			if tok != tt.wantToken {
				t.Fatalf("expected token %v, got %v", tt.wantToken, tok)
			}
		})
	}
}

