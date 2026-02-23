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

package testcases

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"go.opentelemetry.io/collector/pdata/ptrace"
	distributionpb "google.golang.org/genproto/googleapis/api/distribution"
	googlemetricpb "google.golang.org/genproto/googleapis/api/metric"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/integrationtest/protos"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/logsutil"
)

var (
	// selfObsMetricsToNormalize is the set of self-observability metrics which may not record
	// the same value every time due to side effects. The values of these metrics get cleared
	// and are not checked in the fixture. Their labels and types are still checked.
	selfObsMetricsToNormalize = map[string]struct{}{
		"workload.googleapis.com/grpc.client.attempt.duration":                           {},
		"workload.googleapis.com/grpc.client.attempt.rcvd_total_compressed_message_size": {},
		"workload.googleapis.com/grpc.client.attempt.sent_total_compressed_message_size": {},
		"workload.googleapis.com/grpc.client.call.duration":                              {},
	}
	// selfObsMetricsToNormalize is the set of labels on self-observability metrics which change
	// each run of the fixture. Override the label with the provided value.
	selfObsLabelsToNormalize = map[string]string{
		"grpc_target": "dns:///127.0.0.1:40441",
	}
)

const (
	SecondProjectEnv = "SECOND_PROJECT_ID"
	ProejctEnv       = "PROJECT_ID"
)

type TestCase struct {
	// ConfigureCollector will be called to modify the default configuration for this test case. Optional.
	ConfigureCollector func(cfg *collector.Config)
	// ConfigureLogsExporter uses internal types to add extra post-init config to an exporter object.
	ConfigureLogsExporter *logsutil.ExporterConfig
	// Name of the test case
	Name string
	// OTLPInputFixturePath is the path to the JSON encoded OTLP
	// ExportMetricsServiceRequest input metrics fixture.
	OTLPInputFixturePath string
	// ExpectFixturePath is the path to the JSON encoded MetricExpectFixture
	// (see fixtures.proto) that contains request messages the exporter is expected to send.
	ExpectFixturePath string
	// CompareFixturePath is a second output fixture that should be equal to this test's output fixture.
	// Used for cross-referencing multiple tests that should have the same output, without overwriting the same fixtures.
	CompareFixturePath string
	// When testing the SDK metrics exporter (not collector), this is the options to use. Optional.
	MetricSDKExporterOptions []metric.Option
	// Skip, if true, skips this test case
	Skip bool
	// SkipForSDK, if true, skips this test case when testing the SDK
	SkipForSDK bool
	// ExpectErr sets whether the test is expected to fail
	ExpectErr bool
	// ExpectRetries sets whether the test expects the server to report multiple attempts
	ExpectRetries bool
}

func (tc *TestCase) LoadOTLPTracesInput(
	t testing.TB,
	startTimestamp time.Time,
	endTimestamp time.Time,
) ptrace.Traces {
	fixtureBytes, err := os.ReadFile(tc.OTLPInputFixturePath)
	require.NoError(t, err)
	unmarshaler := &ptrace.JSONUnmarshaler{}
	traces, err := unmarshaler.UnmarshalTraces(fixtureBytes)
	require.NoError(t, err)

	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rs := traces.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			sss := rs.ScopeSpans().At(j)
			for k := 0; k < sss.Spans().Len(); k++ {
				span := sss.Spans().At(k)
				if span.StartTimestamp() != 0 {
					span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTimestamp))
				}
				if span.EndTimestamp() != 0 {
					span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTimestamp))
				}
			}
		}
	}
	return traces
}

func (tc *TestCase) LoadTraceExpectFixture(
	t testing.TB,
	startTimestamp time.Time,
	endTimestamp time.Time,
) *protos.TraceExpectFixture {
	fixtureBytes, err := os.ReadFile(tc.ExpectFixturePath)
	require.NoError(t, err)
	fixture := &protos.TraceExpectFixture{}
	require.NoError(t, protojson.Unmarshal(fixtureBytes, fixture))

	for _, request := range fixture.BatchWriteSpansRequest {
		for _, span := range request.Spans {
			span.StartTime = timestamppb.New(startTimestamp)
			span.EndTime = timestamppb.New(endTimestamp)
		}
	}

	return fixture
}

