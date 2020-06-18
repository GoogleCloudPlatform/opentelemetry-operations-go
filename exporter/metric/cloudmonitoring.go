// Copyright 2020, Google Inc.
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
	"errors"
	"fmt"
	"time"

	export "go.opentelemetry.io/otel/sdk/export/metric"
	"golang.org/x/oauth2/google"

	monitoring "cloud.google.com/go/monitoring/apiv3"
)

const (
	// defaultReportingDuration defaults to 60 seconds.
	// https://cloud.google.com/monitoring/custom-metrics/creating-metrics#monitoring_write_timeseries-go
	defaultReportingDuration = 60 * time.Second

	// minimumReportingDuration is the minimum duration supported by Google Cloud Monitoring.
	// As of Apr 2020, the minimum duration is 1 second for custom metrics.
	minimumReportingDuration = 1 * time.Second
)

var (
	errReportingIntervalTooLow = fmt.Errorf("reporting interval less than %d", minimumReportingDuration)
)

// Exporter is the public interface of OpenTelemetry metric exporter for
// Google Cloud Monitoring.
type Exporter struct {
	metricExporter *metricExporter
}

// NewRawExporter creates a new Exporter thats implements metric.Exporter.
func NewRawExporter(opts ...Option) (*Exporter, error) {
	o := options{Context: context.Background()}
	for _, opt := range opts {
		fmt.Errorf("opt %v", opt)
		opt(&o)
	}


	if o.ProjectID == "" {
		creds, err := google.FindDefaultCredentials(o.Context, monitoring.DefaultAuthScopes()...)
		if err != nil {
			return nil, fmt.Errorf("123Failed to find Google Cloud credentials: %v", err)
		}
		if creds.ProjectID == "" {
			return nil, errors.New("123Google Cloud Monitoring: no project found with application default credentials")
		}
		o.ProjectID = creds.ProjectID
	}
	if o.ReportingInterval == 0 {
		o.ReportingInterval = defaultReportingDuration
	}
	if o.ReportingInterval < minimumReportingDuration {
		return nil, errReportingIntervalTooLow
	}

	me, err := newMetricExporter(&o)
	if err != nil {
		return nil, err
	}
	return &Exporter{
		metricExporter: me,
	}, nil
}

// Export exports the provide metric record to Google Cloud Monitoring.
func (e *Exporter) Export(ctx context.Context, cps export.CheckpointSet) error {
	return e.metricExporter.ExportMetrics(ctx, cps)
}
