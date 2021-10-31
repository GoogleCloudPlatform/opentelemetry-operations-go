// Copyright 2020-2021 Google LLC
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
	"context"
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/metrictest"
	"go.opentelemetry.io/otel/metric/number"
	"go.opentelemetry.io/otel/metric/sdkapi"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/controller/basic"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/controller/controllertest"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/processor/processortest"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"github.com/googleinterns/cloud-operations-api-mock/cloudmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	googlelabelpb "google.golang.org/genproto/googleapis/api/label"
	googlemetricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	formatter = func(d *metric.Descriptor) string {
		return fmt.Sprintf("test.googleapis.com/%s", d.Name())
	}
)

func TestExportMetrics(t *testing.T) {
	ctx := context.Background()
	cloudMock := cloudmock.NewCloudMock()
	defer cloudMock.Shutdown()

	clientOpt := option.WithGRPCConn(cloudMock.ClientConn())
	res := &resource.Resource{}
	aggSel := processortest.AggregatorSelector()
	proc := processor.NewFactory(aggSel, export.CumulativeExportKindSelector())

	opts := []Option{
		WithProjectID("PROJECT_ID_NOT_REAL"),
		WithMonitoringClientOptions(clientOpt),
		WithMetricDescriptorTypeFormatter(formatter),
	}

	exporter, err := NewRawExporter(opts...)
	if err != nil {
		t.Errorf("Error occurred when creating exporter: %v", err)
	}
	cont := controller.New(proc,
		controller.WithExporter(exporter),
		controller.WithResource(res),
	)

	assert.NoError(t, cont.Start(ctx))
	meter := cont.Meter("test")
	counter := metric.Must(meter).NewInt64Counter("name.lastvalue")

	counter.Add(ctx, 1)
	require.NoError(t, cont.Stop(ctx))
}

func TestExportCounter(t *testing.T) {
	cloudMock := cloudmock.NewCloudMock()
	defer cloudMock.Shutdown()

	clientOpt := option.WithGRPCConn(cloudMock.ClientConn())

	resOpt := basic.WithResource(
		resource.NewWithAttributes(
			semconv.SchemaURL,
			attribute.String("test_id", "abc123"),
		),
	)

	pusher, err := InstallNewPipeline(
		[]Option{
			WithProjectID("PROJECT_ID_NOT_REAL"),
			WithMonitoringClientOptions(clientOpt),
			WithMetricDescriptorTypeFormatter(formatter),
		},
		resOpt,
	)
	assert.NoError(t, err)

	ctx := context.Background()
	defer pusher.Stop(ctx)

	// Start meter
	meter := pusher.Meter("cloudmonitoring/test")

	// Register counter value
	counter := metric.Must(meter).NewInt64Counter("counter-a")
	clabels := []attribute.KeyValue{attribute.Key("key").String("value")}
	counter.Add(ctx, 100, clabels...)
}

func TestDescToMetricType(t *testing.T) {
	inMe := []*metricExporter{
		{
			o: &options{},
		},
		{
			o: &options{
				MetricDescriptorTypeFormatter: formatter,
			},
		},
	}

	inDesc := []metric.Descriptor{
		metrictest.NewDescriptor("testing", sdkapi.HistogramInstrumentKind, number.Float64Kind),
		metrictest.NewDescriptor("test/of/path", sdkapi.HistogramInstrumentKind, number.Float64Kind),
	}

	wants := []string{
		"custom.googleapis.com/opentelemetry/testing",
		"test.googleapis.com/test/of/path",
	}

	for i, w := range wants {
		out := inMe[i].descToMetricType(&inDesc[i])
		if out != w {
			t.Errorf("expected: %v, actual: %v", w, out)
		}
	}
}