func (tc *TestCase) SaveRecordedTraceFixtures(
	t testing.TB,
	fixture *protos.TraceExpectFixture,
) {
	NormalizeTraceFixture(t, fixture)

	jsonBytes, err := protojson.Marshal(fixture)
	require.NoError(t, err)
	formatted := bytes.Buffer{}
	require.NoError(t, json.Indent(&formatted, jsonBytes, "", "  "))
	_, err = formatted.WriteString("\n")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tc.ExpectFixturePath, formatted.Bytes(), 0640))
	t.Logf("Updated fixture %v", tc.ExpectFixturePath)
}

func NormalizeTraceFixture(t testing.TB, fixture *protos.TraceExpectFixture) {
	normalizeSelfObs(t, fixture.SelfObservabilityMetrics)
	for _, req := range fixture.BatchWriteSpansRequest {
		for _, span := range req.Spans {
			if span.GetStartTime() != nil {
				span.StartTime = &timestamppb.Timestamp{}
			}
			if span.GetEndTime() != nil {
				span.EndTime = &timestamppb.Timestamp{}
			}
		}
	}
}

func (tc *TestCase) CreateTraceConfig() collector.Config {
	cfg := collector.DefaultConfig()
	cfg.ProjectID = "fake-project"

	if tc.ConfigureCollector != nil {
		tc.ConfigureCollector(&cfg)
	}

	return cfg
}

func (tc *TestCase) LoadOTLPLogsInput(
	t testing.TB,
	timestamp time.Time,
) plog.Logs {
	fixtureBytes, err := os.ReadFile(tc.OTLPInputFixturePath)
	require.NoError(t, err)
	unmarshaler := &plog.JSONUnmarshaler{}
	logs, err := unmarshaler.UnmarshalLogs(fixtureBytes)
	require.NoError(t, err)

	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rl := logs.ResourceLogs().At(i)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sls := rl.ScopeLogs().At(j)
			for k := 0; k < sls.LogRecords().Len(); k++ {
				log := sls.LogRecords().At(k)
				if log.Timestamp() != 0 {
					log.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
				}
			}
		}
	}
	return logs
}

func (tc *TestCase) CreateLogConfig() collector.Config {
	cfg := collector.DefaultConfig()
	cfg.ProjectID = "fake-project"

	if tc.ConfigureCollector != nil {
		tc.ConfigureCollector(&cfg)
	}

	return cfg
}

func (tc *TestCase) LoadLogExpectFixture(
	t testing.TB,
	timestamp time.Time,
) *protos.LogExpectFixture {
	fixtureBytes, err := os.ReadFile(tc.ExpectFixturePath)
	require.NoError(t, err)
	fixture := &protos.LogExpectFixture{}
	require.NoError(t, protojson.Unmarshal(fixtureBytes, fixture))

	for _, request := range fixture.WriteLogEntriesRequests {
		for _, entry := range request.Entries {
			entry.Timestamp = timestamppb.New(timestamp)
		}
	}

	return fixture
}

func (tc *TestCase) SaveRecordedLogFixtures(
	t testing.TB,
	fixture *protos.LogExpectFixture,
) {
	NormalizeLogFixture(t, fixture)

	jsonBytes, err := protojson.Marshal(fixture)
	require.NoError(t, err)
	formatted := bytes.Buffer{}
	require.NoError(t, json.Indent(&formatted, jsonBytes, "", "  "))
	_, err = formatted.WriteString("\n")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tc.ExpectFixturePath, formatted.Bytes(), 0640))
	t.Logf("Updated fixture %v", tc.ExpectFixturePath)
}

