// Copyright 2021 Google LLC
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

package integrationtest

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

var (
	cmpOptions = []cmp.Option{
		protocmp.Transform(),
		cmpopts.EquateEmpty(),
	}
)

// Diff uses cmp.Diff(), protocmp, and some custom options to compare two protobuf messages.
func DiffProtos(t testing.TB, x, y *MetricExpectFixture) string {
	x = proto.Clone(x).(*MetricExpectFixture)
	y = proto.Clone(y).(*MetricExpectFixture)
	normalizeFixture(t, x)
	normalizeFixture(t, y)

	return cmp.Diff(x, y, cmpOptions...)
}
