// Copyright 2020 Google LLC
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

	"go.opentelemetry.io/otel/sdk/metric/export"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/sdkapi"
	"go.opentelemetry.io/otel/sdk/resource"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"golang.org/x/oauth2/google"
)

const (
	// defaultReportingDuration defaults to 60 seconds.
	defaultReportingDuration = 60 * time.Second

	// minimumReportingDuration is the minimum duration supported by Google Cloud Monitoring.
	// As of Apr 2020, the minimum duration is 10 second for custom metrics.
	// https://cloud.google.com/monitoring/custom-metrics/creating-metrics#writing-ts
	minimumReportingDuration = 10 * time.Second
)

var (
	errReportingIntervalTooLow = fmt.Errorf("reporting interval less than %s", minimumReportingDuration)
)

// Exporter is the public interface of OpenTelemetry metric exporter for
// Google Cloud Monitoring.
type Exporter struct {
	metricExporter *metricExporter
}

// NewRawExporter creates a new Exporter thats implements metric.Exporter.
func NewRawExporter(opts ...Option) (*Exporter, error) {
	o := options{context: context.Background()}
	for _, opt := range opts {
		opt(&o)
	}

	if o.projectID == "" {
		creds, err := google.FindDefaultCredentials(o.context, monitoring.DefaultAuthScopes()...)
		if err != nil {
			return nil, fmt.Errorf("Failed to find Google Cloud credentials: %v", err)
		}
		if creds.ProjectID == "" {
			return nil, errors.New("Google Cloud Monitoring: no project found with application default credentials")
		}
		o.projectID = creds.ProjectID
	}
	if o.reportingInterval == 0 {
		o.reportingInterval = defaultReportingDuration
	}
	if o.reportingInterval < minimumReportingDuration {
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
func (e *Exporter) Export(ctx context.Context, res *resource.Resource, ilr export.InstrumentationLibraryReader) error {
	return e.metricExporter.ExportMetrics(ctx, res, ilr)
}

// TemporalityFor returns cumulative temporality
func (e *Exporter) TemporalityFor(*sdkapi.Descriptor, aggregation.Kind) aggregation.Temporality {
	return aggregation.CumulativeTemporality
}