// Normalizes timestamps which create noise in the fixture because they can
// vary each test run.
func NormalizeLogFixture(t testing.TB, fixture *protos.LogExpectFixture) {
	normalizeSelfObs(t, fixture.SelfObservabilityMetrics)
	for listIndex, req := range fixture.WriteLogEntriesRequests {
		// sort the entries in each request
		sort.Slice(fixture.WriteLogEntriesRequests[listIndex].Entries, func(i, j int) bool {
			if fixture.WriteLogEntriesRequests[listIndex].Entries[i].LogName < fixture.WriteLogEntriesRequests[listIndex].Entries[j].LogName {
				return fixture.WriteLogEntriesRequests[listIndex].Entries[i].LogName < fixture.WriteLogEntriesRequests[listIndex].Entries[j].LogName
			}
			return fixture.WriteLogEntriesRequests[listIndex].Entries[i].String() < fixture.WriteLogEntriesRequests[listIndex].Entries[j].String()
		})
		for _, entry := range req.Entries {
			// Normalize timestamps if they are set
			if entry.GetTimestamp() != nil {
				entry.Timestamp = &timestamppb.Timestamp{}
			}
		}
	}
	// sort each request. if the requests have the same name (or just as likely, they both have no name set at the request level),
	// peek at the first entry's logname in the request
	sort.Slice(fixture.WriteLogEntriesRequests, func(i, j int) bool {
		if fixture.WriteLogEntriesRequests[i].LogName != fixture.WriteLogEntriesRequests[j].LogName {
			return fixture.WriteLogEntriesRequests[i].LogName < fixture.WriteLogEntriesRequests[j].LogName
		}
		return fixture.WriteLogEntriesRequests[i].Entries[0].LogName < fixture.WriteLogEntriesRequests[j].Entries[0].LogName
	})
}

// Load OTLP metric fixture, test expectation fixtures and modify them so they're suitable for
// testing. Currently, this just updates the timestamps.
//
//nolint:revive
func (tc *TestCase) LoadOTLPMetricsInput(
	t testing.TB,
	startTime time.Time,
	endTime time.Time,
) pmetric.Metrics {
	fixtureBytes, err := os.ReadFile(tc.OTLPInputFixturePath)
	require.NoError(t, err)
	unmarshaler := &pmetric.JSONUnmarshaler{}
	metrics, err := unmarshaler.UnmarshalMetrics(fixtureBytes)
	require.NoError(t, err)

	// Interface with common fields that pdata metric points have
	type point interface {
		StartTimestamp() pcommon.Timestamp
		Timestamp() pcommon.Timestamp
		SetStartTimestamp(pcommon.Timestamp)
		SetTimestamp(pcommon.Timestamp)
	}
	type pointWithExemplars interface {
		point
		Exemplars() pmetric.ExemplarSlice
	}
	updatePoint := func(p point) {
		if p.StartTimestamp() != 0 {
			p.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
		}
		if p.Timestamp() != 0 {
			p.SetTimestamp(pcommon.NewTimestampFromTime(endTime))
		}
	}
	updatePointWithExemplars := func(p pointWithExemplars) {
		updatePoint(p)
		for i := 0; i < p.Exemplars().Len(); i++ {
			p.Exemplars().At(i).SetTimestamp(pcommon.NewTimestampFromTime(endTime))
		}

	}

	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for i := 0; i < rm.ScopeMetrics().Len(); i++ {
			sms := rm.ScopeMetrics().At(i)
			for i := 0; i < sms.Metrics().Len(); i++ {
				m := sms.Metrics().At(i)

				switch m.Type() {
				case pmetric.MetricTypeGauge:
					for i := 0; i < m.Gauge().DataPoints().Len(); i++ {
						updatePointWithExemplars(m.Gauge().DataPoints().At(i))
					}
				case pmetric.MetricTypeSum:
					for i := 0; i < m.Sum().DataPoints().Len(); i++ {
						updatePointWithExemplars(m.Sum().DataPoints().At(i))
					}
				case pmetric.MetricTypeHistogram:
					for i := 0; i < m.Histogram().DataPoints().Len(); i++ {
						updatePointWithExemplars(m.Histogram().DataPoints().At(i))
					}
				case pmetric.MetricTypeSummary:
					for i := 0; i < m.Summary().DataPoints().Len(); i++ {
						updatePoint(m.Summary().DataPoints().At(i))
					}
				case pmetric.MetricTypeExponentialHistogram:
					for i := 0; i < m.ExponentialHistogram().DataPoints().Len(); i++ {
						updatePointWithExemplars(m.ExponentialHistogram().DataPoints().At(i))
					}
				}
			}
		}
	}

	return metrics
}

