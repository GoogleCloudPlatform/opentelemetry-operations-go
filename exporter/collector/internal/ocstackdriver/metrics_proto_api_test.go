// Copyright 2019, OpenCensus Authors
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

package stackdriver_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/testing/protocmp"

	googlemetricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/grpc"

	sd "contrib.go.opencensus.io/exporter/stackdriver"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
)

type testCases struct {
	name     string
	inMetric []*metricspb.Metric
	outTSR   []*monitoringpb.CreateTimeSeriesRequest
	outMDR   []*monitoringpb.CreateMetricDescriptorRequest
}

var (
	// project
	projectID = "metrics_proto_test"

	// default exporter options
	defaultOpts = sd.Options{
		ProjectID: projectID,
		// Set empty labels to avoid the opencensus-task
		DefaultMonitoringLabels: &sd.Labels{},
	}

	// label keys and values
	inEmptyValue = &metricspb.LabelValue{Value: "", HasValue: true}
)

func TestVariousCasesFromFile(t *testing.T) {
	files := []string{
		"ExportLabels",
		"ExportMetricsOfAllTypes",
		"BuiltInMetrics",
	}
	for _, file := range files {
		tc := readTestCaseFromFiles(t, file)
		server, conn, doneFn := createFakeServerConn(t)
		defer doneFn()

		// Finally create the OpenCensus stats exporter
		se := createExporter(t, conn, defaultOpts)
		executeTestCase(t, tc, se, server, nil)

	}
}

func TestMetricsWithPrefix(t *testing.T) {
	server, conn, doneFn := createFakeServerConn(t)
	defer doneFn()

	tc := readTestCaseFromFiles(t, "SingleMetric")

	prefixes := []string{
		"example.com/",
		"example.com/foo/",
	}
	metricName := "ocagent.io"
	saveMetricType := tc.outTSR[0].TimeSeries[0].Metric.Type
	saveMDRName := tc.outMDR[0].MetricDescriptor.Name

	for _, prefix := range prefixes {
		opts := defaultOpts
		opts.MetricPrefix = prefix
		se := createExporter(t, conn, opts)

		mt := strings.Replace(saveMetricType, metricName, (prefix + metricName), -1)
		tc.outMDR[0].MetricDescriptor.Name = strings.Replace(saveMDRName, metricName, (prefix + metricName), -1)
		tc.outMDR[0].MetricDescriptor.Type = mt

		tc.outTSR[0].TimeSeries[0].Metric.Type = mt

		executeTestCase(t, tc, se, server, nil)
	}
}

func TestMetricsWithPrefixWithDomain(t *testing.T) {
	server, conn, doneFn := createFakeServerConn(t)
	defer doneFn()

	tc := readTestCaseFromFiles(t, "SingleMetric")

	prefixes := []string{
		"custom.googleapis.com/prometheus/",
		"external.googleapis.com/prometheus/",
	}
	metricName := "ocagent.io"
	saveMetricType := strings.Replace(tc.outTSR[0].TimeSeries[0].Metric.Type, "custom.googleapis.com/opencensus/", "", -1)
	saveMDRName := strings.Replace(tc.outMDR[0].MetricDescriptor.Name, "custom.googleapis.com/opencensus/", "", -1)

	for _, prefix := range prefixes {
		opts := defaultOpts
		opts.MetricPrefix = prefix
		se := createExporter(t, conn, opts)

		mt := strings.Replace(saveMetricType, metricName, (prefix + metricName), -1)
		tc.outMDR[0].MetricDescriptor.Name = strings.Replace(saveMDRName, metricName, (prefix + metricName), -1)
		tc.outMDR[0].MetricDescriptor.Type = mt

		tc.outTSR[0].TimeSeries[0].Metric.Type = mt

		executeTestCase(t, tc, se, server, nil)
	}
}

func TestBuiltInMetricsUsingPrefix(t *testing.T) {
	server, conn, doneFn := createFakeServerConn(t)
	defer doneFn()

	tc := readTestCaseFromFiles(t, "SingleMetric")

	prefixes := []string{
		"googleapis.com/",
		"kubernetes.io/",
		"istio.io/",
	}
	metricName := "ocagent.io"
	saveMetricType := tc.outTSR[0].TimeSeries[0].Metric.Type

	for _, prefix := range prefixes {
		opts := defaultOpts
		opts.MetricPrefix = prefix
		se := createExporter(t, conn, opts)

		// no CreateMetricDescriptorRequest expected.
		tc.outMDR = nil

		mt := strings.Replace(saveMetricType, "custom.googleapis.com/opencensus/"+metricName, (prefix + metricName), -1)
		tc.outTSR[0].TimeSeries[0].Metric.Type = mt

		executeTestCase(t, tc, se, server, nil)
	}
}

