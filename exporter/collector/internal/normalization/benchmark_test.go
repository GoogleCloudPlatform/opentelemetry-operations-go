// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file excepoint in compliance with the License.
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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func BenchmarkNormalizeNumberDataPoint(b *testing.B) {
	shutdown := make(chan struct{})
	defer close(shutdown)
	normalizer := NewStandardNormalizer(shutdown, zap.NewNop())
	startPoint := pmetric.NewNumberDataPoint()
	startPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	startPoint.SetIntValue(12)
	startPoint.Exemplars().AppendEmpty().SetIntValue(0)
	addAttributes(startPoint.Attributes())
	id := "abc123"
	// ensure each run is the same by skipping the first call, which will populate caches
	normalizer.NormalizeNumberDataPoint(startPoint, id)
	newPoint := pmetric.NewNumberDataPoint()
	startPoint.CopyTo(newPoint)
	newPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	var ok bool
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, ok = normalizer.NormalizeNumberDataPoint(newPoint, id)
		assert.True(b, ok)
	}
}

func BenchmarkNormalizeHistogramDataPoint(b *testing.B) {
	shutdown := make(chan struct{})
	defer close(shutdown)
	normalizer := NewStandardNormalizer(shutdown, zap.NewNop())
	startPoint := pmetric.NewHistogramDataPoint()
	startPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	startPoint.SetCount(2)
	startPoint.SetSum(10.1)
	startPoint.BucketCounts().FromRaw([]uint64{1, 1})
	startPoint.ExplicitBounds().FromRaw([]float64{1, 2})
	startPoint.Exemplars().AppendEmpty().SetIntValue(0)
	addAttributes(startPoint.Attributes())
	id := "abc123"
	// ensure each run is the same by skipping the first call, which will populate caches
	normalizer.NormalizeHistogramDataPoint(startPoint, id)
	newPoint := pmetric.NewHistogramDataPoint()
	startPoint.CopyTo(newPoint)
	newPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	var ok bool
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, ok = normalizer.NormalizeHistogramDataPoint(newPoint, id)
		assert.True(b, ok)
	}
}

func BenchmarkNormalizeExopnentialHistogramDataPoint(b *testing.B) {
	shutdown := make(chan struct{})
	defer close(shutdown)
	normalizer := NewStandardNormalizer(shutdown, zap.NewNop())
	startPoint := pmetric.NewExponentialHistogramDataPoint()
	startPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	startPoint.SetCount(4)
	startPoint.SetSum(10.1)
	startPoint.SetScale(1)
	startPoint.SetZeroCount(1)
	startPoint.Exemplars().AppendEmpty().SetIntValue(0)
	startPoint.Positive().BucketCounts().FromRaw([]uint64{1, 1})
	startPoint.Positive().SetOffset(1)
	startPoint.Negative().BucketCounts().FromRaw([]uint64{1, 1})
	startPoint.Negative().SetOffset(1)
	addAttributes(startPoint.Attributes())
	id := "abc123"
	// ensure each run is the same by skipping the first call, which will populate caches
	normalizer.NormalizeExponentialHistogramDataPoint(startPoint, id)
	newPoint := pmetric.NewExponentialHistogramDataPoint()
	startPoint.CopyTo(newPoint)
	newPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	var ok bool
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, ok = normalizer.NormalizeExponentialHistogramDataPoint(newPoint, id)
		assert.True(b, ok)
	}
}

func BenchmarkNormalizeSummaryDataPoint(b *testing.B) {
	shutdown := make(chan struct{})
	defer close(shutdown)
	normalizer := NewStandardNormalizer(shutdown, zap.NewNop())
	startPoint := pmetric.NewSummaryDataPoint()
	startPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	startPoint.SetCount(2)
	startPoint.SetSum(10.1)
	startPoint.QuantileValues().AppendEmpty().SetValue(1)
	addAttributes(startPoint.Attributes())
	id := "abc123"
	// ensure each run is the same by skipping the first call, which will populate caches
	normalizer.NormalizeSummaryDataPoint(startPoint, id)
	newPoint := pmetric.NewSummaryDataPoint()
	startPoint.CopyTo(newPoint)
	newPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	var ok bool
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, ok = normalizer.NormalizeSummaryDataPoint(newPoint, id)
		assert.True(b, ok)
	}
}

func addAttributes(attrs pcommon.Map) {
	attrs.PutStr("str", "val")
	attrs.PutBool("bool", true)
	attrs.PutInt("int", 10)
	attrs.PutDouble("double", 1.2)
	attrs.PutEmptyBytes("bytes").FromRaw([]byte{1, 3, 2})
}
