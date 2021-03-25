// Copyright 2020-2021 OpenTelemetry Authors
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

	"go.opentelemetry.io/otel/attribute"
	export "go.opentelemetry.io/otel/sdk/export/trace"
	"go.opentelemetry.io/otel/sdk/resource"
)

func TestInjectLabelsFromResources(t *testing.T) {
	testcases := []struct {
		name     string
		input    export.SpanSnapshot
		expected export.SpanSnapshot
	}{
		{
			name: "empty resource",
			input: export.SpanSnapshot{
				Resource: resource.NewWithAttributes(),
				Attributes: []attribute.KeyValue{
					attribute.String("a", "1"),
				},
			},
			expected: export.SpanSnapshot{
				Resource: resource.NewWithAttributes(),
				Attributes: []attribute.KeyValue{
					attribute.String("a", "1"),
				},
			},
		},
		{
			name: "empty attributes",
			input: export.SpanSnapshot{
				Resource: resource.NewWithAttributes(
					attribute.String("b", "2"),
				),
				Attributes: []attribute.KeyValue{},
			},
			expected: export.SpanSnapshot{
				Resource: resource.NewWithAttributes(
					attribute.String("b", "2"),
				),
				Attributes: []attribute.KeyValue{
					attribute.String("b", "2"),
				},
			},
		},
		{
			name: "normal insert",
			input: export.SpanSnapshot{
				Resource: resource.NewWithAttributes(
					attribute.String("b", "2"),
				),
				Attributes: []attribute.KeyValue{
					attribute.String("a", "1"),
				},
			},
			expected: export.SpanSnapshot{
				Resource: resource.NewWithAttributes(
					attribute.String("b", "2"),
				),
				Attributes: []attribute.KeyValue{
					attribute.String("a", "1"),
					attribute.String("b", "2"),
				},
			},
		},
		{
			name: "conflicts with the existing keys",
			input: export.SpanSnapshot{
				Resource: resource.NewWithAttributes(
					attribute.String("a", "2"),
				),
				Attributes: []attribute.KeyValue{
					attribute.String("a", "1"),
				},
			},
			expected: export.SpanSnapshot{
				Resource: resource.NewWithAttributes(
					attribute.String("a", "2"),
				),
				Attributes: []attribute.KeyValue{
					attribute.String("a", "1"),
				},
			},
		},
		{
			name: "allowed duplicate keys in attributes",
			input: export.SpanSnapshot{
				Resource: resource.NewWithAttributes(
					attribute.String("c", "1"),
				),
				Attributes: []attribute.KeyValue{
					attribute.String("a", "1"),
					attribute.String("b", "1"),
					attribute.String("b", "2"),
					attribute.String("b", "3"),
				},
			},
			expected: export.SpanSnapshot{
				Resource: resource.NewWithAttributes(
					attribute.String("c", "1"),
				),
				Attributes: []attribute.KeyValue{
					attribute.String("a", "1"),
					attribute.String("b", "1"),
					attribute.String("b", "2"),
					attribute.String("b", "3"),
					attribute.String("c", "1"),
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			injectLabelsFromResources(&tc.input)
			if len(tc.input.Attributes) != len(tc.expected.Attributes) {
				t.Errorf("expected: %v, actual: %v", tc.expected.Attributes, tc.input.Attributes)
				return
			}
			attrs := make(map[attribute.KeyValue]bool, len(tc.input.Attributes))

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