func (tc *TestCase) LoadMetricFixture(
	t testing.TB,
	path string,
	startTime time.Time,
	endTime time.Time,
) *protos.MetricExpectFixture {
	fixtureBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	fixture := &protos.MetricExpectFixture{}
	require.NoError(t, protojson.Unmarshal(fixtureBytes, fixture))
	tc.updateMetricExpectFixture(t, startTime, endTime, fixture)

	return fixture
}

func (tc *TestCase) updateMetricExpectFixture(
	t testing.TB,
	startTime time.Time,
	endTime time.Time,
	fixture *protos.MetricExpectFixture,
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
				if ts.GetValueType() == googlemetricpb.MetricDescriptor_DISTRIBUTION {
					for _, ex := range p.GetValue().GetDistributionValue().GetExemplars() {
						ex.Timestamp = timestamppb.New(endTime)
					}
				}
			}
		}
	}
}

func (tc *TestCase) SaveRecordedMetricFixtures(
	t testing.TB,
	fixture *protos.MetricExpectFixture,
) {
	NormalizeMetricFixture(t, fixture)

	jsonBytes, err := protojson.Marshal(fixture)
	require.NoError(t, err)
	formatted := bytes.Buffer{}
	require.NoError(t, json.Indent(&formatted, jsonBytes, "", "  "))
	_, err = formatted.WriteString("\n")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tc.ExpectFixturePath, formatted.Bytes(), 0640))
	t.Logf("Updated fixture %v", tc.ExpectFixturePath)
}

// Normalizes timestamps which create noise in the fixture because they can
// vary each test run.
func NormalizeMetricFixture(t testing.TB, fixture *protos.MetricExpectFixture) {
	normalizeTimeSeriesReqs(t, fixture.CreateTimeSeriesRequests...)
	normalizeTimeSeriesReqs(t, fixture.CreateServiceTimeSeriesRequests...)
	normalizeMetricDescriptorReqs(t, fixture.CreateMetricDescriptorRequests...)
	normalizeSelfObs(t, fixture.SelfObservabilityMetrics)
	sort.Slice(fixture.CreateTimeSeriesRequests, func(i, j int) bool {
		return fixture.CreateTimeSeriesRequests[i].Name < fixture.CreateTimeSeriesRequests[j].Name
	})
	sort.Slice(fixture.CreateMetricDescriptorRequests, func(i, j int) bool {
		if fixture.CreateMetricDescriptorRequests[i].Name != fixture.CreateMetricDescriptorRequests[j].Name {
			return fixture.CreateMetricDescriptorRequests[i].Name < fixture.CreateMetricDescriptorRequests[j].Name
		}
		return fixture.CreateMetricDescriptorRequests[i].MetricDescriptor.Name < fixture.CreateMetricDescriptorRequests[j].MetricDescriptor.Name
	})
	sort.Slice(fixture.CreateServiceTimeSeriesRequests, func(i, j int) bool {
		return fixture.CreateServiceTimeSeriesRequests[i].Name < fixture.CreateServiceTimeSeriesRequests[j].Name
	})
}

