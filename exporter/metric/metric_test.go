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
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/genproto/googleapis/api/label"
	googlemetricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock"
)

var (
	formatter = func(d metricdata.Metrics) string {
		return fmt.Sprintf("test.googleapis.com/%s", d.Name)
	}
)

func TestExportMetrics(t *testing.T) {
	ctx := context.Background()
	testServer, err := cloudmock.NewMetricTestServer()
	go testServer.Serve()
	defer testServer.Shutdown()
	assert.NoError(t, err)

	clientOpts := []option.ClientOption{
		option.WithEndpoint(testServer.Endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}
	res := &resource.Resource{}

	opts := []Option{
		WithProjectID("PROJECT_ID_NOT_REAL"),
		WithMonitoringClientOptions(clientOpts...),
		WithMetricDescriptorTypeFormatter(formatter),
	}

	exporter, err := New(opts...)
	if err != nil {
		t.Errorf("Error occurred when creating exporter: %v", err)
	}
	provider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(exporter)),
		metric.WithResource(res),
	)

	meter := provider.Meter("test")
	counter, err := meter.SyncInt64().Counter("name.lastvalue")
	require.NoError(t, err)

	counter.Add(ctx, 1)
	require.NoError(t, provider.Shutdown(ctx))
}

func TestExportCounter(t *testing.T) {
	testServer, err := cloudmock.NewMetricTestServer()
	go testServer.Serve()
	defer testServer.Shutdown()
	assert.NoError(t, err)

	clientOpts := []option.ClientOption{
		option.WithEndpoint(testServer.Endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	exporter, err := New(
		WithProjectID("PROJECT_ID_NOT_REAL"),
		WithMonitoringClientOptions(clientOpts...),
		WithMetricDescriptorTypeFormatter(formatter),
	)
	assert.NoError(t, err)
	provider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(exporter)),
		metric.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("test_id", "abc123"),
			)),
	)

	ctx := context.Background()
	defer provider.Shutdown(ctx)

	// Start meter
	meter := provider.Meter("cloudmonitoring/test")

	// Register counter value
	counter, err := meter.SyncInt64().Counter("counter-a")
	assert.NoError(t, err)
	clabels := []attribute.KeyValue{attribute.Key("key").String("value")}
	counter.Add(ctx, 100, clabels...)
}

func TestExportHistogram(t *testing.T) {
	testServer, err := cloudmock.NewMetricTestServer()
	go testServer.Serve()
	defer testServer.Shutdown()
	assert.NoError(t, err)

	clientOpts := []option.ClientOption{
		option.WithEndpoint(testServer.Endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	exporter, err := New(
		WithProjectID("PROJECT_ID_NOT_REAL"),
		WithMonitoringClientOptions(clientOpts...),
		WithMetricDescriptorTypeFormatter(formatter),
	)
	assert.NoError(t, err)
	provider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(exporter)),
		metric.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("test_id", "abc123"),
			),
		),
	)
	assert.NoError(t, err)

	ctx := context.Background()
	defer provider.Shutdown(ctx)

	// Start meter
	meter := provider.Meter("cloudmonitoring/test")

	// Register counter value
	counter, err := meter.SyncInt64().Histogram("counter-a")
	assert.NoError(t, err)
	clabels := []attribute.KeyValue{attribute.Key("key").String("value")}
	counter.Record(ctx, 100, clabels...)
	counter.Record(ctx, 50, clabels...)
	counter.Record(ctx, 200, clabels...)
}

func TestDescToMetricType(t *testing.T) {
	inMe := []*metricExporter{
		{
			o: &options{},
		},
		{
			o: &options{
				metricDescriptorTypeFormatter: formatter,
			},
		},
	}

	inDesc := []metricdata.Metrics{
		{Name: "testing", Data: metricdata.Histogram{}},
		{Name: "test/of/path", Data: metricdata.Histogram{}},
	}

	wants := []string{
		"workload.googleapis.com/testing",
		"test.googleapis.com/test/of/path",
	}

	for i, w := range wants {
		out := inMe[i].descToMetricType(inDesc[i])
		if out != w {
			t.Errorf("expected: %v, actual: %v", w, out)
		}
	}
}

