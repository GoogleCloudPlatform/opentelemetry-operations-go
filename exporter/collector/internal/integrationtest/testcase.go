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
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
)

type MetricsTestCase struct {
	// Name of the test case
	Name string

	// Path to the JSON encoded OTLP ExportMetricsServiceRequest input metrics fixture.
	OTLPInputFixturePath string

	// Path to the JSON encoded MetricExpectFixture (see fixtures.proto) that contains request
	// messages the exporter is expected to send.
	ExpectFixturePath string

	// Whether to skip this test case
	Skip bool

	// Configure will be called to modify the default configuration for this test case. Optional.
	Configure func(cfg *collector.Config)
}

// Load OTLP metric fixture, test expectation fixtures and modify them so they're suitable for
// testing. Currently, this just updates the timestamps.
func (m *MetricsTestCase) LoadOTLPMetricsInput(
	t testing.TB,
	startTime time.Time,
	endTime time.Time,
) pdata.Metrics {
	bytes, err := ioutil.ReadFile(m.OTLPInputFixturePath)
	require.NoError(t, err)
	metrics, err := otlp.NewJSONMetricsUnmarshaler().UnmarshalMetrics(bytes)
	require.NoError(t, err)

	// Interface with common fields that pdata metric points have
	type point interface {
		StartTimestamp() pdata.Timestamp
		Timestamp() pdata.Timestamp
		SetStartTimestamp(pdata.Timestamp)
		SetTimestamp(pdata.Timestamp)
	}
	updatePoint := func(p point) {
		if p.StartTimestamp() != 0 {
			p.SetStartTimestamp(pdata.NewTimestampFromTime(startTime))
		}
		if p.Timestamp() != 0 {
			p.SetTimestamp(pdata.NewTimestampFromTime(endTime))
		}
	}

	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for i := 0; i < rm.InstrumentationLibraryMetrics().Len(); i++ {
			rmi := rm.InstrumentationLibraryMetrics().At(i)
			for i := 0; i < rmi.Metrics().Len(); i++ {
				m := rmi.Metrics().At(i)

				switch m.DataType() {
				case pdata.MetricDataTypeGauge:
					for i := 0; i < m.Gauge().DataPoints().Len(); i++ {
						updatePoint(m.Gauge().DataPoints().At(i))
					}
				case pdata.MetricDataTypeSum:
					for i := 0; i < m.Sum().DataPoints().Len(); i++ {
						updatePoint(m.Sum().DataPoints().At(i))
					}
				case pdata.MetricDataTypeHistogram:
					for i := 0; i < m.Histogram().DataPoints().Len(); i++ {
						updatePoint(m.Histogram().DataPoints().At(i))
					}
				case pdata.MetricDataTypeSummary:
					for i := 0; i < m.Summary().DataPoints().Len(); i++ {
						updatePoint(m.Summary().DataPoints().At(i))
					}
				case pdata.MetricDataTypeExponentialHistogram:
					for i := 0; i < m.ExponentialHistogram().DataPoints().Len(); i++ {
						updatePoint(m.ExponentialHistogram().DataPoints().At(i))
					}
				}
			}
		}
	}

	return metrics
}

func (m *MetricsTestCase) LoadExpectFixture(
	t testing.TB,
	startTime time.Time,
	endTime time.Time,
) *MetricExpectFixture {
	bytes, err := ioutil.ReadFile(m.ExpectFixturePath)
	require.NoError(t, err)
	fixture := &MetricExpectFixture{}
	require.NoError(t, protojson.Unmarshal(bytes, fixture))
	m.updateExpectFixture(t, startTime, endTime, fixture)

	return fixture
}

func (m *MetricsTestCase) updateExpectFixture(
	t testing.TB,
	startTime time.Time,
	endTime time.Time,
	fixture *MetricExpectFixture,
) {
	reqs := append(
		fixture.GetCreateTimeSeriesRequests(),
		fixture.GetCreateServiceTimeSeriesRequests()...,
	)
	for _, req := range reqs {
		for _, ts := range req.GetTimeSeries() {
			for _, p := range ts.GetPoints() {
				if p.GetInterval().GetStartTime() != nil {
					p.GetInterval().StartTime = timestamppb.New(startTime)
				}
				if p.GetInterval().GetEndTime() != nil {
					p.GetInterval().EndTime = timestamppb.New(endTime)
				}
			}
		}

	}
}

func (m *MetricsTestCase) SaveRecordedFixtures(
	t testing.TB,
	fixture *MetricExpectFixture,
) {
	normalizeFixture(fixture)

	jsonBytes, err := protojson.Marshal(fixture)
	require.NoError(t, err)
	formatted := bytes.Buffer{}
	require.NoError(t, json.Indent(&formatted, jsonBytes, "", "  "))
	formatted.WriteString("\n")
	require.NoError(t, ioutil.WriteFile(m.ExpectFixturePath, formatted.Bytes(), 0640))
	t.Logf("Updated fixture %v", m.ExpectFixturePath)
}

// Normalizes timestamps and removes project ID fields which create noise in the fixture
// because they can vary each test run
func normalizeFixture(fixture *MetricExpectFixture) {
	timeSeriesReqs := append(
		fixture.GetCreateTimeSeriesRequests(),
		fixture.GetCreateServiceTimeSeriesRequests()...,
	)
	for _, req := range timeSeriesReqs {
		// clear project ID
		req.Name = ""

		for _, ts := range req.GetTimeSeries() {
			for _, p := range ts.GetPoints() {
				// Normalize timestamps if they are set
				if p.GetInterval().GetStartTime() != nil {
					p.GetInterval().StartTime = &timestamppb.Timestamp{}
				}
				if p.GetInterval().GetEndTime() != nil {
					p.GetInterval().EndTime = &timestamppb.Timestamp{}
				}
			}

			// clear project ID from monitored resource
			delete(ts.GetResource().GetLabels(), "project_id")
		}
	}

	for _, req := range fixture.GetCreateMetricDescriptorRequests() {
		req.Name = ""
		if md := req.GetMetricDescriptor(); md != nil {
			md.Name = ""
		}
	}
}

func (m *MetricsTestCase) SkipIfNeeded(t testing.TB) {
	if m.Skip {
		t.Skip("Test case is marked to skip in internal/integrationtest/testcases.go")
	}
}

func (m *MetricsTestCase) CreateConfig() *collector.Config {
	cfg := collector.NewFactory().CreateDefaultConfig().(*collector.Config)
	// If not set it will use ADC
	cfg.ProjectID = os.Getenv("PROJECT_ID")
	// Disable queued retries as there is no way to flush them
	cfg.RetrySettings.Enabled = false
	cfg.QueueSettings.Enabled = false
	// Set a big buffer to capture all CMD requests without dropping
	cfg.MetricConfig.CreateMetricDescriptorBufferSize = 500

	if m.Configure != nil {
		m.Configure(cfg)
	}

	return cfg
}