func TestMetricsWithResourcePerPushCall(t *testing.T) {
	server, conn, doneFn := createFakeServerConn(t)
	defer doneFn()

	inResources, outResources := readTestResourcesFiles(t, "Resources")
	inLen := len(inResources)
	outLen := len(outResources)
	if inLen != outLen {
		t.Errorf("Data invalid: input Resource len (%d) != output Resource len (%d)\n", inLen, outLen)
		return
	}

	tcSingleMetric := readTestCaseFromFiles(t, "SingleMetric")

	for i, inRes := range inResources {
		se := createExporter(t, conn, defaultOpts)

		tc := *tcSingleMetric
		tc.name = inRes.Type
		tc.outTSR[0].TimeSeries[0].Resource = outResources[i]

		executeTestCase(t, &tc, se, server, inRes)
	}
}

func TestMetricsWithResourcePerMetric(t *testing.T) {
	server, conn, doneFn := createFakeServerConn(t)
	defer doneFn()

	inResources, outResources := readTestResourcesFiles(t, "Resources")
	inLen := len(inResources)
	outLen := len(outResources)
	if inLen != outLen {
		t.Errorf("Data invalid: input Resource len (%d) != output Resource len (%d)\n", inLen, outLen)
		return
	}

	tcSingleMetric := readTestCaseFromFiles(t, "SingleMetric")

	for i, inRes := range inResources {
		se := createExporter(t, conn, defaultOpts)

		tc := *tcSingleMetric
		tc.name = inRes.Type
		tc.inMetric[0].Resource = inResources[i]
		tc.outTSR[0].TimeSeries[0].Resource = outResources[i]

		executeTestCase(t, &tc, se, server, nil)
	}
}

func TestMetricsWithResourcePerMetricTakesPrecedence(t *testing.T) {
	server, conn, doneFn := createFakeServerConn(t)
	defer doneFn()

	inResources, outResources := readTestResourcesFiles(t, "Resources")
	inLen := len(inResources)
	outLen := len(outResources)
	if inLen != outLen {
		t.Errorf("Data invalid: input Resource len (%d) != output Resource len (%d)\n", inLen, outLen)
		return
	}

	tcSingleMetric := readTestCaseFromFiles(t, "SingleMetric")

	// use the same resource for push call. Resource per metric should take precedence.
	perPushRes := inResources[inLen-1]

	for i, inRes := range inResources {
		se := createExporter(t, conn, defaultOpts)

		tc := *tcSingleMetric
		tc.name = inRes.Type
		tc.inMetric[0].Resource = inResources[i]
		tc.outTSR[0].TimeSeries[0].Resource = outResources[i]

		executeTestCase(t, &tc, se, server, perPushRes)
	}
}

func TestMetricsWithResourceWithMissingFieldsPerPushCall(t *testing.T) {
	server, conn, doneFn := createFakeServerConn(t)
	defer doneFn()

	inResources, outResources := readTestResourcesFiles(t, "ResourcesWithMissingFields")
	inLen := len(inResources)
	outLen := len(outResources)
	if inLen != outLen {
		t.Errorf("Data invalid: input Resource len (%d) != output Resource len (%d)\n", inLen, outLen)
		return
	}

	tcSingleMetric := readTestCaseFromFiles(t, "SingleMetric")

	for i, inRes := range inResources {
		se := createExporter(t, conn, defaultOpts)

		tc := *tcSingleMetric
		tc.name = inRes.Type
		tc.outTSR[0].TimeSeries[0].Resource = outResources[i]

		executeTestCase(t, &tc, se, server, inRes)
	}
}

