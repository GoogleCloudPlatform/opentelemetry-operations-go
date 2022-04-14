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
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/logging"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	logpb "google.golang.org/genproto/googleapis/logging/v2"
)

const HTTPRequestAttributeKey = "com.google.httpRequest"

type LogsExporter struct {
	cfg    Config
	obs    selfObservability
	mapper logMapper

	client *logging.Client
	logger *logging.Logger
}

type logMapper struct {
	obs selfObservability
	cfg Config
}

func NewGoogleCloudLogsExporter(
	ctx context.Context,
	cfg Config,
	log *zap.Logger,
) (*LogsExporter, error) {
	client, err := logging.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		return nil, err
	}
	logger := client.Logger("my-log") // TODO(@damemi) detect log name
	obs := selfObservability{
		log: log,
	}
	return &LogsExporter{
		cfg: cfg,
		obs: obs,
		mapper: logMapper{
			obs: obs,
			cfg: cfg,
		},

		client: client,
		logger: logger,
	}, nil
}

func (l *LogsExporter) Shutdown(ctx context.Context) error {
	return l.client.Close()
}

func (l *LogsExporter) PushLogs(ctx context.Context, ld pdata.Logs) error {
	mapper := &metricMapper{} // Refactor metricMapper to map MRs for logging?
	logPushErrors := []error{}
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		mr, _ := mapper.resourceToMonitoredResource(rl.Resource())

		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			instrumentationSource := sl.Scope().Name()
			instrumentationVersion := sl.Scope().Version()

			for k := 0; k < sl.LogRecords().Len(); k++ {
				log := sl.LogRecords().At(k)

				entry, err := l.mapper.logToEntry(
					log,
					mr,
					instrumentationSource,
					instrumentationVersion,
					time.Now())
				if err != nil {
					logPushErrors = append(logPushErrors, err)
				} else {
					l.logger.Log(entry)
				}
			}
		}
	}

	if len(logPushErrors) > 0 {
		return multierr.Combine(logPushErrors...)
	}
	return nil
}

func (l logMapper) logToEntry(
	log pdata.LogRecord,
	mr *monitoredres.MonitoredResource,
	instrumentationSource string,
	instrumentationVersion string,
	processTime time.Time,
) (logging.Entry, error) {
	entry := logging.Entry{
		Resource: mr,
	}

	// TODO(damemi): Make overwriting these labels (if they already exist) configurable
	if len(instrumentationSource) > 0 {
		entry.Labels["instrumentation_source"] = instrumentationSource
	}
	if len(instrumentationVersion) > 0 {
		entry.Labels["instrumentation_version"] = instrumentationVersion
	}

	// if timestamp has not been explicitly initialized, default to current time
	// TODO: figure out how to fall back to observed_time_unix_nano as recommended
	//   (see https://github.com/open-telemetry/opentelemetry-proto/blob/4abbb78/opentelemetry/proto/logs/v1/logs.proto#L176-L179)
	entry.Timestamp = log.Timestamp().AsTime()
	if log.Timestamp() == 0 {
		entry.Timestamp = processTime
	}

	// parse LogEntrySourceLocation struct from OTel attribute
	sourceLocation, ok := log.Attributes().Get("com.google.sourceLocation")
	if ok {
		var logEntrySourceLocation logpb.LogEntrySourceLocation
		err := json.Unmarshal(sourceLocation.BytesVal(), &logEntrySourceLocation)
		if err != nil {
			return entry, err
		}
		entry.SourceLocation = &logEntrySourceLocation
	}

	// parse TraceID and SpanID, if present
	if !log.TraceID().IsEmpty() {
		entry.Trace = log.TraceID().HexString()
	}
	if !log.SpanID().IsEmpty() {
		entry.SpanID = log.SpanID().HexString()
	}

	httpRequestAttr, ok := log.Attributes().Get(HTTPRequestAttributeKey)
	if ok {
		httpRequest, err := l.parseHTTPRequest(httpRequestAttr.BytesVal())
		if err != nil {
			l.obs.log.Debug("Unable to parse httpRequest", zap.Error(err))
		}
		entry.HTTPRequest = httpRequest
	}

	payload, err := parseEntryPayload(log.Body())
	if err != nil {
		return entry, nil
	}
	entry.Payload = payload

	return entry, nil
}

func parseEntryPayload(logBody pdata.Value) (interface{}, error) {
	if len(logBody.AsString()) == 0 {
		return nil, nil
	}
	switch logBody.Type() {
	case pdata.ValueTypeBytes:
		return logBody.BytesVal(), nil
	case pdata.ValueTypeString:
		return logBody.AsString(), nil
	case pdata.ValueTypeMap:
		return logBody.MapVal().AsRaw(), nil

	default:
		return nil, fmt.Errorf("unknown log body value %v", logBody.Type().String())
	}
}

// JSON keys derived from:
// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#httprequest
type httpRequestLog struct {
	RequestMethod                  string `json:"requestMethod"`
	RequestURL                     string `json:"requestUrl"`
	RequestSize                    int64  `json:"requestSize,string"`
	Status                         int    `json:"status,string"`
	ResponseSize                   int64  `json:"responseSize,string"`
	UserAgent                      string `json:"userAgent"`
	RemoteIP                       string `json:"remoteIp"`
	ServerIP                       string `json:"serverIp"`
	Referer                        string `json:"referer"`
	Latency                        string `json:"latency"`
	CacheLookup                    bool   `json:"cacheLookup"`
	CacheHit                       bool   `json:"cacheHit"`
	CacheValidatedWithOriginServer bool   `json:"cacheValidatedWithOriginServer"`
	CacheFillBytes                 int64  `json:"cacheFillBytes,string"`
	Protocol                       string `json:"protocol"`
}

func (l logMapper) parseHTTPRequest(httpRequestAttr []byte) (*logging.HTTPRequest, error) {
	// TODO: Investigate doing this without the JSON unmarshal. Getting the attribute as a map
	// instead of a slice of bytes could do, but would need a lot of type casting and checking
	// assertions with it.
	var parsedHTTPRequest httpRequestLog
	if err := json.Unmarshal(httpRequestAttr, &parsedHTTPRequest); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(parsedHTTPRequest.RequestMethod, parsedHTTPRequest.RequestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", parsedHTTPRequest.Referer)
	req.Header.Set("User-Agent", parsedHTTPRequest.UserAgent)

	httpRequest := &logging.HTTPRequest{
		Request:                        req,
		RequestSize:                    parsedHTTPRequest.RequestSize,
		Status:                         parsedHTTPRequest.Status,
		ResponseSize:                   parsedHTTPRequest.ResponseSize,
		LocalIP:                        parsedHTTPRequest.ServerIP,
		RemoteIP:                       parsedHTTPRequest.RemoteIP,
		CacheHit:                       parsedHTTPRequest.CacheHit,
		CacheValidatedWithOriginServer: parsedHTTPRequest.CacheValidatedWithOriginServer,
		CacheFillBytes:                 parsedHTTPRequest.CacheFillBytes,
		CacheLookup:                    parsedHTTPRequest.CacheLookup,
	}
	if parsedHTTPRequest.Latency != "" {
		latency, err := time.ParseDuration(parsedHTTPRequest.Latency)
		if err == nil {
			httpRequest.Latency = latency
		}
	}

	return httpRequest, nil
}
