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
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cloud.google.com/go/logging"
	loggingv2 "cloud.google.com/go/logging/apiv2"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	logpb "google.golang.org/genproto/googleapis/logging/v2"
	"google.golang.org/protobuf/proto"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

const (
	defaultMaxEntrySize   = 256000   // 256 KB
	defaultMaxRequestSize = 10000000 // 10 MB

	HTTPRequestAttributeKey    = "gcp.http_request"
	LogNameAttributeKey        = "gcp.log_name"
	SourceLocationAttributeKey = "gcp.source_location"
	TraceSampledAttributeKey   = "gcp.trace_sampled"
)

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

// otelSeverityForText maps the generic aliases of SeverityTexts to SeverityNumbers.
// This can be useful if SeverityText is manually set to one of the values from the data
// model in a way that doesn't automatically parse the SeverityNumber as well
// (see https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/issues/442)
// Otherwise, this is the mapping that is automatically used by the Stanza log severity parser
// (https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.54.0/pkg/stanza/operator/helper/severity_builder.go#L34-L57)
var otelSeverityForText = map[string]plog.SeverityNumber{
	"trace":  plog.SeverityNumberTrace,
	"trace2": plog.SeverityNumberTrace2,
	"trace3": plog.SeverityNumberTrace3,
	"trace4": plog.SeverityNumberTrace4,
	"debug":  plog.SeverityNumberDebug,
	"debug2": plog.SeverityNumberDebug2,
	"debug3": plog.SeverityNumberDebug3,
	"debug4": plog.SeverityNumberDebug4,
	"info":   plog.SeverityNumberInfo,
	"info2":  plog.SeverityNumberInfo2,
	"info3":  plog.SeverityNumberInfo3,
	"info4":  plog.SeverityNumberInfo4,
	"warn":   plog.SeverityNumberWarn,
	"warn2":  plog.SeverityNumberWarn2,
	"warn3":  plog.SeverityNumberWarn3,
	"warn4":  plog.SeverityNumberWarn4,
	"error":  plog.SeverityNumberError,
	"error2": plog.SeverityNumberError2,
	"error3": plog.SeverityNumberError3,
	"error4": plog.SeverityNumberError4,
	"fatal":  plog.SeverityNumberFatal,
	"fatal2": plog.SeverityNumberFatal2,
	"fatal3": plog.SeverityNumberFatal3,
	"fatal4": plog.SeverityNumberFatal4,
}

type LogsExporter struct {
	obs           selfObservability
	loggingClient *loggingv2.Client
	cfg           Config
	mapper        logMapper
}

type logMapper struct {
	obs          selfObservability
	cfg          Config
	maxEntrySize int
}

func NewGoogleCloudLogsExporter(
	ctx context.Context,
	cfg Config,
	log *zap.Logger,
) (*LogsExporter, error) {
	err := setProjectFromADC(ctx, &cfg, loggingv2.DefaultAuthScopes())
	if err != nil {
		return nil, err
	}

	clientOpts, err := generateClientOptions(ctx, &cfg.LogConfig.ClientConfig, cfg.UserAgent, cfg.ImpersonateConfig)
	if err != nil {
		return nil, err
	}

	loggingClient, err := loggingv2.NewClient(ctx, clientOpts...)
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
			obs:          obs,
			cfg:          cfg,
			maxEntrySize: defaultMaxEntrySize,
		},

		loggingClient: loggingClient,
	}, nil
}

func (l *LogsExporter) Shutdown(ctx context.Context) error {
	return l.loggingClient.Close()
}

func (l *LogsExporter) PushLogs(ctx context.Context, ld plog.Logs) error {
	entries, err := l.mapper.createEntries(ld)
	if err != nil {
		return err
	}

	errors := []error{}
	entry := 0
	currentBatchSize := 0
	// Send entries in WriteRequest chunks
	// TODO(damemi): Add integration test for batch request processing
	for len(entries) > 0 {
		// default to max int so that when we are at index=len we skip the size check to avoid panic
		// (index=len is the break condition when we reassign entries=entries[len:])
		entrySize := defaultMaxRequestSize
		if entry < len(entries) {
			entrySize = proto.Size(entries[entry])
		}

		// this block gets skipped if we are out of entries to check
		if currentBatchSize+entrySize < defaultMaxRequestSize {
			// if adding the current entry to the current batch doesn't go over the request size,
			// increase the index and account for the new request size, then continue
			currentBatchSize += entrySize
			entry++
			continue
		}

		// if the current entry goes over the request size (or we have gone over every entry, i.e. index=len),
		// write the list up to but not including the current entry's index
		_, err := l.writeLogEntries(ctx, entries[:entry])
		if err != nil {
			errors = append(errors, err)
		}

		entries = entries[entry:]
		entry = 0
	}

	if len(errors) > 0 {
		return multierr.Combine(errors...)
	}
	return nil
}