func normalizeTimeSeriesReqs(t testing.TB, reqs ...*monitoringpb.CreateTimeSeriesRequest) {
	for _, req := range reqs {
		for _, ts := range req.TimeSeries {
			for _, p := range ts.Points {
				// Normalize timestamps if they are set
				if p.GetInterval().GetStartTime() != nil {
					p.GetInterval().StartTime = &timestamppb.Timestamp{}
				}
				if p.GetInterval().GetEndTime() != nil {
					p.GetInterval().EndTime = &timestamppb.Timestamp{}
				}
				if ts.GetValueType() == googlemetricpb.MetricDescriptor_DISTRIBUTION {
					for _, ex := range p.GetValue().GetDistributionValue().GetExemplars() {
						ex.Timestamp = &timestamppb.Timestamp{}
					}
				}
			}

			// clear project ID from monitored resource
			delete(ts.GetResource().GetLabels(), "project_id")
		}
	}
}

func normalizeMetricDescriptorReqs(t testing.TB, reqs ...*monitoringpb.CreateMetricDescriptorRequest) {
	for _, req := range reqs {
		if req.MetricDescriptor == nil {
			continue
		}
		md := req.MetricDescriptor
		sort.Slice(md.Labels, func(i, j int) bool {
			return md.Labels[i].Key < md.Labels[j].Key
		})
	}
}

func normalizeSelfObs(t testing.TB, selfObs *protos.SelfObservabilityMetric) {
	if selfObs == nil {
		return
	}
	for _, req := range selfObs.CreateTimeSeriesRequests {
		normalizeTimeSeriesReqs(t, req)
		tss := req.TimeSeries
		for _, ts := range tss {
			if _, ok := selfObsMetricsToNormalize[ts.Metric.Type]; ok {
				// zero out the specific value type
				switch value := ts.Points[0].Value.Value.(type) {
				case *monitoringpb.TypedValue_Int64Value:
					value.Int64Value = 0
				case *monitoringpb.TypedValue_DoubleValue:
					value.DoubleValue = 0
				case *monitoringpb.TypedValue_DistributionValue:
					// Only preserve the bucket options and zeroed out counts
					for i := range value.DistributionValue.BucketCounts {
						value.DistributionValue.BucketCounts[i] = 0
					}
					value.DistributionValue = &distributionpb.Distribution{
						BucketOptions: value.DistributionValue.BucketOptions,
						BucketCounts:  value.DistributionValue.BucketCounts,
					}
				default:
					t.Logf("Do not know how to normalize typed value type %T", value)
				}
			}
			for k, v := range selfObsLabelsToNormalize {
				if _, ok := ts.Metric.Labels[k]; ok {
					ts.Metric.Labels[k] = v
				}
			}
		}
		// sort time series by (type, labelset)
		sort.Slice(tss, func(i, j int) bool {
			iMetric := tss[i].Metric
			jMetric := tss[j].Metric
			if iMetric.Type == jMetric.Type {
				// Doesn't need to sorted correctly, just consistently
				return fmt.Sprint(iMetric.Labels) < fmt.Sprint(jMetric.Labels)
			}
			return iMetric.Type < jMetric.Type
		})
	}

	normalizeMetricDescriptorReqs(t, selfObs.CreateMetricDescriptorRequests...)
	// sort descriptors by type
	sort.Slice(selfObs.CreateMetricDescriptorRequests, func(i, j int) bool {
		return selfObs.CreateMetricDescriptorRequests[i].MetricDescriptor.Type <
			selfObs.CreateMetricDescriptorRequests[j].MetricDescriptor.Type
	})
}

func (tc *TestCase) SkipIfNeeded(t testing.TB) {
	if tc.Skip {
		t.Skip("Test case is marked to skip")
	}
}

func (tc *TestCase) SkipIfNeededForSDK(t testing.TB) {
	if tc.SkipForSDK {
		t.Skip("Test case is marked to skip")
	}
}

func (tc *TestCase) CreateCollectorMetricConfig() collector.Config {
	cfg := collector.DefaultConfig()
	cfg.ProjectID = "fakeprojectid"
	// Set a big buffer to capture all CMD requests without dropping
	cfg.MetricConfig.CreateMetricDescriptorBufferSize = 500
	cfg.MetricConfig.InstrumentationLibraryLabels = false

	if tc.ConfigureCollector != nil {
		tc.ConfigureCollector(&cfg)
	}

	return cfg
}
