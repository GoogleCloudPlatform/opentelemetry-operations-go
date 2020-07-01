// Copyright 2020, Google Inc.
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

package metric

import (
	"errors"
	"fmt"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregation"

	apimetric "go.opentelemetry.io/otel/api/metric"
)

var (
	errBlankProjectID = errors.New("expecting a non-blank ProjectID")
)

type errUnsupportedAggregation struct {
	agg  aggregation.Aggregation
}

func (e errUnsupportedAggregation) Error() string {
	return fmt.Sprintf("currently the aggregator is not supported: %v", e.agg)
}

type errUnexpectedNumberKind struct {
	kind apimetric.NumberKind
}

func (e errUnexpectedNumberKind) Error() string {
	return fmt.Sprintf("the number kind is unexpected: %v", e.kind)
}

type errUnexpectedMetricKind struct {
	kind apimetric.Kind
}

func (e errUnexpectedMetricKind) Error() string {
	return fmt.Sprintf("the metric kind is unexpected: %v", e.kind)
}
