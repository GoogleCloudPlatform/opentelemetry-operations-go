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

package normalization

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Normalizer can normalize data points to handle cases in which the start time is unknown.
type Normalizer interface {
	// NormalizeExponentialHistogramDataPoint normalizes an exponential histogram.
	// It returns the normalized point, and true if the point should be kept.
	NormalizeExponentialHistogramDataPoint(point pmetric.ExponentialHistogramDataPoint, identifier uint64) bool
	// NormalizeHistogramDataPoint normalizes a cumulative histogram.
	// It returns the normalized point, and true if the point should be kept.
	NormalizeHistogramDataPoint(point pmetric.HistogramDataPoint, identifier uint64) bool
	// NormalizeNumberDataPoint normalizes a cumulative, monotonic sum.
	// It returns the normalized point, and true if the point should be kept.
	NormalizeNumberDataPoint(point pmetric.NumberDataPoint, identifier uint64) bool
	// NormalizeSummaryDataPoint normalizes a summary.
	// It returns the normalized point, and true if the point should be kept.
	NormalizeSummaryDataPoint(point pmetric.SummaryDataPoint, identifier uint64) bool
}
