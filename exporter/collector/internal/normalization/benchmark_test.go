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
	startPoint := testNumberDataPoint()
	startPoint.SetTimestamp(start)
	id := uint64(12345)
	// ensure each run is the same by skipping the first call, which will populate caches
	normalizer.NormalizeNumberDataPoint(startPoint, id)

	var ok bool
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		newPoint := testNumberDataPoint()
		b.StartTimer()
		ok = normalizer.NormalizeNumberDataPoint(newPoint, id)
		assert.True(b, ok)
	}
}

func BenchmarkNormalizeHistogramDataPoint(b *testing.B) {
	shutdown := make(chan struct{})
	defer close(shutdown)
	normalizer := NewStandardNormalizer(shutdown, zap.NewNop())
	startPoint := testHistogramDataPoint()
	startPoint.SetTimestamp(start)
	id := uint64(12345)
	// ensure each run is the same by skipping the first call, which will populate caches
	normalizer.NormalizeHistogramDataPoint(startPoint, id)

	var ok bool
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		newPoint := testHistogramDataPoint()
		b.StartTimer()
		ok = normalizer.NormalizeHistogramDataPoint(newPoint, id)
		assert.True(b, ok)
	}
}

func BenchmarkNormalizeExopnentialHistogramDataPoint(b *testing.B) {
	shutdown := make(chan struct{})
	defer close(shutdown)
	normalizer := NewStandardNormalizer(shutdown, zap.NewNop())
	startPoint := testExponentialHistogramDataPoint()
	startPoint.SetTimestamp(start)
	id := uint64(12345)
	// ensure each run is the same by skipping the first call, which will populate caches
	normalizer.NormalizeExponentialHistogramDataPoint(startPoint, id)

	var ok bool
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		newPoint := testExponentialHistogramDataPoint()
		b.StartTimer()
		ok = normalizer.NormalizeExponentialHistogramDataPoint(newPoint, id)
		assert.True(b, ok)
	}
}

func BenchmarkNormalizeSummaryDataPoint(b *testing.B) {
	shutdown := make(chan struct{})
	defer close(shutdown)
	normalizer := NewStandardNormalizer(shutdown, zap.NewNop())
	startPoint := testSummaryDataPoint()
	startPoint.SetTimestamp(start)
	id := uint64(12345)
	// ensure each run is the same by skipping the first call, which will populate caches
	normalizer.NormalizeSummaryDataPoint(startPoint, id)

	var ok bool
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		newPoint := testSummaryDataPoint()
		b.StartTimer()
		ok = normalizer.NormalizeSummaryDataPoint(newPoint, id)
		assert.True(b, ok)
	}
}

func BenchmarkResetNormalizeNumberDataPoint(b *testing.B) {
	shutdown := make(chan struct{})
	defer close(shutdown)
	normalizer := NewStandardNormalizer(shutdown, zap.NewNop())
	startPoint := testNumberDataPoint()
	startPoint.SetTimestamp(start)
	startPoint.SetIntValue(int64(b.N + 1))
	id := uint64(12345)
	// ensure each run is the same by skipping the first call, which will populate caches
	normalizer.NormalizeNumberDataPoint(startPoint, id)

	var ok bool
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// each point decreases in value, which triggers a reset.
		b.StopTimer()
		newPoint := testNumberDataPoint()
		newPoint.SetIntValue(int64(b.N - i))
		b.StartTimer()
		_, ok = normalizer.NormalizeNumberDataPoint(newPoint, id)
		assert.True(b, ok)
	}
}

func BenchmarkResetNormalizeHistogramDataPoint(b *testing.B) {
	shutdown := make(chan struct{})
	defer close(shutdown)
	normalizer := NewStandardNormalizer(shutdown, zap.NewNop())
	startPoint := testHistogramDataPoint()
	startPoint.SetTimestamp(start)
	startPoint.SetSum(float64(b.N + 1))
	id := uint64(12345)
	// ensure each run is the same by skipping the first call, which will populate caches
	normalizer.NormalizeHistogramDataPoint(startPoint, id)

	var ok bool
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// each point decreases in value, which triggers a reset.
		b.StopTimer()
		newPoint := testHistogramDataPoint()
		newPoint.SetSum(float64(b.N - i))
		b.StartTimer()
		_, ok = normalizer.NormalizeHistogramDataPoint(newPoint, id)
		assert.True(b, ok)
	}
}