func (l logMapper) createEntries(ld plog.Logs) ([]*logpb.LogEntry, error) {
	errors := []error{}
	entries := make([]*logpb.LogEntry, 0, 0)
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		mr := defaultResourceToMonitoredResource(rl.Resource())
		projectID := l.cfg.ProjectID
		// override project ID with gcp.project.id, if present
		if projectFromResource, found := rl.Resource().Attributes().Get(resourcemapping.ProjectIDAttributeKey); found {
			projectID = projectFromResource.AsString()
		}

		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			instrumentationSource := sl.Scope().Name()
			instrumentationVersion := sl.Scope().Version()

			for k := 0; k < sl.LogRecords().Len(); k++ {
				log := sl.LogRecords().At(k)

				// We can't just set logName on these entries otherwise the conversion to internal will fail
				// We also need the logName here to be able to accurately calculate the overhead of entry
				// metadata in case the payload needs to be split between multiple entries.
				logName, err := l.getLogName(log)
				if err != nil {
					errors = append(errors, err)
					continue
				}

				splitEntries, err := l.logToSplitEntries(
					log,
					mr,
					instrumentationSource,
					instrumentationVersion,
					time.Now(),
					logName,
					projectID,
				)
				if err != nil {
					errors = append(errors, err)
					continue
				}

				for splitIndex, entry := range splitEntries {
					internalLogEntry, err := l.logEntryToInternal(entry, logName, projectID, mr, len(splitEntries), splitIndex)
					if err != nil {
						errors = append(errors, err)
						continue
					}
					entries = append(entries, internalLogEntry)
				}
			}
		}
	}

	return entries, multierr.Combine(errors...)
}

func (l logMapper) logEntryToInternal(
	entry logging.Entry,
	logName string,
	projectID string,
	mr *monitoredres.MonitoredResource,
	splits int,
	splitIndex int,
) (*logpb.LogEntry, error) {

	internalLogEntry, err := logging.ToLogEntry(entry, fmt.Sprintf("projects/%s", projectID))
	if err != nil {
		return nil, err
	}

	internalLogEntry.LogName = fmt.Sprintf("projects/%s/logs/%s", projectID, url.PathEscape(logName))
	internalLogEntry.Resource = mr
	if splits > 1 {
		internalLogEntry.Split = &logpb.LogSplit{
			Uid:         fmt.Sprintf("%s-%s", logName, entry.Timestamp.String()),
			Index:       int32(splitIndex),
			TotalSplits: int32(splits),
		}
	}
	return internalLogEntry, nil
}

func (l *LogsExporter) writeLogEntries(ctx context.Context, batch []*logpb.LogEntry) (*logpb.WriteLogEntriesResponse, error) {
	request := &logpb.WriteLogEntriesRequest{
		PartialSuccess: true,
		Entries:        batch,
	}

	// TODO(damemi): handle response code
	return l.loggingClient.WriteLogEntries(ctx, request)
}

func (l logMapper) getLogName(log plog.LogRecord) (string, error) {
	logNameAttr, exists := log.Attributes().Get(LogNameAttributeKey)
	if exists {
		return logNameAttr.AsString(), nil
	}
	if len(l.cfg.LogConfig.DefaultLogName) > 0 {
		return l.cfg.LogConfig.DefaultLogName, nil
	}
	return "", fmt.Errorf("no log name provided.  Set the 'default_log_name' option, or add the 'gcp.log_name' attribute to set a log name")
}

