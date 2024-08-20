// Copyright 2024 Google LLC
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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func BenchmarkAttributesToLabels(b *testing.B) {
	attr := pcommon.NewMap()
	attr.FromRaw(map[string]interface{}{
		"a": true,
		"b": false,
		"c": int64(12),
		"d": 12.3,
		"e": nil,
		"f": []byte{0xde, 0xad, 0xbe, 0xef},
		"g": []interface{}{"x", nil, "y"},
		"h": map[string]interface{}{"a": "b"},
	})
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		assert.Len(b, attributesToLabels(attr), 8)
	}
}