func TestRecordToMpb(t *testing.T) {
	ctx := context.Background()
	cloudMock := cloudmock.NewCloudMock()
	defer cloudMock.Shutdown()

	clientOpt := option.WithGRPCConn(cloudMock.ClientConn())

	randSource = rand.NewSource(0)

	counterName := "testing.sum"
	metricType := "test.googleapis.com/" + counterName
	// Generated string for the seed set above.
	uniqueIdentifier := "f1f85ff5"

	tests := []struct {
		name                 string
		useUniqueID          bool
		counterLabels        []attribute.KeyValue
		wantMetricDescriptor *googlemetricpb.MetricDescriptor
		wantMetric           *googlemetricpb.Metric
	}{
		{
			name:        "no useUniqueID",
			useUniqueID: false,
			counterLabels: []attribute.KeyValue{
				attribute.Key("a").String("A"),
			},
			wantMetricDescriptor: &googlemetricpb.MetricDescriptor{
				Name:       counterName,
				Type:       metricType,
				MetricKind: googlemetricpb.MetricDescriptor_CUMULATIVE,
				ValueType:  googlemetricpb.MetricDescriptor_INT64,
			},
			wantMetric: &googlemetricpb.Metric{
				Type: metricType,
				Labels: map[string]string{
					"a": "A",
				},
			},
		},
		{
			name:        "useUniqueID",
			useUniqueID: true,
			counterLabels: []attribute.KeyValue{
				attribute.Key("a").String("A"),
			},
			wantMetricDescriptor: &googlemetricpb.MetricDescriptor{
				Name:       counterName,
				Type:       metricType,
				MetricKind: googlemetricpb.MetricDescriptor_CUMULATIVE,
				ValueType:  googlemetricpb.MetricDescriptor_INT64,
				Labels: []*googlelabelpb.LabelDescriptor{{
					Key:       UniqueIdentifier,
					ValueType: googlelabelpb.LabelDescriptor_STRING,
				}},
			},
			wantMetric: &googlemetricpb.Metric{
				Type: metricType,
				Labels: map[string]string{
					"a":              "A",
					UniqueIdentifier: uniqueIdentifier,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := &resource.Resource{}
			aggSel := processortest.AggregatorSelector()
			proc := processor.NewFactory(aggSel, export.CumulativeExportKindSelector())

			exporterOpts := []Option{
				WithProjectID("PROJECT_ID_NOT_REAL"),
				WithMonitoringClientOptions(clientOpt),
				WithMetricDescriptorTypeFormatter(formatter),
			}
			if test.useUniqueID {
				exporterOpts = append(exporterOpts, WithUniqueID())
			}

			exporter, err := NewRawExporter(exporterOpts...)
			assert.NoError(t, err)

			cont := controller.New(proc,
				controller.WithExporter(exporter),
				controller.WithResource(res),
			)

			meter := cont.Meter("test")
			metric.Must(meter).NewInt64Counter(counterName).Add(ctx, 1, test.counterLabels...)

			require.NoError(t, cont.Collect(ctx))

			me := exporter.metricExporter

			var gotMD *googlemetricpb.MetricDescriptor
			var gotMetric *googlemetricpb.Metric
			aggError := controllertest.ReadAll(cont, export.CumulativeExportKindSelector(), func(library instrumentation.Library, r export.Record) error {
				gotMD = me.recordToMdpb(&r)
				gotMetric = me.recordToMpb(&r, library)
				return nil
			})
			require.NoError(t, aggError)

			if !reflect.DeepEqual(test.wantMetricDescriptor, gotMD) {
				t.Errorf("recordToMdpb expected: %v, got: %v", test.wantMetricDescriptor, gotMD)
			}
			if !reflect.DeepEqual(test.wantMetric, gotMetric) {
				t.Errorf("recordToMpb expected: %v, actual: %v", test.wantMetricDescriptor, gotMetric)
			}
		})
	}
}

func TestResourceToMonitoredResourcepb(t *testing.T) {
	testCases := []struct {
		resource       *resource.Resource
		expectedType   string
		expectedLabels map[string]string
	}{
		// k8s_container
		{
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.availability_zone", "us-central1-a"),
				attribute.String("k8s.cluster.name", "opentelemetry-cluster"),
				attribute.String("k8s.namespace.name", "default"),
				attribute.String("k8s.pod.name", "opentelemetry-pod-autoconf"),
				attribute.String("container.name", "opentelemetry"),
			),
			"k8s_container",
			map[string]string{
				"project_id":     "",
				"location":       "us-central1-a",
				"cluster_name":   "opentelemetry-cluster",
				"namespace_name": "default",
				"pod_name":       "opentelemetry-pod-autoconf",
				"container_name": "opentelemetry",
			},
		},
		// k8s_node
		{
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.availability_zone", "us-central1-a"),
				attribute.String("k8s.cluster.name", "opentelemetry-cluster"),
				attribute.String("host.name", "opentelemetry-node"),
			),
			"k8s_node",
			map[string]string{
				"project_id":   "",
				"location":     "us-central1-a",
				"cluster_name": "opentelemetry-cluster",
				"node_name":    "opentelemetry-node",
			},
		},
		// k8s_pod
		{
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.availability_zone", "us-central1-a"),
				attribute.String("k8s.cluster.name", "opentelemetry-cluster"),
				attribute.String("k8s.namespace.name", "default"),
				attribute.String("k8s.pod.name", "opentelemetry-pod-autoconf"),
			),
			"k8s_pod",
			map[string]string{
				"project_id":     "",
				"location":       "us-central1-a",
				"cluster_name":   "opentelemetry-cluster",
				"namespace_name": "default",
				"pod_name":       "opentelemetry-pod-autoconf",
			},
		},
		// k8s_cluster
		{
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.availability_zone", "us-central1-a"),
				attribute.String("k8s.cluster.name", "opentelemetry-cluster"),
			),
			"k8s_cluster",
			map[string]string{
				"project_id":   "",
				"location":     "us-central1-a",
				"cluster_name": "opentelemetry-cluster",
			},
		},
		// k8s_node missing a field
		{
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("k8s.cluster.name", "opentelemetry-cluster"),
				attribute.String("host.name", "opentelemetry-node"),
			),
			"global",
			map[string]string{
				"project_id": "",
			},
		},
		// nonexisting resource types
		{
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "none"),
				attribute.String("cloud.availability_zone", "us-central1-a"),
				attribute.String("k8s.cluster.name", "opentelemetry-cluster"),
				attribute.String("k8s.namespace.name", "default"),
				attribute.String("k8s.pod.name", "opentelemetry-pod-autoconf"),
				attribute.String("container.name", "opentelemetry"),
			),
			"global",
			map[string]string{
				"project_id": "",
			},
		},
		// GCE resource fields
		{
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("host.id", "123"),
				attribute.String("cloud.availability_zone", "us-central1-a"),
			),
			"gce_instance",
			map[string]string{
				"project_id":  "",
				"instance_id": "123",
				"zone":        "us-central1-a",
			},
		},
		// AWS resources
		{
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "aws"),
				attribute.String("cloud.region", "us-central1-a"),
				attribute.String("host.id", "123"),
				attribute.String("cloud.account.id", "fake_account"),
			),
			"aws_ec2_instance",
			map[string]string{
				"project_id":  "",
				"instance_id": "123",
				"region":      "us-central1-a",
				"aws_account": "fake_account",
			},
		},
		// Cloud Run
		{
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.region", "utopia"),
				attribute.String("service.instance.id", "bar"),
				attribute.String("service.name", "x-service"),
				attribute.String("service.namespace", "cloud-run-managed"),
			),
			"generic_task",
			map[string]string{
				"project_id": "",
				"location":   "utopia",
				"namespace":  "cloud-run-managed",
				"job":        "x-service",
				"task_id":    "bar",
			},
		},
	}

	desc := metrictest.NewDescriptor("testing", sdkapi.HistogramInstrumentKind, number.Float64Kind)

	md := &googlemetricpb.MetricDescriptor{
		Name:        desc.Name(),
		Type:        fmt.Sprintf(cloudMonitoringMetricDescriptorNameFormat, desc.Name()),
		MetricKind:  googlemetricpb.MetricDescriptor_GAUGE,
		ValueType:   googlemetricpb.MetricDescriptor_DOUBLE,
		Description: "test",
	}

	mdkey := key{
		name:        md.Name,
		libraryname: "",
	}

	me := &metricExporter{
		o: &options{},
		mdCache: map[key]*googlemetricpb.MetricDescriptor{
			mdkey: md,
		},
	}

	for _, test := range testCases {
		t.Run(test.resource.String(), func(t *testing.T) {
			got := me.resourceToMonitoredResourcepb(test.resource)
			if !reflect.DeepEqual(got.GetLabels(), test.expectedLabels) {
				t.Errorf("expected: %v, actual: %v", test.expectedLabels, got.GetLabels())
			}
			if got.GetType() != test.expectedType {
				t.Errorf("expected: %v, actual: %v", test.expectedType, got.GetType())
			}
		})
	}
}