func TestExportMaxTSPerRequest(t *testing.T) {
	server, conn, doneFn := createFakeServerConn(t)
	defer doneFn()

	// Finally create the OpenCensus stats exporter
	se := createExporter(t, conn, defaultOpts)

	tcFromFile := readTestCaseFromFiles(t, "SingleMetric")

	// update tcFromFile with additional input Time-series and expected Time-Series in
	// CreateTimeSeriesRequest(s). Replicate time-series with different label values.
	for i := 1; i < 250; i++ {
		v := fmt.Sprintf("value_%d", i)
		lv := &metricspb.LabelValue{Value: v, HasValue: true}

		ts := *tcFromFile.inMetric[0].Timeseries[0]
		ts.LabelValues = []*metricspb.LabelValue{inEmptyValue, lv}
		tcFromFile.inMetric[0].Timeseries = append(tcFromFile.inMetric[0].Timeseries, &ts)

		j := i / 200
		outTS := *(tcFromFile.outTSR[0].TimeSeries[0])
		outTS.Metric = &googlemetricpb.Metric{
			Type: tcFromFile.outMDR[0].MetricDescriptor.Type,
			Labels: map[string]string{
				"empty_key":      "",
				"operation_type": v,
			},
		}
		if j > len(tcFromFile.outTSR)-1 {
			newOutTSR := &monitoringpb.CreateTimeSeriesRequest{
				Name: tcFromFile.outTSR[0].Name,
			}
			tcFromFile.outTSR = append(tcFromFile.outTSR, newOutTSR)
		}
		tcFromFile.outTSR[j].TimeSeries = append(tcFromFile.outTSR[j].TimeSeries, &outTS)
	}
	executeTestCase(t, tcFromFile, se, server, nil)
}

func TestExportMaxTSPerRequestAcrossTwoMetrics(t *testing.T) {
	server, conn, doneFn := createFakeServerConn(t)
	defer doneFn()

	// Finally create the OpenCensus stats exporter
	se := createExporter(t, conn, defaultOpts)

	// Read two metrics, one CreateTimeSeriesRequest and two CreateMetricDescriptorRequest.
	tcFromFile := readTestCaseFromFiles(t, "TwoMetrics")

	// update tcFromFile with additional input Time-series and expected Time-Series in
	// CreateTimeSeriesRequest(s).
	// Replicate time-series for both metrics.
	for k := 0; k < 2; k++ {
		for i := 1; i < 250; i++ {
			v := fmt.Sprintf("value_%d", i+k*250)
			ts := *tcFromFile.inMetric[k].Timeseries[0]
			lv := &metricspb.LabelValue{Value: v, HasValue: true}
			ts.LabelValues = []*metricspb.LabelValue{inEmptyValue, lv}
			tcFromFile.inMetric[k].Timeseries = append(tcFromFile.inMetric[k].Timeseries, &ts)
		}
	}

	// Don't need the second TimeSeries in TSR
	tcFromFile.outTSR[0].TimeSeries = tcFromFile.outTSR[0].TimeSeries[:1]

	// Replicate time-series in CreateTimeSeriesRequest
	for k := 0; k < 2; k++ {
		for i := 0; i < 250; i++ {
			v := i + k*250
			if v == 0 {
				// skip first TS, it is already there.
				continue
			}
			val := fmt.Sprintf("value_%d", v)

			j := v / 200

			// pick metric-1 for first 250 time-series and metric-2 for next 250 time-series.
			mt := tcFromFile.outMDR[k].MetricDescriptor.Type
			outTS := *(tcFromFile.outTSR[0].TimeSeries[0])
			outTS.Metric = &googlemetricpb.Metric{
				Type: mt,
				Labels: map[string]string{
					"empty_key":      "",
					"operation_type": val,
				},
			}
			if j > len(tcFromFile.outTSR)-1 {
				newOutTSR := &monitoringpb.CreateTimeSeriesRequest{
					Name: tcFromFile.outTSR[0].Name,
				}
				tcFromFile.outTSR = append(tcFromFile.outTSR, newOutTSR)
			}
			tcFromFile.outTSR[j].TimeSeries = append(tcFromFile.outTSR[j].TimeSeries, &outTS)
		}
	}
	executeTestCase(t, tcFromFile, se, server, nil)
}

func createConn(t *testing.T, addr string) *grpc.ClientConn {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to make a gRPC connection to the server: %v", err)
	}
	return conn
}

func createExporter(t *testing.T, conn *grpc.ClientConn, opts sd.Options) *sd.Exporter {
	opts.MonitoringClientOptions = []option.ClientOption{option.WithGRPCConn(conn)}
	se, err := sd.NewExporter(opts)
	if err != nil {
		t.Fatalf("Failed to create the statsExporter: %v", err)
	}
	return se
}

