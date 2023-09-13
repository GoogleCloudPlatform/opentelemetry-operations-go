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
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoundTripper(t *testing.T) {
	ca := clientAuthenticator{}
	rt, err := ca.roundTripper(roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return nil, nil
	}))
	assert.NotNil(t, rt)
	assert.NoError(t, err)
}

func TestRoundTrip(t *testing.T) {
	tr := parameterTransport{
		config: &Config{
			Project:      "my-project",
			QuotaProject: "other-project",
		},
		base: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			assert.Equal(t, r.Header.Get("X-Goog-User-Project"), "other-project")
			assert.Equal(t, r.Header.Get("X-Goog-Project-ID"), "my-project")
			assert.Equal(t, r.Header.Get("foo"), "bar")
			return &http.Response{}, nil
		}),
	}
	header := make(http.Header)
	header.Set("foo", "bar")
	_, err := tr.RoundTrip(&http.Request{Header: header})
	assert.NoError(t, err)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func TestNilBase(t *testing.T) {
	tr := parameterTransport{
		config: &Config{},
		base:   nil,
	}
	_, err := tr.RoundTrip(&http.Request{})
	assert.Error(t, err)
}