func TestGenerateResLabelMap(t *testing.T) {
	testCases := []struct {
		resource       *resource.Resource
		expectedLabels map[string]string
	}{
		// Bool attribute
		{
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.Bool("key", true),
			),
			map[string]string{
				"key": "true",
			},
		},
		// Int attribute
		{
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.Int("key", 1),
			),
			map[string]string{
				"key": "1",
			},
		},
		// Int64 attribute
		{
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.Int64("key", 1),
			),
			map[string]string{
				"key": "1",
			},
		},
		// Float64 attribute
		{
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.Float64("key", 1.3),
			),
			map[string]string{
				"key": "1.3",
			},
		},
		// String attribute
		{
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("key", "str"),
			),
			map[string]string{
				"key": "str",
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.resource.String(), func(t *testing.T) {
			got := generateResLabelMap(test.resource)
			if !reflect.DeepEqual(got, test.expectedLabels) {
				t.Errorf("expected: %v, actual: %v", test.expectedLabels, got)
			}
		})
	}
}

func TestTimeIntervalStaggering(t *testing.T) {
	var tm time.Time

	interval, err := toNonemptyTimeIntervalpb(tm, tm)
	if err != nil {
		t.Fatalf("conversion to PB failed: %v", err)
	}

	if err := interval.StartTime.CheckValid(); err != nil {
		t.Fatalf("unable to convert start time from PB: %v", err)
	}
	start := interval.StartTime.AsTime()

	if err := interval.EndTime.CheckValid(); err != nil {
		t.Fatalf("unable to convert end time to PB: %v", err)
	}
	end := interval.EndTime.AsTime()

	if end.Before(start.Add(time.Millisecond)) {
		t.Fatalf("expected end=%v to be at least %v after start=%v, but it wasn't", end, time.Millisecond, start)
	}
}