func executeTestCase(t *testing.T, tc *testCases, se *sd.Exporter, server *fakeMetricsServer, rsc *resourcepb.Resource) {
	dropped, err := se.PushMetricsProto(context.Background(), nil, rsc, tc.inMetric)
	if dropped != 0 || err != nil {
		t.Fatalf("Name: %s, Error pushing metrics, dropped:%d err:%v", tc.name, dropped, err)
	}

	gotTimeSeries := []*monitoringpb.CreateTimeSeriesRequest{}
	server.forEachStackdriverTimeSeries(func(sdt *monitoringpb.CreateTimeSeriesRequest) {
		gotTimeSeries = append(gotTimeSeries, sdt)
	})

	if diff, idx := requireTimeSeriesRequestEqual(t, gotTimeSeries, tc.outTSR); diff != "" {
		t.Errorf("Name[%s], TimeSeries[%d], Error: -got +want %s\n", tc.name, idx, diff)
	}

	gotCreateMDRequest := []*monitoringpb.CreateMetricDescriptorRequest{}
	server.forEachStackdriverMetricDescriptor(func(sdm *monitoringpb.CreateMetricDescriptorRequest) {
		gotCreateMDRequest = append(gotCreateMDRequest, sdm)
	})

	if diff, idx := requireMetricDescriptorRequestEqual(t, gotCreateMDRequest, tc.outMDR); diff != "" {
		t.Errorf("Name[%s], MetricDescriptor[%d], Error: -got +want %s\n", tc.name, idx, diff)
	}
	server.resetStackdriverMetricDescriptors()
	server.resetStackdriverTimeSeries()
}

func readTestCaseFromFiles(t *testing.T, filename string) *testCases {
	tc := &testCases{
		name: filename,
	}

	// Read input Metrics proto.
	f, err := readFile("testdata/" + filename + "/inMetrics.txt")
	if err != nil {
		t.Fatalf("error opening in file " + filename)
	}

	strMetrics := strings.Split(string(f), "---")
	for _, strMetric := range strMetrics {
		in := metricspb.Metric{}
		err = prototext.Unmarshal([]byte(strMetric), &in)
		if err != nil {
			t.Fatalf("error unmarshalling Metric protos from file " + filename)
		}
		tc.inMetric = append(tc.inMetric, &in)
	}

	// Read expected output CreateMetricDescriptorRequest proto.
	f, err = readFile("testdata/" + filename + "/outMDR.txt")
	if err != nil {
		t.Fatalf("error opening in file " + filename)
	}

	strOutMDRs := strings.Split(string(f), "---")
	for _, strOutMDR := range strOutMDRs {
		outMDR := monitoringpb.CreateMetricDescriptorRequest{}
		err = prototext.Unmarshal([]byte(strOutMDR), &outMDR)
		if err != nil {
			t.Fatalf("error unmarshalling CreateMetricDescriptorRequest protos from file " + filename)
		}
		if outMDR.Name != "" {
			tc.outMDR = append(tc.outMDR, &outMDR)
		}
	}

	// Read expected output CreateTimeSeriesRequest proto.
	f, err = readFile("testdata/" + filename + "/outTSR.txt")
	if err != nil {
		t.Fatalf("error opening in file " + filename)
	}

	strOutTSRs := strings.Split(string(f), "---")
	for _, strOutTSR := range strOutTSRs {
		outTSR := monitoringpb.CreateTimeSeriesRequest{}
		err = prototext.Unmarshal([]byte(strOutTSR), &outTSR)
		if err != nil {
			t.Fatalf("error unmarshalling CreateTimeSeriesRequest protos from file " + filename)
		}
		tc.outTSR = append(tc.outTSR, &outTSR)
	}
	return tc
}

func readTestResourcesFiles(t *testing.T, filename string) ([]*resourcepb.Resource, []*monitoredrespb.MonitoredResource) {
	// Read input Resource proto.
	f, err := readFile("testdata/" + filename + "/in.txt")
	if err != nil {
		t.Fatalf("error opening in file " + filename)
	}

	inResources := []*resourcepb.Resource{}
	strResources := strings.Split(string(f), "---")
	for _, strRes := range strResources {
		inRes := resourcepb.Resource{}
		err = prototext.Unmarshal([]byte(strRes), &inRes)
		if err != nil {
			t.Fatalf("error unmarshalling input Resource protos from file " + filename)
		}
		inResources = append(inResources, &inRes)
	}

	// Read output Resource proto.
	f, err = readFile("testdata/" + filename + "/out.txt")
	if err != nil {
		t.Fatalf("error opening out file " + filename)
	}

	outResources := []*monitoredrespb.MonitoredResource{}
	strResources = strings.Split(string(f), "---")
	for _, strRes := range strResources {
		outRes := monitoredrespb.MonitoredResource{}
		err = prototext.Unmarshal([]byte(strRes), &outRes)
		if err != nil {
			t.Fatalf("error unmarshalling output Resource protos from file " + filename)
		}
		outResources = append(outResources, &outRes)
	}

	return inResources, outResources
}