func BenchmarkResetNormalizeExponentialHistogramDataPoint(b *testing.B) {
	shutdown := make(chan struct{})
	defer close(shutdown)
	normalizer := NewStandardNormalizer(shutdown, zap.NewNop())
	startPoint := testExponentialHistogramDataPoint()
	startPoint.SetTimestamp(start)
	startPoint.SetSum(float64(b.N + 1))
	id := uint64(12345)
	// ensure each run is the same by skipping the first call, which will populate caches
	normalizer.NormalizeExponentialHistogramDataPoint(startPoint, id)

	var ok bool
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// each point decreases in value, which triggers a reset.
		b.StopTimer()
		newPoint := testExponentialHistogramDataPoint()
		newPoint.SetSum(float64(b.N - i))
		b.StartTimer()
		_, ok = normalizer.NormalizeExponentialHistogramDataPoint(newPoint, id)
		assert.True(b, ok)
	}
}

func BenchmarkResetNormalizeSummaryDataPoint(b *testing.B) {
	shutdown := make(chan struct{})
	defer close(shutdown)
	normalizer := NewStandardNormalizer(shutdown, zap.NewNop())
	startPoint := testSummaryDataPoint()
	startPoint.SetTimestamp(start)
	startPoint.SetSum(float64(b.N + 1))
	id := uint64(12345)
	// ensure each run is the same by skipping the first call, which will populate caches
	normalizer.NormalizeSummaryDataPoint(startPoint, id)

	var ok bool
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// each point decreases in value, which triggers a reset.
		b.StopTimer()
		newPoint := testSummaryDataPoint()
		newPoint.SetSum(float64(b.N - i))
		b.StartTimer()
		_, ok = normalizer.NormalizeSummaryDataPoint(newPoint, id)
		assert.True(b, ok)
	}
}

var (
	now   = pcommon.NewTimestampFromTime(time.Now())
	start = pcommon.NewTimestampFromTime(time.Now().Add(-time.Minute))
)

func testNumberDataPoint() pmetric.NumberDataPoint {
	point := pmetric.NewNumberDataPoint()
	point.SetTimestamp(now)
	point.SetIntValue(12)
	point.Exemplars().AppendEmpty().SetIntValue(0)
	addAttributes(point.Attributes())
	return point
}

func testHistogramDataPoint() pmetric.HistogramDataPoint {
	point := pmetric.NewHistogramDataPoint()
	point.SetTimestamp(now)
	point.SetCount(2)
	point.SetSum(10.1)
	point.BucketCounts().FromRaw([]uint64{1, 1})
	point.ExplicitBounds().FromRaw([]float64{1, 2})
	point.Exemplars().AppendEmpty().SetIntValue(0)
	addAttributes(point.Attributes())
	return point
}

func testExponentialHistogramDataPoint() pmetric.ExponentialHistogramDataPoint {
	point := pmetric.NewExponentialHistogramDataPoint()
	point.SetTimestamp(now)
	point.SetCount(4)
	point.SetSum(10.1)
	point.SetScale(1)
	point.SetZeroCount(1)
	point.Exemplars().AppendEmpty().SetIntValue(0)
	point.Positive().BucketCounts().FromRaw([]uint64{1, 1})
	point.Positive().SetOffset(1)
	point.Negative().BucketCounts().FromRaw([]uint64{1, 1})
	point.Negative().SetOffset(1)
	addAttributes(point.Attributes())
	return point
}

func testSummaryDataPoint() pmetric.SummaryDataPoint {
	point := pmetric.NewSummaryDataPoint()
	point.SetTimestamp(now)
	point.SetCount(2)
	point.SetSum(10.1)
	point.QuantileValues().AppendEmpty().SetValue(1)
	addAttributes(point.Attributes())
	return point
}

func addAttributes(attrs pcommon.Map) {
	attrs.PutStr("str", "val")
	attrs.PutBool("bool", true)
	attrs.PutInt("int", 10)
	attrs.PutDouble("double", 1.2)
	attrs.PutEmptyBytes("bytes").FromRaw([]byte{1, 3, 2})
}