func (l logMapper) logToSplitEntries(
	log plog.LogRecord,
	mr *monitoredres.MonitoredResource,
	instrumentationSource string,
	instrumentationVersion string,
	processTime time.Time,
	logName string,
	projectID string,
) ([]logging.Entry, error) {
	entry := logging.Entry{
		Resource: mr,
	}

	// if timestamp has not been explicitly initialized, default to current time
	// TODO: figure out how to fall back to observed_time_unix_nano as recommended
	//   (see https://github.com/open-telemetry/opentelemetry-proto/blob/4abbb78/opentelemetry/proto/logs/v1/logs.proto#L176-L179)
	entry.Timestamp = log.Timestamp().AsTime()
	if log.Timestamp() == 0 {
		entry.Timestamp = processTime
	}

	// build our own map off OTel attributes so we don't have to call .Get() for each special case
	// (.Get() ranges over all attributes each time)
	attrsMap := make(map[string]pcommon.Value)
	log.Attributes().Range(func(k string, v pcommon.Value) bool {
		attrsMap[k] = v
		return true
	})

	// parse LogEntrySourceLocation struct from OTel attribute
	if sourceLocation, ok := attrsMap[SourceLocationAttributeKey]; ok {
		var logEntrySourceLocation logpb.LogEntrySourceLocation
		err := json.Unmarshal(sourceLocation.BytesVal().AsRaw(), &logEntrySourceLocation)
		if err != nil {
			return []logging.Entry{entry}, err
		}
		entry.SourceLocation = &logEntrySourceLocation
		delete(attrsMap, SourceLocationAttributeKey)
	}

	// parse TraceSampled boolean from OTel attribute
	if traceSampled, ok := attrsMap[TraceSampledAttributeKey]; ok {
		entry.TraceSampled = traceSampled.BoolVal()
		delete(attrsMap, TraceSampledAttributeKey)
	}

	// parse TraceID and SpanID, if present
	if !log.TraceID().IsEmpty() {
		entry.Trace = fmt.Sprintf("projects/%s/traces/%s", projectID, log.TraceID().HexString())
	}
	if !log.SpanID().IsEmpty() {
		entry.SpanID = log.SpanID().HexString()
	}

	if httpRequestAttr, ok := attrsMap[HTTPRequestAttributeKey]; ok {
		var httpRequest *logging.HTTPRequest
		httpRequest, err := l.parseHTTPRequest(httpRequestAttr)
		if err != nil {
			l.obs.log.Debug("Unable to parse httpRequest", zap.Error(err))
		}
		entry.HTTPRequest = httpRequest
		delete(attrsMap, HTTPRequestAttributeKey)
	}

	if log.SeverityNumber() < 0 || int(log.SeverityNumber()) > len(severityMapping)-1 {
		return []logging.Entry{entry}, fmt.Errorf("Unknown SeverityNumber %v", log.SeverityNumber())
	}
	severityNumber := log.SeverityNumber()
	// Log severity levels are based on numerical values defined by Otel/GCP, which are informally mapped to generic text values such as "ALERT", "Debug", etc.
	// In some cases, a SeverityText value can be automatically mapped to a matching SeverityNumber.
	// If not (for example, when directly setting the SeverityText on a Log entry with the Transform processor), then the
	// SeverityText might be something like "ALERT" while the SeverityNumber is still "0".
	// In this case, we will attempt to map the text ourselves to one of the defined Otel SeverityNumbers.
	// We do this by checking that the SeverityText is NOT "default" (ie, it exists in our map) and that the SeverityNumber IS "0".
	// (This also excludes other unknown/custom severity text values, which may have user-defined mappings in the collector)
	if severityForText, ok := otelSeverityForText[strings.ToLower(log.SeverityText())]; ok && severityNumber == 0 {
		severityNumber = severityForText
	}
	entry.Severity = severityMapping[severityNumber]

	if entry.Labels == nil &&
		(len(instrumentationSource) > 0 ||
			len(instrumentationVersion) > 0) {
		entry.Labels = make(map[string]string)
	}

	// TODO(damemi): Make overwriting these labels (if they already exist) configurable
	if len(instrumentationSource) > 0 {
		entry.Labels["instrumentation_source"] = instrumentationSource
	}
	if len(instrumentationVersion) > 0 {
		entry.Labels["instrumentation_version"] = instrumentationVersion
	}

	// parse remaining OTel attributes to GCP labels
	for k, v := range attrsMap {
		// skip "gcp.*" attributes since we process these to other fields
		if strings.HasPrefix(k, "gcp.") {
			continue
		}
		if entry.Labels == nil {
			entry.Labels = make(map[string]string)
		}
		if _, ok := entry.Labels[k]; !ok {
			entry.Labels[k] = v.AsString()
		}
	}

	// Calculate the size of the internal log entry so this overhead can be accounted
	// for when determining the need to split based on payload size
	// TODO(damemi): Find an appropriate estimated buffer to account for the LogSplit struct as well
	logOverhead, err := l.logEntryToInternal(entry, logName, projectID, mr, 0, 0)
	if err != nil {
		return []logging.Entry{entry}, err
	}
	// make a copy so the proto initialization doesn't modify the original entry
	overheadClone := proto.Clone(logOverhead)
	overheadBytes := proto.Size(overheadClone)

	payload, splits, err := parseEntryPayload(log.Body(), l.maxEntrySize-overheadBytes)
	if err != nil {
		return []logging.Entry{entry}, err
	}

	// Split log entries with a string payload into fewer entries
	if splits > 1 {
		entries := make([]logging.Entry, splits)
		payloadString := payload.(string)

		// Start by assuming all splits will be even (this may not be the case)
		startIndex := 0
		endIndex := int(math.Floor((1.0 / float64(splits)) * float64(len(payloadString))))
		for i := 1; i <= splits; i++ {
			currentSplit := payloadString[startIndex:endIndex]

			// If the current split is larger than the entry size, iterate until it is within the max
			// (This may happen since not all characters are exactly 1 byte)
			for len([]byte(currentSplit)) > l.maxEntrySize {
				endIndex--
				currentSplit = payloadString[startIndex:endIndex]
			}
			entries[i-1] = entry
			entries[i-1].Payload = currentSplit

			// Update slice indices to the next chunk
			startIndex = endIndex
			endIndex = int(math.Floor((float64(i+1) / float64(splits)) * float64(len(payloadString))))
		}
		return entries, nil
	}

	entry.Payload = payload
	return []logging.Entry{entry}, nil
}

