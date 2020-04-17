// Copyright 2018, OpenCensus Authors
//           2020, Google Cloud Operations Exporter Authors
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

package metrics

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	_ "sync"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	monitoringapi "cloud.google.com/go/monitoring/apiv3"
	"go.opencensus.io/metric/metricdata"
	_ "go.opencensus.io/metric/metricexport"
	"go.opentelemetry.io/otel/api/core"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/metric"
	otel "go.opentelemetry.io/otel/sdk"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregator"
	"go.opentelemetry.io/otel/sdk/metric/batcher/defaultkeys"
	"go.opentelemetry.io/otel/sdk/metric/controller/push"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"google.golang.org/api/option"
)

const (
	// maxTimeSeriesPerUpload    = 200
	operationsTaskKey         = "operations_task"
	operationsTaskDescription = "Operations task identifier"
	// defaultDisplayNamePrefix  = "CloudMonitoring"
	version = "0.2.3" // TODO(ymotongpoo): Sort out how to keep up with the release version automatically.
)

var userAgent = fmt.Sprintf("opentelemetry-go %s; metrics-exporter %s", otel.Version(), version)

// metricsExporter exports OTel metrics to the Google Cloud Monitoring.
type metricsExporter struct {
	o *options

	// protoMu                sync.Mutex
	protoMetricDescriptors map[string]bool // Metric descriptors that were already created remotely

	// metricMu          sync.Mutex
	metricDescriptors map[string]bool // Metric descriptors that were already created remotely

	client        *monitoringapi.MetricClient
	defaultLabels map[string]labelValue
	// ir            *metricexport.IntervalReader

	// initReaderOnce sync.Once
}

var (
	errBlankProjectID = errors.New("expecting a non-blank ProjectID")
)

// InstallNewPipeline instantiates a NewExportPipeline and registers it globally.
func InstallNewPipeline(opts ...Option) (*push.Controller, error) {
	pusher, err := NewExportPipeline(opts...)
	if err != nil {
		return nil, err
	}
	global.SetMeterProvider(pusher)
	return pusher, err
}

// NewExportPipeline sets up a complete export pipeline with the recommended setup,
// chaining a NewRawExporter into the recommended selectors and batchers.
func NewExportPipeline(opts ...Option) (*push.Controller, error) {
	selector := simple.NewWithExactMeasure()
	exporter, err := NewRawExporter()
	if err != nil {
		return nil, err
	}
	batcher := defaultkeys.New(selector, export.NewDefaultLabelEncoder(), true)
	period := exporter.metricsExporter.o.ReportingInterval
	pusher := push.New(batcher, exporter, period)
	return pusher, nil
}

// newMetricsExporter returns an exporter that uploads stats data to Google Cloud Monitoring.
// Only one Cloud Monitoring exporter should be created per ProjectID per process, any subsequent
// invocations of NewExporter with the same ProjectID will return an error.
func newMetricsExporter(o *options) (*metricsExporter, error) {
	if strings.TrimSpace(o.ProjectID) == "" {
		return nil, errBlankProjectID
	}

	opts := append(o.MonitoringClientOptions, option.WithUserAgent(userAgent))
	ctx := o.Context
	if ctx == nil {
		ctx = context.Background()
	}
	client, err := monitoring.NewMetricClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	e := &metricsExporter{
		client:                 client,
		o:                      o,
		protoMetricDescriptors: make(map[string]bool),
		metricDescriptors:      make(map[string]bool),
	}

	var defaultLablesNotSanitized map[string]labelValue
	if o.DefaultMonitoringLabels != nil {
		defaultLablesNotSanitized = o.DefaultMonitoringLabels.m
	} else {
		defaultLablesNotSanitized = map[string]labelValue{
			operationsTaskKey: {val: getTaskValue(), desc: operationsTaskDescription},
		}
	}

	e.defaultLabels = make(map[string]labelValue)
	// Fill in the defaults firstly, irrespective of if the labelKeys and labelValues are mismatched.
	for key, label := range defaultLablesNotSanitized {
		e.defaultLabels[sanitize(key)] = label
	}

	return e, nil
}

// ExportMetrics exports OpenTelemetry Metrics to Google Cloud Monitoring.
func (me *metricsExporter) ExportMetrics(ctx context.Context, checkpointSet export.CheckpointSet) error {
	var aggError error
	aggError = checkpointSet.ForEach(func(record export.Record) error {
		desc := record.Descriptor()
		agg := record.Aggregator()
		kind := desc.NumberKind()

		if sum, ok := agg.(aggregator.Sum); ok {
			if err := me.exportSum(sum, kind, desc, []string{}); err != nil {
				return fmt.Errorf("exporting sum: %w", err)
			}
		} else if count, ok := agg.(aggregator.Count); ok {
			if err := me.exportCounter(count, kind, desc, []string{}); err != nil {
				return fmt.Errorf("exporting counter: %w", err)
			}
		} else if lv, ok := agg.(aggregator.LastValue); ok {
			if err := me.exportLastValue(lv, kind, desc, []string{}); err != nil {
				return fmt.Errorf("exporting last value: %w", err)
			}
		}
		return nil
	})

	return aggError
}

func (me *metricsExporter) exportSum(sum aggregator.Sum, kind core.NumberKind, desc *metric.Descriptor, labels []string) error {
	return nil
}

func (me *metricsExporter) exportCounter(count aggregator.Count, kind core.NumberKind, desc *metric.Descriptor, labels []string) error {
	return nil
}

func (me *metricsExporter) exportLastValue(lv aggregator.LastValue, kind core.NumberKind, desc *metric.Descriptor, labels []string) error {
	return nil
}

func (me *metricsExporter) handleMetricsUpload(metrics []*metricdata.Metric) {
	err := me.uploadMetrics(metrics)
	if err != nil {
		me.o.handleError(err)
	}
}

// TODO(ymotongpoo): replace with actual implementation
func (me *metricsExporter) uploadMetrics(m []*metricdata.Metric) error {
	return nil
}

// getTaskValue returns a task label value in the format of
// "go-<pid>@<hostname>".
func getTaskValue() string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}
	return "go-" + strconv.Itoa(os.Getpid()) + "@" + hostname
}
