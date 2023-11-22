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
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

func TestFileTokenSource(t *testing.T) {
	orig := readAndMarshallFile
	fts := fileTokenSource{}
	defer func() {
		readAndMarshallFile = orig
	}()

	// Environment variable not defined error.
	_, err := fts.Token()
	require.EqualError(t, err, fmt.Errorf("%v not defined", accessTokenPath).Error())

	err = os.Setenv(accessTokenPath, "foobar")
	require.NoError(t, err)
	defer func() {
		err = os.Unsetenv(accessTokenPath)
		require.NoError(t, err)
	}()

	// no accessToken.
	readAndMarshallFile = func(filePath string) (map[string]string, error) {
		require.Equal(t, "foobar", filePath)
		return map[string]string{"": "foobar"}, nil
	}
	_, err = fts.Token()
	require.EqualError(t, err, "accessToken field not present")

	// no expireTime.
	readAndMarshallFile = func(filePath string) (map[string]string, error) {
		require.Equal(t, "foobar", filePath)
		return map[string]string{"accessToken": "foobar"}, nil
	}
	_, err = fts.Token()
	require.EqualError(t, err, "expireTime field not present")

	// expireTime invalid format.
	readAndMarshallFile = func(filePath string) (map[string]string, error) {
		require.Equal(t, "foobar", filePath)
		return map[string]string{"accessToken": "foobar", "expireTime": "2023-10-16"}, nil
	}
	_, err = fts.Token()
	require.ErrorContains(t, err, "unable to parse expiry")

	// success.
	readAndMarshallFile = func(filePath string) (map[string]string, error) {
		require.Equal(t, "foobar", filePath)
		return map[string]string{"accessToken": "foobar", "expireTime": "2023-10-16T17:59:59Z"}, nil
	}
	token, err := fts.Token()
	require.NoError(t, err)
	expiry, _ := time.Parse(time.RFC3339, "2023-10-16T17:59:59Z")
	require.Equal(t, "foobar", token.AccessToken)
	require.Equal(t, expiry, token.Expiry)
}
