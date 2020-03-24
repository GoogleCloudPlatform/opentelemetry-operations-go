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
	_ "go.opencensus.io/metric/metricexport"
	otel "go.opentelemetry.io/otel/sdk"
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

	c             *monitoringapi.MetricClient
	defaultLabels map[string]labelValue
	// ir            *metricexport.IntervalReader

	// initReaderOnce sync.Once
}

var (
	errBlankProjectID = errors.New("expecting a non-blank ProjectID")
)

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
		c:                      client,
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

	// TODO(ymotongpoo): Confirm how to change bundler part by referring trace implementation.
	//
	// e.viewDataBundler = bundler.NewBundler((*view.Data)(nil), func(bundle interface{}) {
	// 	vds := bundle.([]*view.Data)
	// 	e.handleUpload(vds...)
	// })
	// e.metricsBundler = bundler.NewBundler((*metricdata.Metric)(nil), func(bundle interface{}) {
	// 	metrics := bundle.([]*metricdata.Metric)
	// 	e.handleMetricsUpload(metrics)
	// })
	// if delayThreshold := e.o.BundleDelayThreshold; delayThreshold > 0 {
	// 	e.viewDataBundler.DelayThreshold = delayThreshold
	// 	e.metricsBundler.DelayThreshold = delayThreshold
	// }
	// if countThreshold := e.o.BundleCountThreshold; countThreshold > 0 {
	// 	e.viewDataBundler.BundleCountThreshold = countThreshold
	// 	e.metricsBundler.BundleCountThreshold = countThreshold
	// }
	return e, nil
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
