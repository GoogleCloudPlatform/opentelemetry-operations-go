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

import "go.opentelemetry.io/collector/model/pdata"

// NewDisabledNormalizer returns a Normalizer which does not perform any
// normalization. This may be useful if standard normalization consumes too
// much memory.
func NewDisabledNormalizer() Normalizer {
	return &disabledNormalizer{}
}

type disabledNormalizer struct{}

// NormalizeExponentialHistogramDataPoint returns the point without normalizing.
func (d *disabledNormalizer) NormalizeExponentialHistogramDataPoint(point pdata.ExponentialHistogramDataPoint, _ string) *pdata.ExponentialHistogramDataPoint {
	return &point
}

// NormalizeHistogramDataPoint returns the point without normalizing.
func (d *disabledNormalizer) NormalizeHistogramDataPoint(point pdata.HistogramDataPoint, _ string) *pdata.HistogramDataPoint {
	return &point
}

// NormalizeNumberDataPoint returns the point without normalizing.
func (d *disabledNormalizer) NormalizeNumberDataPoint(point pdata.NumberDataPoint, _ string) *pdata.NumberDataPoint {
	return &point
}

// NormalizeSummaryDataPoint returns the point without normalizing.
func (d *disabledNormalizer) NormalizeSummaryDataPoint(point pdata.SummaryDataPoint, _ string) *pdata.SummaryDataPoint {
	return &point
}
