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
)

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
		for j := 0; j < rl.InstrumentationLibraryLogs().Len(); j++ {
			ill := rl.InstrumentationLibraryLogs().At(j)
			for k := 0; k < ill.LogRecords().Len(); k++ {
				log := ill.LogRecords().At(k)
				entry, err := l.mapper.logToEntry(log, mr)
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

func (l *logMapper) logToEntry(
	log pdata.LogRecord,
	mr *monitoredres.MonitoredResource,
) (logging.Entry, error) {
	entry := logging.Entry{
		Resource: mr,
	}

	logBody := log.Body().BytesVal()

	if l.cfg.LoggingConfig.ParseHttpRequest {
		httpRequest, strippedLogBody, err := parseHttpRequest(logBody)
		if err != nil {
			l.obs.log.Debug("error parsing HTTPRequest", zap.Error(err))
		} else {
			entry.HTTPRequest = httpRequest
			logBody = strippedLogBody
		}
	}

	if len(logBody) > 0 {
		entry.Payload = json.RawMessage(logBody)
	}

	return entry, nil
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

func parseHttpRequest(message []byte) (*logging.HTTPRequest, []byte, error) {
	parsedLog, strippedMessage, err := extractHttpRequestFromLog(message)
	if err != nil {
		return nil, message, err
	}

	req, err := http.NewRequest(parsedLog.RequestMethod, parsedLog.RequestURL, nil)
	if err != nil {
		return nil, message, err
	}
	req.Header.Set("Referer", parsedLog.Referer)
	req.Header.Set("User-Agent", parsedLog.UserAgent)

	httpRequest := &logging.HTTPRequest{
		Request:                        req,
		RequestSize:                    parsedLog.RequestSize,
		Status:                         parsedLog.Status,
		ResponseSize:                   parsedLog.ResponseSize,
		LocalIP:                        parsedLog.ServerIP,
		RemoteIP:                       parsedLog.RemoteIP,
		CacheHit:                       parsedLog.CacheHit,
		CacheValidatedWithOriginServer: parsedLog.CacheValidatedWithOriginServer,
		CacheFillBytes:                 parsedLog.CacheFillBytes,
		CacheLookup:                    parsedLog.CacheLookup,
	}
	if parsedLog.Latency != "" {
		latency, err := time.ParseDuration(parsedLog.Latency)
		if err == nil {
			httpRequest.Latency = latency
		}
	}

	return httpRequest, strippedMessage, nil
}

func extractHttpRequestFromLog(message []byte) (*httpRequestLog, []byte, error) {
	httpRequestKey := "httpRequest" // TODO(@braydonk) Should this come from the config?

	var unmarshalledMessage map[string]interface{}
	if err := json.Unmarshal(message, &unmarshalledMessage); err != nil {
		return nil, message, err
	}

	httpRequestMap, ok := unmarshalledMessage[httpRequestKey]
	if !ok {
		return nil, message, fmt.Errorf("message has no key %s", httpRequestKey)
	}
	httpRequestBytes, err := json.Marshal(httpRequestMap)
	if err != nil {
		return nil, message, err
	}
	var httpRequest *httpRequestLog
	if err := json.Unmarshal(httpRequestBytes, &httpRequest); err != nil {
		return nil, message, err
	}

	delete(unmarshalledMessage, httpRequestKey)
	if len(unmarshalledMessage) == 0 {
		return httpRequest, []byte{}, nil
	}
	strippedMessage, err := json.Marshal(unmarshalledMessage)
	if err != nil {
		return httpRequest, message, err
	}
	return httpRequest, strippedMessage, nil
}