type fakeMetricsServer struct {
	monitoringpb.MetricServiceServer
	mu                           sync.RWMutex
	stackdriverTimeSeries        []*monitoringpb.CreateTimeSeriesRequest
	stackdriverMetricDescriptors []*monitoringpb.CreateMetricDescriptorRequest
}

func createFakeServerConn(t *testing.T) (*fakeMetricsServer, *grpc.ClientConn, func()) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to bind to an available address: %v", err)
	}
	server := new(fakeMetricsServer)
	srv := grpc.NewServer()
	monitoringpb.RegisterMetricServiceServer(srv, server)
	go func() {
		_ = srv.Serve(ln)
	}()
	_, serverPortStr, _ := net.SplitHostPort(ln.Addr().String())
	conn := createConn(t, "localhost:"+serverPortStr)

	stop := func() {
		conn.Close()
		srv.Stop()
		_ = ln.Close()
	}
	return server, conn, stop
}

func (server *fakeMetricsServer) forEachStackdriverTimeSeries(fn func(sdt *monitoringpb.CreateTimeSeriesRequest)) {
	server.mu.RLock()
	defer server.mu.RUnlock()

	for _, sdt := range server.stackdriverTimeSeries {
		fn(sdt)
	}
}

func (server *fakeMetricsServer) forEachStackdriverMetricDescriptor(fn func(sdmd *monitoringpb.CreateMetricDescriptorRequest)) {
	server.mu.RLock()
	defer server.mu.RUnlock()

	for _, sdmd := range server.stackdriverMetricDescriptors {
		fn(sdmd)
	}
}

func (server *fakeMetricsServer) resetStackdriverTimeSeries() {
	server.mu.Lock()
	server.stackdriverTimeSeries = server.stackdriverTimeSeries[:0]
	server.mu.Unlock()
}

func (server *fakeMetricsServer) resetStackdriverMetricDescriptors() {
	server.mu.Lock()
	server.stackdriverMetricDescriptors = server.stackdriverMetricDescriptors[:0]
	server.mu.Unlock()
}

func (server *fakeMetricsServer) CreateMetricDescriptor(ctx context.Context, req *monitoringpb.CreateMetricDescriptorRequest) (*googlemetricpb.MetricDescriptor, error) {
	server.mu.Lock()
	server.stackdriverMetricDescriptors = append(server.stackdriverMetricDescriptors, req)
	server.mu.Unlock()
	return req.MetricDescriptor, nil
}

func (server *fakeMetricsServer) CreateTimeSeries(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest) (*empty.Empty, error) {
	server.mu.Lock()
	server.stackdriverTimeSeries = append(server.stackdriverTimeSeries, req)
	server.mu.Unlock()
	return new(empty.Empty), nil
}

func requireTimeSeriesRequestEqual(t *testing.T, got, want []*monitoringpb.CreateTimeSeriesRequest) (string, int) {
	diff := ""
	i := 0
	if len(got) != len(want) {
		diff = fmt.Sprintf("Unexpected slice len got: %d want: %d", len(got), len(want))
		return diff, i
	}
	for i, g := range got {
		w := want[i]
		diff = cmp.Diff(g, w, protocmp.Transform())
		if diff != "" {
			return diff, i
		}
	}
	return diff, i
}

func requireMetricDescriptorRequestEqual(t *testing.T, got, want []*monitoringpb.CreateMetricDescriptorRequest) (string, int) {
	diff := ""
	i := 0
	if len(got) != len(want) {
		diff = fmt.Sprintf("Unexpected slice len got: %d want: %d", len(got), len(want))
		return diff, i
	}
	for i, g := range got {
		w := want[i]
		diff = cmp.Diff(g, w, protocmp.Transform())
		if diff != "" {
			return diff, i
		}
	}
	return diff, i
}