func TestRecordToMpb(t *testing.T) {
	metricName := "testing"

	md := &googlemetricpb.MetricDescriptor{
		Name:        metricName,
		Type:        fmt.Sprintf(cloudMonitoringMetricDescriptorNameFormat, metricName),
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

	inputLibrary := instrumentation.Library{Name: "workload.googleapis.com"}
	inputAttributes := attribute.NewSet(
		attribute.Key("a").String("A"),
		attribute.Key("b?b").String("B"),
		attribute.Key("8foo").Int64(100),
	)
	inputMetrics := metricdata.Metrics{
		Name: metricName,
	}
	inputExtraLabels := attribute.NewSet(
		attribute.Key("service.name").String("servicename"),
		attribute.Key("service.namespace").String("servicenamespace"),
		attribute.Key("service.instance.id").String("23490238490235gfdg87g"),
	)

	want := &googlemetricpb.Metric{
		Type: md.Type,
		Labels: map[string]string{
			"a":                   "A",
			"b_b":                 "B",
			"key_8foo":            "100",
			"service_instance_id": "23490238490235gfdg87g",
			"service_name":        "servicename",
			"service_namespace":   "servicenamespace",
		},
	}
	out := me.recordToMpb(inputMetrics, inputAttributes, inputLibrary, &inputExtraLabels)
	if !reflect.DeepEqual(want, out) {
		t.Errorf("expected: %v, actual: %v", want, out)
	}
}

func TestExtraLabelsFromResource(t *testing.T) {
	serviceLabelsSet := attribute.NewSet(
		semconv.ServiceNameKey.String("myservicename"),
		semconv.ServiceNamespaceKey.String("myservicenamespace"),
		semconv.ServiceInstanceIDKey.String("123456789"),
	)
	allLabelsSet := attribute.NewSet(
		semconv.ServiceNameKey.String("myservicename"),
		semconv.ServiceNamespaceKey.String("myservicenamespace"),
		semconv.ServiceInstanceIDKey.String("123456789"),
		semconv.CloudProviderKey.String("gcp"),
	)
	for _, tc := range []struct {
		input                   *resource.Resource
		expected                *attribute.Set
		resourceAttributeFilter attribute.Filter
		desc                    string
	}{
		{
			desc:                    "empty resource",
			resourceAttributeFilter: DefaultResourceAttributesFilter,
			expected:                attribute.EmptySet(),
		},
		{
			desc:                    "service labels added",
			resourceAttributeFilter: DefaultResourceAttributesFilter,
			input: resource.NewSchemaless(
				semconv.ServiceNameKey.String("myservicename"),
				semconv.ServiceNamespaceKey.String("myservicenamespace"),
				semconv.ServiceInstanceIDKey.String("123456789"),
			),
			expected: &serviceLabelsSet,
		},
		{
			desc:                    "non-service labels ignored",
			resourceAttributeFilter: DefaultResourceAttributesFilter,
			input: resource.NewSchemaless(
				semconv.ServiceNameKey.String("myservicename"),
				semconv.ServiceNamespaceKey.String("myservicenamespace"),
				semconv.ServiceInstanceIDKey.String("123456789"),
				semconv.CloudProviderKey.String("gcp"),
			),
			expected: &serviceLabelsSet,
		},
		{
			desc:                    "all labels with custom filter",
			resourceAttributeFilter: func(attribute.KeyValue) bool { return true },
			input: resource.NewSchemaless(
				semconv.ServiceNameKey.String("myservicename"),
				semconv.ServiceNamespaceKey.String("myservicenamespace"),
				semconv.ServiceInstanceIDKey.String("123456789"),
				semconv.CloudProviderKey.String("gcp"),
			),
			expected: &allLabelsSet,
		},
		{
			desc:                    "service labels disabled",
			resourceAttributeFilter: NoAttributes,
			input: resource.NewSchemaless(
				semconv.ServiceNameKey.String("myservicename"),
				semconv.ServiceNamespaceKey.String("myservicenamespace"),
				semconv.ServiceInstanceIDKey.String("123456789"),
				semconv.CloudProviderKey.String("gcp"),
			),
			expected: attribute.EmptySet(),
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			me := &metricExporter{
				o: &options{
					resourceAttributeFilter: tc.resourceAttributeFilter,
				},
			}
			actual := me.extraLabelsFromResource(tc.input)
			if !tc.expected.Equals(actual) {
				t.Errorf("expected: %v, actual: %v", tc.expected, actual)
			}

		})
	}
}