func TestTimeIntervalPassthru(t *testing.T) {
	var tm time.Time

	interval, err := toNonemptyTimeIntervalpb(tm, tm.Add(time.Second))
	if err != nil {
		t.Fatalf("conversion to PB failed: %v", err)
	}

	if err := interval.StartTime.CheckValid(); err != nil {
		t.Fatalf("unable to convert start time from PB: %v", err)
	}
	start := interval.StartTime.AsTime()

	if err := interval.EndTime.CheckValid(); err != nil {
		t.Fatalf("unable to convert end time to PB: %v", err)
	}
	end := interval.EndTime.AsTime()

	assert.Equal(t, start, tm)
	assert.Equal(t, end, tm.Add(time.Second))
}

type mock struct {
	monitoringpb.UnimplementedMetricServiceServer
	createTimeSeries       func(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest) (*emptypb.Empty, error)
	createMetricDescriptor func(ctx context.Context, req *monitoringpb.CreateMetricDescriptorRequest) (*googlemetricpb.MetricDescriptor, error)
}

func (m *mock) CreateTimeSeries(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest) (*emptypb.Empty, error) {
	return m.createTimeSeries(ctx, req)
}

func (m *mock) CreateMetricDescriptor(ctx context.Context, req *monitoringpb.CreateMetricDescriptorRequest) (*googlemetricpb.MetricDescriptor, error) {
	return m.createMetricDescriptor(ctx, req)
}

func TestExportMetricsWithUserAgent(t *testing.T) {
	server := grpc.NewServer()
	t.Cleanup(server.Stop)

	// Channel to shove user agent strings from createTimeSeries
	ch := make(chan []string, 1)

	m := mock{
		createTimeSeries: func(ctx context.Context, r *monitoringpb.CreateTimeSeriesRequest) (*emptypb.Empty, error) {
			md, _ := metadata.FromIncomingContext(ctx)
			ch <- md.Get("User-Agent")
			return &emptypb.Empty{}, nil
		},
		createMetricDescriptor: func(ctx context.Context, req *monitoringpb.CreateMetricDescriptorRequest) (*googlemetricpb.MetricDescriptor, error) {
			md, _ := metadata.FromIncomingContext(ctx)
			ch <- md.Get("User-Agent")
			return req.MetricDescriptor, nil
		},
	}
	// Make sure all the calls have the right user agents.
	// We have to run this in parallel because BOTH calls happen seamlessly when exporting metrics.
	go func() {
		for {
			ua := <-ch
			require.Regexp(t, "opentelemetry-go .*; google-cloud-metric-exporter .*", ua[0])
		}
	}()
	monitoringpb.RegisterMetricServiceServer(server, &m)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go server.Serve(lis)

	clientOpts := []option.ClientOption{
		option.WithEndpoint(lis.Addr().String()),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithInsecure()),
	}
	res := &resource.Resource{}
	aggSel := processortest.AggregatorSelector()
	proc := processor.NewFactory(aggSel, export.CumulativeExportKindSelector())
	ctx := context.Background()

	opts := []Option{
		WithProjectID("PROJECT_ID_NOT_REAL"),
		WithMonitoringClientOptions(clientOpts...),
		WithMetricDescriptorTypeFormatter(formatter),
	}

	exporter, err := NewRawExporter(opts...)
	if err != nil {
		t.Errorf("Error occurred when creating exporter: %v", err)
	}
	cont := controller.New(proc,
		controller.WithExporter(exporter),
		controller.WithResource(res),
	)

	assert.NoError(t, cont.Start(ctx))
	meter := cont.Meter("test")

	counter := metric.Must(meter).NewInt64Counter("name.lastvalue")

	counter.Add(ctx, 1)
	require.NoError(t, cont.Stop(ctx))

	// User agent checking happens above in parallel to this flow.
}
