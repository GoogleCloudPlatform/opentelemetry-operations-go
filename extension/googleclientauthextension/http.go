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
	"net/http"

	"golang.org/x/oauth2"
)

// roundTripper provides an HTTP RoundTripper which adds gcp credentials and
// headers.
func (ca *clientAuthenticator) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	if ca.TokenSource == nil {
		return nil, errors.New("not started")
	}
	return &oauth2.Transport{
		Source: ca,
		Base: &parameterTransport{
			base:   base,
			config: ca.config,
		},
	}, nil
}

type parameterTransport struct {
	base   http.RoundTripper
	config *Config
}

// RoundTrip adds headers related to
// Based on headers added by the google go client:
// https://github.com/googleapis/google-api-go-client/blob/113082d14d54f188d1b6c34c652e416592fc51b5/transport/http/dial.go#L122
func (t *parameterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.base == nil {
		return nil, errors.New("transport: no Transport specified")
	}
	newReq := *req
	newReq.Header = make(http.Header)
	for k, vv := range req.Header {
		newReq.Header[k] = vv
	}

	// Attach system parameters into the header
	if t.config.QuotaProject != "" {
		newReq.Header.Set("X-Goog-User-Project", t.config.QuotaProject)
	}
	if t.config.Project != "" {
		newReq.Header.Set("X-Goog-Project-ID", t.config.Project)
	}

	return t.base.RoundTrip(&newReq)
}