func parseEntryPayload(logBody pcommon.Value, maxEntrySize int) (interface{}, int, error) {
	if len(logBody.AsString()) == 0 {
		return nil, 0, nil
	}
	switch logBody.Type() {
	case pcommon.ValueTypeBytes:
		return logBody.BytesVal().AsRaw(), 1, nil
	case pcommon.ValueTypeString:
		return logBody.AsString(), int(math.Ceil(float64(len([]byte(logBody.AsString()))) / float64(maxEntrySize))), nil
	case pcommon.ValueTypeMap:
		return logBody.MapVal().AsRaw(), 1, nil

	default:
		return nil, 0, fmt.Errorf("unknown log body value %v", logBody.Type().String())
	}
}

// JSON keys derived from:
// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#httprequest
type httpRequestLog struct {
	RemoteIP                       string `json:"remoteIp"`
	RequestURL                     string `json:"requestUrl"`
	Latency                        string `json:"latency"`
	Referer                        string `json:"referer"`
	ServerIP                       string `json:"serverIp"`
	UserAgent                      string `json:"userAgent"`
	RequestMethod                  string `json:"requestMethod"`
	Protocol                       string `json:"protocol"`
	ResponseSize                   int64  `json:"responseSize,string"`
	RequestSize                    int64  `json:"requestSize,string"`
	CacheFillBytes                 int64  `json:"cacheFillBytes,string"`
	Status                         int    `json:"status,string"`
	CacheLookup                    bool   `json:"cacheLookup"`
	CacheHit                       bool   `json:"cacheHit"`
	CacheValidatedWithOriginServer bool   `json:"cacheValidatedWithOriginServer"`
}

func (l logMapper) parseHTTPRequest(httpRequestAttr pcommon.Value) (*logging.HTTPRequest, error) {
	var bytes []byte
	switch httpRequestAttr.Type() {
	case pcommon.ValueTypeBytes:
		bytes = httpRequestAttr.BytesVal().AsRaw()
	case pcommon.ValueTypeString, pcommon.ValueTypeMap:
		bytes = []byte(httpRequestAttr.AsString())
	}

	// TODO: Investigate doing this without the JSON unmarshal. Getting the attribute as a map
	// instead of a slice of bytes could do, but would need a lot of type casting and checking
	// assertions with it.
	var parsedHTTPRequest httpRequestLog
	if err := json.Unmarshal(bytes, &parsedHTTPRequest); err != nil {
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