func TestRecordToMdpb(t *testing.T) {
	metricName := "testing"

	want := &googlemetricpb.MetricDescriptor{
		Name:        metricName,
		DisplayName: metricName,
		Type:        fmt.Sprintf(cloudMonitoringMetricDescriptorNameFormat, metricName),
		MetricKind:  googlemetricpb.MetricDescriptor_GAUGE,
		ValueType:   googlemetricpb.MetricDescriptor_DOUBLE,
		Description: "test",
		Labels: []*label.LabelDescriptor{
			{Key: normalizeLabelKey("service.instance.id")},
			{Key: normalizeLabelKey("service.name")},
			{Key: normalizeLabelKey("service.namespace")},
			{Key: normalizeLabelKey("a")},
			{Key: normalizeLabelKey("b.b")},
			{Key: normalizeLabelKey("foo")},
		},
	}

	mdkey := key{
		name:        want.Name,
		libraryname: "",
	}
	me := &metricExporter{
		o: &options{},
		mdCache: map[key]*googlemetricpb.MetricDescriptor{
			mdkey: want,
		},
	}
	inputMetrics := metricdata.Metrics{
		Name:        metricName,
		Description: "test",
		Data: metricdata.Sum[float64]{
			IsMonotonic: false,
			DataPoints: []metricdata.DataPoint[float64]{
				{
					Attributes: attribute.NewSet(
						attribute.Key("a").String("A"),
						attribute.Key("b_b").String("B"),
						attribute.Key("foo").Int64(100),
					),
				},
			},
		},
	}
	inputExtraLabels := attribute.NewSet(
		attribute.Key("service.name").String("servicename"),
		attribute.Key("service.namespace").String("servicenamespace"),
		attribute.Key("service.instance.id").String("23490238490235gfdg87g"),
	)
	out := me.recordToMdpb(inputMetrics, &inputExtraLabels)
	if !reflect.DeepEqual(want, out) {
		t.Errorf("expected: %v, actual: %v", want, out)
	}
}

