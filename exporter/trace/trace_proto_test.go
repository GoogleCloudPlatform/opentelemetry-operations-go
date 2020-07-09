// Copyright 2020, OpenTelemetry Authors
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

package trace

import (
	"testing"

	export "go.opentelemetry.io/otel/sdk/export/trace"
	"go.opentelemetry.io/otel/sdk/resource"

	"go.opentelemetry.io/otel/api/kv"
)

func TestInjectLabelsFromResources(t *testing.T) {
	testcases := []struct {
		input    export.SpanData
		expected export.SpanData
	}{
		{
			export.SpanData{
				Resource: resource.New(),
				Attributes: []kv.KeyValue{
					kv.String("a", "1"),
				},
			},
			export.SpanData{
				Resource: resource.New(),
				Attributes: []kv.KeyValue{
					kv.String("a", "1"),
				},
			},
		},
		{
			export.SpanData{
				Resource:   resource.New(kv.String("b", "2")),
				Attributes: []kv.KeyValue{},
			},
			export.SpanData{
				Resource: resource.New(kv.String("b", "2")),
				Attributes: []kv.KeyValue{
					kv.String("b", "2"),
				},
			},
		},
		{
			export.SpanData{
				Resource: resource.New(kv.String("b", "2")),
				Attributes: []kv.KeyValue{
					kv.String("a", "1"),
				},
			},
			export.SpanData{
				Resource: resource.New(kv.String("b", "2")),
				Attributes: []kv.KeyValue{
					kv.String("a", "1"),
					kv.String("b", "2"),
				},
			},
		},
		{
			export.SpanData{
				Resource: resource.New(
					kv.String("c", "1"),
					kv.String("b", "1"),
					kv.String("b", "2"),
					kv.String("b", "3"),
				),
				Attributes: []kv.KeyValue{
					kv.String("a", "1"),
				},
			},
			export.SpanData{
				Resource: resource.New(kv.String("b", "2")),
				Attributes: []kv.KeyValue{
					kv.String("a", "1"),
					kv.String("b", "3"),
					kv.String("c", "1"),
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.input.Resource.String(), func(t *testing.T) {
			injectLabelsFromResources(&tc.input)
			if len(tc.input.Attributes) != len(tc.expected.Attributes) {
				t.Errorf("expected: %v, actual: %v", tc.expected.Attributes, tc.input.Attributes)
				return
			}
			attrs := make(map[kv.KeyValue]bool, len(tc.input.Attributes))

			for _, ele := range tc.input.Attributes {
				attrs[ele] = true
			}

			for _, ele := range tc.expected.Attributes {
				if !attrs[ele] {
					t.Errorf("expected: %v, actual: %v", tc.expected.Attributes, tc.input.Attributes)
					break
				}
			}
		})
	}

}
