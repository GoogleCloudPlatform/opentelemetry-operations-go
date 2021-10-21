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
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Interface with common fields that pdata metric points have
type point interface {
	StartTimestamp() pdata.Timestamp
	Timestamp() pdata.Timestamp
	SetStartTimestamp(pdata.Timestamp)
	SetTimestamp(pdata.Timestamp)
}

type MetricsTestCase struct {
	// Name of the test case
	Name string

	// Path to the JSON encoded OTLP ExportMetricsServiceRequest input metrics fixture.
	OTLPInputFixturePath string

	// Path to the JSON encoded GCM CreateMetricDescriptorRequest output fixture that we expect
	// the exporter to send.
	CreateMetricDescriptorRequestFixturePath string

	// Path to the JSON encoded GCM CreateTimeSeriesRequest output fixture that we expect the
	// exporter to send.
	CreateTimeSeriesRequestFixturePath string
}

// Load OTLP metric fixture, test expectation fixtures and modify them so they're suitable for
// testing. Currently, this just updates the timestamps.
func (m *MetricsTestCase) LoadOTLPMetricsInput(
	t *testing.T,
	startTime time.Time,
	endTime time.Time,
) pdata.Metrics {
	bytes, err := ioutil.ReadFile(m.OTLPInputFixturePath)
	require.NoError(t, err)
	metrics, err := otlp.NewJSONMetricsUnmarshaler().UnmarshalMetrics(bytes)
	require.NoError(t, err)

	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for i := 0; i < rm.InstrumentationLibraryMetrics().Len(); i++ {
			rmi := rm.InstrumentationLibraryMetrics().At(i)
			for i := 0; i < rmi.Metrics().Len(); i++ {
				m := rmi.Metrics().At(i)

				points := []point{}
				switch m.DataType() {
				case pdata.MetricDataTypeGauge:
					for i := 0; i < m.Gauge().DataPoints().Len(); i++ {
						points = append(points, m.Gauge().DataPoints().At(i))
					}
				case pdata.MetricDataTypeSum:
					for i := 0; i < m.Sum().DataPoints().Len(); i++ {
						points = append(points, m.Sum().DataPoints().At(i))
					}
				case pdata.MetricDataTypeHistogram:
					for i := 0; i < m.Histogram().DataPoints().Len(); i++ {
						points = append(points, m.Histogram().DataPoints().At(i))
					}
				case pdata.MetricDataTypeSummary:
					for i := 0; i < m.Summary().DataPoints().Len(); i++ {
						points = append(points, m.Summary().DataPoints().At(i))
					}
				}

				for _, p := range points {
					p.SetStartTimestamp(pdata.NewTimestampFromTime(startTime))
					p.SetTimestamp(pdata.NewTimestampFromTime(endTime))
				}
			}
		}
	}

	return metrics
}

func (m *MetricsTestCase) LoadCreateMetricDescriptorFixture(
	t *testing.T,
	startTime time.Time,
	endTime time.Time,
) *monitoringpb.CreateMetricDescriptorRequest {
	bytes, err := ioutil.ReadFile(m.CreateMetricDescriptorRequestFixturePath)
	require.NoError(t, err)
	gcmRequest := &monitoringpb.CreateMetricDescriptorRequest{}
	require.NoError(t, protojson.Unmarshal(bytes, gcmRequest))
	return gcmRequest
}

func (m *MetricsTestCase) LoadCreateTimeSeriesFixture(
	t *testing.T,
	startTime time.Time,
	endTime time.Time,
) *monitoringpb.CreateTimeSeriesRequest {
	bytes, err := ioutil.ReadFile(m.CreateTimeSeriesRequestFixturePath)
	require.NoError(t, err)
	gcmRequest := &monitoringpb.CreateTimeSeriesRequest{}
	require.NoError(t, protojson.Unmarshal(bytes, gcmRequest))

	for _, ts := range gcmRequest.TimeSeries {
		for _, p := range ts.Points {
			p.Interval = &monitoringpb.TimeInterval{
				StartTime: timestamppb.New(startTime),
				EndTime:   timestamppb.New(endTime),
			}
		}
	}

	return gcmRequest
}
