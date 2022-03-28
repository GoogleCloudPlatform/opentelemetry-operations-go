// Copyright 2022 Google LLC
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

package collector

import (
	"context"

	"cloud.google.com/go/logging"
	"go.opentelemetry.io/collector/model/pdata"
)

type LogsExporter struct {
	client *logging.Client
	logger *logging.Logger
}

func NewGoogleCloudLogsExporter(ctx context.Context, cfg Config) (*LogsExporter, error) {
	client, err := logging.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		return nil, err
	}
	logger := client.Logger("my-log") // TODO(@damemi) detect log name
	return &LogsExporter{
		client: client,
		logger: logger,
	}, nil
}

func (l *LogsExporter) Shutdown(ctx context.Context) error {
	return l.client.Close()
}

func (l *LogsExporter) PushLogs(ctx context.Context, ld pdata.Logs) error {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		mapper := &metricMapper{} // Refactor metricMapper to map MRs for logging?
		mr, _ := mapper.resourceToMonitoredResource(rl.Resource())
		for j := 0; j < rl.InstrumentationLibraryLogs().Len(); j++ {
			ill := rl.InstrumentationLibraryLogs().At(j)
			for k := 0; k < ill.LogRecords().Len(); k++ {
				log := ill.LogRecords().At(k)
				l.logger.Log(logging.Entry{
					Resource: mr,
					Payload:  log.Body().AsString(),
				})
			}
		}
	}
	return nil
}
