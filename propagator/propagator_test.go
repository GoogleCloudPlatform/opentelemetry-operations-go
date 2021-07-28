// Copyright 2021 Google LLC
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

package propagator

import (
	"fmt"
	"testing"
)

func TestTraceContextHeaderFormat(t *testing.T) {
	valid_trace_id := "d36a105d7002f0dee73c0dfb9553764a"
	valid_span_id := "f5fcf8089dec2751839"

	header := fmt.Sprintf("%s/%s;o=1", valid_trace_id, valid_span_id)
	match := TraceContextHeaderRe.FindStringSubmatch(header)
	names := TraceContextHeaderRe.SubexpNames()
	for i, n := range match {
		switch names[i] {
		case "trace_id":
			if n != valid_trace_id {
				t.Errorf("Expected %s, but got %s", valid_trace_id, n)
			}
		case "span_id":
			if n != valid_span_id {
				t.Errorf("Expected %s, but got %s", valid_span_id, n)
			}
		case "trace_flags":
			if n != "1" {
				t.Errorf("Expected %s, but got %s", "1", n)
			}
		}
	}
}
