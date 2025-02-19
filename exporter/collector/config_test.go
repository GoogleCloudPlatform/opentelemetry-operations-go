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
	"fmt"
	"testing"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"gopkg.in/yaml.v3"
)

var testBuildInfo = component.BuildInfo{
	Description: "GoogleCloudExporter Tests",
	Version:     Version(),
}

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
	setUserAgent(&config, testBuildInfo)
	expectedUserAgent := fmt.Sprintf("GoogleCloudExporter Tests/%s (linux/amd64)", Version())
	if config.UserAgent != expectedUserAgent {
		t.Fatalf("expected user agent to be %s, was %s", expectedUserAgent, config.UserAgent)
	}
}