func TestResourceToMonitoredResourcepb(t *testing.T) {
	testCases := []struct {
		desc           string
		resource       *resource.Resource
		expectedLabels map[string]string
		expectedType   string
	}{
		{
			desc: "k8s_container success",
			resource: resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.platform", "gcp_kubernetes_engine"),
				attribute.String("cloud.availability_zone", "us-central1-a"),
				attribute.String("k8s.cluster.name", "opentelemetry-cluster"),
				attribute.String("k8s.namespace.name", "default"),
				attribute.String("k8s.pod.name", "opentelemetry-pod-autoconf"),
				attribute.String("k8s.container.name", "opentelemetry"),
			),
			expectedType: "k8s_container",
			expectedLabels: map[string]string{
				"location":       "us-central1-a",
				"cluster_name":   "opentelemetry-cluster",
				"namespace_name": "default",
				"pod_name":       "opentelemetry-pod-autoconf",
				"container_name": "opentelemetry",
			},
		},
		{
			desc: "k8s_node success",
			resource: resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.platform", "gcp_kubernetes_engine"),
				attribute.String("cloud.availability_zone", "us-central1-a"),
				attribute.String("k8s.cluster.name", "opentelemetry-cluster"),
				attribute.String("k8s.node.name", "opentelemetry-node"),
			),
			expectedType: "k8s_node",
			expectedLabels: map[string]string{
				"location":     "us-central1-a",
				"cluster_name": "opentelemetry-cluster",
				"node_name":    "opentelemetry-node",
			},
		},
		{
			desc: "k8s_pod success",
			resource: resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.platform", "gcp_kubernetes_engine"),
				attribute.String("cloud.availability_zone", "us-central1-a"),
				attribute.String("k8s.cluster.name", "opentelemetry-cluster"),
				attribute.String("k8s.namespace.name", "default"),
				attribute.String("k8s.pod.name", "opentelemetry-pod-autoconf"),
			),
			expectedType: "k8s_pod",
			expectedLabels: map[string]string{
				"location":       "us-central1-a",
				"cluster_name":   "opentelemetry-cluster",
				"namespace_name": "default",
				"pod_name":       "opentelemetry-pod-autoconf",
			},
		},
		{
			desc: "k8s_cluster success",
			resource: resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.platform", "gcp_kubernetes_engine"),
				attribute.String("cloud.availability_zone", "us-central1-a"),
				attribute.String("k8s.cluster.name", "opentelemetry-cluster"),
			),
			expectedType: "k8s_cluster",
			expectedLabels: map[string]string{
				"location":     "us-central1-a",
				"cluster_name": "opentelemetry-cluster",
			},
		},
		{
			desc: "nonexisting resource types",
			resource: resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "none"),
				attribute.String("cloud.platform", "gcp_foobar"),
				attribute.String("cloud.availability_zone", "us-central1-a"),
				attribute.String("k8s.cluster.name", "opentelemetry-cluster"),
				attribute.String("k8s.namespace.name", "default"),
				attribute.String("k8s.pod.name", "opentelemetry-pod-autoconf"),
				attribute.String("k8s.container.name", "opentelemetry"),
			),
			expectedType: "generic_node",
			expectedLabels: map[string]string{
				"location":  "us-central1-a",
				"namespace": "",
				"node_id":   "",
			},
		},
		{
			desc: "GCE instance",
			resource: resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.platform", "gcp_compute_engine"),
				attribute.String("host.id", "123"),
				attribute.String("cloud.availability_zone", "us-central1-a"),
			),
			expectedType: "gce_instance",
			expectedLabels: map[string]string{
				"instance_id": "123",
				"zone":        "us-central1-a",
			},
		},
		{
			desc: "AWS resources",
			resource: resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "aws"),
				attribute.String("cloud.platform", "aws_ec2"),
				attribute.String("cloud.region", "us-central1-a"),
				attribute.String("host.id", "123"),
				attribute.String("cloud.account.id", "fake_account"),
			),
			expectedType: "aws_ec2_instance",
			expectedLabels: map[string]string{
				"instance_id": "123",
				"region":      "us-central1-a",
				"aws_account": "fake_account",
			},
		},
		{
			desc: "Cloud Run",
			resource: resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.platform", "gcp_cloud_run"),
				attribute.String("cloud.region", "utopia"),
				attribute.String("service.instance.id", "bar"),
				attribute.String("service.name", "x-service"),
				attribute.String("service.namespace", "cloud-run-managed"),
			),
			expectedType: "generic_task",
			expectedLabels: map[string]string{
				"location":  "utopia",
				"namespace": "cloud-run-managed",
				"job":       "x-service",
				"task_id":   "bar",
			},
		},
		{
			desc: "GCE instance invalid utf8",
			resource: resource.NewWithAttributes(
				semconv.SchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.platform", "gcp_compute_engine"),
				attribute.String("host.id", invalidUtf8SequenceID),
				attribute.String("cloud.availability_zone", "us-central1-a"),
			),
			expectedType: "gce_instance",
			expectedLabels: map[string]string{
				"instance_id": "�",
				"zone":        "us-central1-a",
			},
		},
	}

	md := &googlemetricpb.MetricDescriptor{
		Name:        "testing",
		Type:        fmt.Sprintf(cloudMonitoringMetricDescriptorNameFormat, "testing"),
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
		t.Run(test.desc, func(t *testing.T) {
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

var (
	invalidUtf8TwoOctet   = string([]byte{0xc3, 0x28}) // Invalid 2-octet sequence
	invalidUtf8SequenceID = string([]byte{0xa0, 0xa1}) // Invalid sequence identifier
)

func TestRecordToMpbUTF8(t *testing.T) {
	metricName := "testing"

	md := &googlemetricpb.MetricDescriptor{
		Name:        metricName,
		Type:        fmt.Sprintf(cloudMonitoringMetricDescriptorNameFormat, metricName),
		MetricKind:  googlemetricpb.MetricDescriptor_GAUGE,
		ValueType:   googlemetricpb.MetricDescriptor_DOUBLE,
		Description: "test",
	}

	mdkey := key{
		name:        md.Name,
		libraryname: "",
	}

	expectedLabels := map[string]string{
		"valid_ascii":         "abcdefg",
		"valid_utf8":          "שלום",
		"invalid_two_octet":   "�(",
		"invalid_sequence_id": "�",
	}

	me := &metricExporter{
		o: &options{},
		mdCache: map[key]*googlemetricpb.MetricDescriptor{
			mdkey: md,
		},
	}

	inputLibrary := instrumentation.Library{Name: "workload.googleapis.com"}
	inputAttributes := attribute.NewSet(
		attribute.Key("valid_ascii").String("abcdefg"),
		attribute.Key("valid_utf8").String("שלום"),
		attribute.Key("invalid_two_octet").String(invalidUtf8TwoOctet),
		attribute.Key("invalid_sequence_id").String(invalidUtf8SequenceID))
	inputMetrics := metricdata.Metrics{
		Name: metricName,
	}

	want := &googlemetricpb.Metric{
		Type:   md.Type,
		Labels: expectedLabels,
	}
	out := me.recordToMpb(inputMetrics, inputAttributes, inputLibrary, attribute.EmptySet())
	if !reflect.DeepEqual(want, out) {
		t.Errorf("expected: %v, actual: %v", want, out)
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

func TestExportWithDisableCreateMetricDescriptors(t *testing.T) {
	for _, tc := range []struct {
		desc                           string
		disableCreateMetricsDescriptor bool
		expectExportMetricDescriptor   bool
	}{
		{
			desc:                           "default",
			disableCreateMetricsDescriptor: false,
			expectExportMetricDescriptor:   true,
		},
		{
			desc:                           "Disable CreateMetricsDescriptor",
			disableCreateMetricsDescriptor: true,
			expectExportMetricDescriptor:   false,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			server := grpc.NewServer()

			metricDescriptorCreated := false

			m := mock{
				createTimeSeries: func(ctx context.Context, r *monitoringpb.CreateTimeSeriesRequest) (*emptypb.Empty, error) {
					return &emptypb.Empty{}, nil
				},
				createMetricDescriptor: func(ctx context.Context, req *monitoringpb.CreateMetricDescriptorRequest) (*googlemetricpb.MetricDescriptor, error) {
					metricDescriptorCreated = true
					return req.MetricDescriptor, nil
				},
			}

			monitoringpb.RegisterMetricServiceServer(server, &m)

			lis, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			go server.Serve(lis)

			clientOpts := []option.ClientOption{
				option.WithEndpoint(lis.Addr().String()),
				option.WithoutAuthentication(),
				option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
			}
			res := &resource.Resource{}
			ctx := context.Background()

			opts := []Option{
				WithProjectID("PROJECT_ID_NOT_REAL"),
				WithMonitoringClientOptions(clientOpts...),
				WithMetricDescriptorTypeFormatter(formatter),
			}

			if tc.disableCreateMetricsDescriptor {
				opts = append(opts, WithDisableCreateMetricDescriptors())
			}

			exporter, err := New(opts...)
			if err != nil {
				t.Errorf("Error occurred when creating exporter: %v", err)
			}
			provider := metric.NewMeterProvider(
				metric.WithReader(metric.NewPeriodicReader(exporter)),
				metric.WithResource(res),
			)

			meter := provider.Meter("test")

			counter, err := meter.SyncInt64().Counter("name.lastvalue")
			require.NoError(t, err)

			counter.Add(ctx, 1)
			require.NoError(t, provider.ForceFlush(ctx))
			server.Stop()
			require.Equal(t, tc.expectExportMetricDescriptor, metricDescriptorCreated)
		})
	}
}

func TestExportMetricsWithUserAgent(t *testing.T) {
	for _, tc := range []struct {
		desc                   string
		expectedUserAgentRegex string
		extraOpts              []option.ClientOption
	}{
		{
			desc:                   "default",
			expectedUserAgentRegex: "opentelemetry-go .*; google-cloud-metric-exporter .*",
		},
		{
			desc:                   "override user agent",
			extraOpts:              []option.ClientOption{option.WithUserAgent("test-user-agent")},
			expectedUserAgentRegex: "test-user-agent",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
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
					require.Regexp(t, tc.expectedUserAgentRegex, ua[0])
				}
			}()
			monitoringpb.RegisterMetricServiceServer(server, &m)

			lis, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			go server.Serve(lis)

			clientOpts := []option.ClientOption{
				option.WithEndpoint(lis.Addr().String()),
				option.WithoutAuthentication(),
				option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
			}
			clientOpts = append(clientOpts, tc.extraOpts...)
			res := &resource.Resource{}
			ctx := context.Background()

			opts := []Option{
				WithProjectID("PROJECT_ID_NOT_REAL"),
				WithMonitoringClientOptions(clientOpts...),
				WithMetricDescriptorTypeFormatter(formatter),
			}

			exporter, err := New(opts...)
			if err != nil {
				t.Errorf("Error occurred when creating exporter: %v", err)
			}
			provider := metric.NewMeterProvider(
				metric.WithReader(metric.NewPeriodicReader(exporter)),
				metric.WithResource(res),
			)

			meter := provider.Meter("test")

			counter, err := meter.SyncInt64().Counter("name.lastvalue")
			require.NoError(t, err)

			counter.Add(ctx, 1)
			require.NoError(t, provider.ForceFlush(ctx))

			// User agent checking happens above in parallel to this flow.
		})
	}
}

func TestConcurrentCallsAfterShutdown(t *testing.T) {
	testServer, err := cloudmock.NewMetricTestServer()
	go testServer.Serve()
	defer testServer.Shutdown()
	assert.NoError(t, err)

	clientOpts := []option.ClientOption{
		option.WithEndpoint(testServer.Endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	exporter, err := New(
		WithProjectID("PROJECT_ID_NOT_REAL"),
		WithMonitoringClientOptions(clientOpts...),
		WithMetricDescriptorTypeFormatter(formatter),
	)
	assert.NoError(t, err)

	ctx := context.Background()
	err = exporter.Shutdown(ctx)
	assert.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		err := exporter.Shutdown(ctx)
		assert.ErrorIs(t, err, errShutdown)
		wg.Done()
	}()
	go func() {
		err := exporter.ForceFlush(ctx)
		assert.NoError(t, err)
		wg.Done()
	}()
	go func() {
		err := exporter.Export(ctx, metricdata.ResourceMetrics{})
		assert.ErrorIs(t, err, errShutdown)
		wg.Done()
	}()

	wg.Wait()
}

func TestConcurrentExport(t *testing.T) {
	testServer, err := cloudmock.NewMetricTestServer()
	go testServer.Serve()
	defer testServer.Shutdown()
	assert.NoError(t, err)

	clientOpts := []option.ClientOption{
		option.WithEndpoint(testServer.Endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	exporter, err := New(
		WithProjectID("PROJECT_ID_NOT_REAL"),
		WithMonitoringClientOptions(clientOpts...),
		WithMetricDescriptorTypeFormatter(formatter),
	)
	assert.NoError(t, err)

	ctx := context.Background()
	defer func() {
		err := exporter.Shutdown(ctx)
		assert.NoError(t, err)
	}()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		err := exporter.Export(ctx, metricdata.ResourceMetrics{
			ScopeMetrics: []metricdata.ScopeMetrics{
				{
					Metrics: []metricdata.Metrics{
						{Name: "testing", Data: metricdata.Histogram{}},
						{Name: "test/of/path", Data: metricdata.Histogram{}},
					},
				},
			},
		})
		assert.NoError(t, err)
		wg.Done()
	}()
	go func() {
		err := exporter.Export(ctx, metricdata.ResourceMetrics{
			ScopeMetrics: []metricdata.ScopeMetrics{
				{
					Metrics: []metricdata.Metrics{
						{Name: "testing", Data: metricdata.Histogram{}},
						{Name: "test/of/path", Data: metricdata.Histogram{}},
					},
				},
			},
		})
		assert.NoError(t, err)
		wg.Done()
	}()

	wg.Wait()
}

func TestMetricTypeToDisplayName(t *testing.T) {
	for _, tc := range []struct {
		desc     string
		input    string
		expected string
	}{
		{desc: "empty input"},
		{
			desc:     "default prefix",
			input:    "workload.googleapis.com/MyCoolMetric",
			expected: "MyCoolMetric",
		},
		{
			desc:     "no prefix",
			input:    "helloworld",
			expected: "helloworld",
		},
		{
			desc:     "other prefix",
			input:    "custom.googleapis.com/MyCoolMetric",
			expected: "MyCoolMetric",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.expected, metricTypeToDisplayName(tc.input))
		})
	}
}
