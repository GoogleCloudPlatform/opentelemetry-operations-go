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
	"net/url"
	"time"

	"cloud.google.com/go/logging"
	loggingv2 "cloud.google.com/go/logging/apiv2"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	logpb "google.golang.org/genproto/googleapis/logging/v2"
)

const HTTPRequestAttributeKey = "com.google.httpRequest"

// severityMapping maps the integer severity level values from OTel [0-24]
// to matching Cloud Logging severity levels
var severityMapping = []logging.Severity{
	logging.Default,   // Default, 0
	logging.Debug,     //
	logging.Debug,     //
	logging.Debug,     //
	logging.Debug,     //
	logging.Debug,     //
	logging.Debug,     //
	logging.Debug,     //
	logging.Debug,     // 1-8 -> Debug
	logging.Info,      //
	logging.Info,      // 9-10 -> Info
	logging.Notice,    //
	logging.Notice,    // 11-12 -> Notice
	logging.Warning,   //
	logging.Warning,   //
	logging.Warning,   //
	logging.Warning,   // 13-16 -> Warning
	logging.Error,     //
	logging.Error,     //
	logging.Error,     //
	logging.Error,     // 17-20 -> Error
	logging.Critical,  //
	logging.Critical,  // 21-22 -> Critical
	logging.Alert,     // 23 -> Alert
	logging.Emergency, // 24 -> Emergency
}

type LogsExporter struct {
	cfg    Config
	obs    selfObservability
	mapper logMapper

	loggingClient *loggingv2.Client
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
	loggingClient, err := loggingv2.NewClient(ctx)
	if err != nil {
		return nil, err
	}

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

		loggingClient: loggingClient,
	}, nil
}

func (l *LogsExporter) Shutdown(ctx context.Context) error {
	return l.loggingClient.Close()
}

func (l *LogsExporter) PushLogs(ctx context.Context, ld plog.Logs) error {
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
					internalLogEntry, err := logging.ToLogEntry(entry, fmt.Sprintf("projects/%s", l.cfg.ProjectID))
					if err != nil {
						return err
					}
					logName, err := l.mapper.getLogName(log)
					if err != nil {
						return err
					}
					request := &logpb.WriteLogEntriesRequest{
						LogName: fmt.Sprintf("projects/%s/logs/%s",
							l.cfg.ProjectID,
							url.PathEscape(logName)),
						Resource: mr,
						Entries:  []*logpb.LogEntry{internalLogEntry},
					}
					// TODO(damemi): handle response code and batching for multiple requests
					_, err = l.loggingClient.WriteLogEntries(ctx, request)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	if len(logPushErrors) > 0 {
		return multierr.Combine(logPushErrors...)
	}
	return nil
}

func (l logMapper) getLogName(log pdata.LogRecord) (string, error) {
	logNameAttr, exists := log.Attributes().Get("com.google.logName")
	if exists {
		return logNameAttr.AsString(), nil
	}
	if len(l.cfg.LogConfig.DefaultLogName) > 0 {
		return l.cfg.LogConfig.DefaultLogName, nil
	}
	return "", fmt.Errorf("no log name provided.  Set the 'default_log_name' option, or add the 'com.google.logName' attribute to set a log name")
}

func (l logMapper) logToEntry(
	log plog.LogRecord,
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

	if log.SeverityNumber() < 0 || int(log.SeverityNumber()) > len(severityMapping)-1 {
		return entry, fmt.Errorf("Unknown SeverityNumber %v", log.SeverityNumber())
	}
	entry.Severity = severityMapping[log.SeverityNumber()]

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
	case pcommon.ValueTypeBytes:
		return logBody.BytesVal(), nil
	case pcommon.ValueTypeString:
		return logBody.AsString(), nil
	case pcommon.ValueTypeMap:
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
