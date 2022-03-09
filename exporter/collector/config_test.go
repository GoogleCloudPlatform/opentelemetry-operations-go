// Copyright 2021 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collector

import "testing"

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
	} {
		t.Run(tc.desc, func(t *testing.T) {
			err := ValidateConfig(tc.input)
			if (err != nil && !tc.expectedErr) || (err == nil && tc.expectedErr) {
				t.Errorf("ValidateConfig(%v) = %v; want no error", tc.input, err)
			}
		})
	}
}
