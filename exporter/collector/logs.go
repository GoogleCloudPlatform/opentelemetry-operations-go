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
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	loggingv2 "cloud.google.com/go/logging/apiv2"
	logpb "cloud.google.com/go/logging/apiv2/loggingpb"
	"github.com/googleapis/gax-go/v2"
	"go.uber.org/zap"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	logtypepb "google.golang.org/genproto/googleapis/logging/type"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/logsutil"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping"
)

const (
	defaultMaxEntrySize   = 256000   // 256 KB
	defaultMaxRequestSize = 10000000 // 10 MB

	HTTPRequestAttributeKey    = "gcp.http_request"
	LogNameAttributeKey        = "gcp.log_name"
	SourceLocationAttributeKey = "gcp.source_location"
	TraceSampledAttributeKey   = "gcp.trace_sampled"

	GCPTypeKey                 = "@type"
	GCPErrorReportingTypeValue = "type.googleapis.com/google.devtools.clouderrorreporting.v1beta1.ReportedErrorEvent"
)

// severityMapping maps the integer severity level values from OTel [0-24]
// to matching Cloud Logging severity levels.
var severityMapping = []logtypepb.LogSeverity{
	logtypepb.LogSeverity_DEFAULT,   // Default, 0
	logtypepb.LogSeverity_DEBUG,     //
	logtypepb.LogSeverity_DEBUG,     //
	logtypepb.LogSeverity_DEBUG,     //
	logtypepb.LogSeverity_DEBUG,     //
	logtypepb.LogSeverity_DEBUG,     //
	logtypepb.LogSeverity_DEBUG,     //
	logtypepb.LogSeverity_DEBUG,     //
	logtypepb.LogSeverity_DEBUG,     // 1-8 -> Debug
	logtypepb.LogSeverity_INFO,      //
	logtypepb.LogSeverity_INFO,      // 9-10 -> Info
	logtypepb.LogSeverity_NOTICE,    //
	logtypepb.LogSeverity_NOTICE,    // 11-12 -> Notice
	logtypepb.LogSeverity_WARNING,   //
	logtypepb.LogSeverity_WARNING,   //
	logtypepb.LogSeverity_WARNING,   //
	logtypepb.LogSeverity_WARNING,   // 13-16 -> Warning
	logtypepb.LogSeverity_ERROR,     //
	logtypepb.LogSeverity_ERROR,     //
	logtypepb.LogSeverity_ERROR,     //
	logtypepb.LogSeverity_ERROR,     // 17-20 -> Error
	logtypepb.LogSeverity_CRITICAL,  //
	logtypepb.LogSeverity_CRITICAL,  // 21-22 -> Critical
	logtypepb.LogSeverity_ALERT,     // 23 -> Alert
	logtypepb.LogSeverity_EMERGENCY, // 24 -> Emergency
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
	// Google Cloud Logging LogSeverity values not mapped in the previous generic aliases.
	// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#logseverity
	"default":   plog.SeverityNumberUnspecified,
	"notice":    plog.SeverityNumberInfo3,
	"warning":   plog.SeverityNumberWarn,
	"critical":  plog.SeverityNumberFatal,
	"alert":     plog.SeverityNumberFatal3,
	"emergency": plog.SeverityNumberFatal4,
}

type attributeProcessingError struct {
	Err error
	Key string
}

func (e *attributeProcessingError) Error() string {
	return fmt.Sprintf("could not process attribute %s: %s", e.Key, e.Err.Error())
}

type unsupportedValueTypeError struct {
	ValueType pcommon.ValueType
}

func (e *unsupportedValueTypeError) Error() string {
	return fmt.Sprintf("unsupported value type %v", e.ValueType)
}

type LogsExporter struct {
	obs           selfObservability
	loggingClient *loggingv2.Client
	cfg           Config
	mapper        logMapper
	timeout       time.Duration
}

type logMapper struct {
	obs            selfObservability
	cfg            Config
	maxEntrySize   int
	maxRequestSize int
}

func NewGoogleCloudLogsExporter(
	ctx context.Context,
	cfg Config,
	set exporter.Settings,
	timeout time.Duration,
) (*LogsExporter, error) {
	SetUserAgent(&cfg, set.BuildInfo)
	obs := selfObservability{
		log:           set.TelemetrySettings.Logger,
		meterProvider: set.TelemetrySettings.MeterProvider,
	}

	return &LogsExporter{
		cfg:     cfg,
		obs:     obs,
		timeout: timeout,
		mapper: logMapper{
			obs:            obs,
			cfg:            cfg,
			maxEntrySize:   defaultMaxEntrySize,
			maxRequestSize: defaultMaxRequestSize,
		},
	}, nil
}

// ConfigureExporter is used by integration tests to set exporter settings not visible to users.
func (l *LogsExporter) ConfigureExporter(config *logsutil.ExporterConfig) {
	if config == nil {
		return
	}
	if config.MaxEntrySize > 0 {
		l.mapper.maxEntrySize = config.MaxEntrySize
	}
	if config.MaxRequestSize > 0 {
		l.mapper.maxRequestSize = config.MaxRequestSize
	}
}

func (l *LogsExporter) Start(ctx context.Context, _ component.Host) error {
	clientOpts, err := generateClientOptions(ctx, &l.cfg.LogConfig.ClientConfig, &l.cfg, loggingv2.DefaultAuthScopes(), l.obs.meterProvider)
	if err != nil {
		return err
	}

	loggingClient, err := loggingv2.NewClient(ctx, clientOpts...)
	if err != nil {
		return err
	}

	if l.cfg.LogConfig.ClientConfig.Compression == gzip.Name {
		loggingClient.CallOptions.WriteLogEntries = append(loggingClient.CallOptions.WriteLogEntries,
			gax.WithGRPCOptions(grpc.UseCompressor(gzip.Name)))
	}
	l.loggingClient = loggingClient
	// We might have modified the config when we generated options above.
	// Make sure changes to the config are synced to the mapper.
	l.mapper.cfg = l.cfg
	return nil
}

func (l *LogsExporter) Shutdown(ctx context.Context) error {
	if l.loggingClient != nil {
		return l.loggingClient.Close()
	}
	return nil
}

func (l *LogsExporter) PushLogs(ctx context.Context, ld plog.Logs) error {
	if l.loggingClient == nil {
		return errors.New("not started")
	}
	projectEntries, err := l.mapper.createEntries(ld)
	if err != nil {
		return err
	}

	var errs []error
	for project, entries := range projectEntries {
		entry := 0
		currentBatchSize := 0
		// Send entries in WriteRequest chunks
		for len(entries) > 0 {
			// default to max int so that when we are at index=len we skip the size check to avoid panic
			// (index=len is the break condition when we reassign entries=entries[len:])
			entrySize := l.mapper.maxRequestSize
			if entry < len(entries) {
				entrySize = proto.Size(entries[entry])
			}

			// this block gets skipped if we are out of entries to check
			if currentBatchSize+entrySize < l.mapper.maxRequestSize {
				// if adding the current entry to the current batch doesn't go over the request size,
				// increase the index and account for the new request size, then continue
				currentBatchSize += entrySize
				entry++
				continue
			}

			// override destination project quota for this write request, if applicable
			if l.cfg.DestinationProjectQuota {
				ctx = metadata.NewOutgoingContext(ctx, metadata.New(map[string]string{"x-goog-user-project": strings.TrimPrefix(project, "projects/")}))
			}

			// if the current entry goes over the request size (or we have gone over every entry, i.e. index=len),
			// write the list up to but not including the current entry's index
			_, err := l.writeLogEntries(ctx, entries[:entry])
			errs = append(errs, err)

			entries = entries[entry:]
			entry = 0
			currentBatchSize = 0
		}
	}
	return errors.Join(errs...)
}

func (l logMapper) createEntries(ld plog.Logs) (map[string][]*logpb.LogEntry, error) {
	// if destination_project_quota is enabled, projectMapKey will be the name of the project for each batch of entries
	// otherwise, we can mix project entries for more efficient batching and store all entries in a single list
	projectMapKey := ""
	var errs []error
	entries := make(map[string][]*logpb.LogEntry)
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		mr := l.cfg.LogConfig.MapMonitoredResource(rl.Resource())
		extraResourceLabels := attributesToUnsanitizedLabels(filterAttributes(rl.Resource().Attributes(), l.cfg.LogConfig.ServiceResourceLabels, l.cfg.LogConfig.ResourceFilters))
		projectID := l.cfg.ProjectID
		// override project ID with gcp.project.id, if present
		if projectFromResource, found := rl.Resource().Attributes().Get(resourcemapping.ProjectIDAttributeKey); found {
			projectID = projectFromResource.AsString()
		}

		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			logLabels := mergeLogLabels(sl.Scope().Name(), sl.Scope().Version(), extraResourceLabels)

			for k := 0; k < sl.LogRecords().Len(); k++ {
				// make a copy of logLabels so that shared attributes (scope/resource) are copied across LogRecords,
				// but that individual LogRecord attributes don't copy over to other Records via map reference.
				entryLabels := make(map[string]string)
				for k, v := range logLabels {
					entryLabels[k] = v
				}
				log := sl.LogRecords().At(k)

				// We can't just set logName on these entries otherwise the conversion to internal will fail
				// We also need the logName here to be able to accurately calculate the overhead of entry
				// metadata in case the payload needs to be split between multiple entries.
				logName, err := l.getLogName(log)
				if err != nil {
					errs = append(errs, err)
					continue
				}

				splitEntries, err := l.logToSplitEntries(
					log,
					mr,
					entryLabels,
					time.Now(),
					logName,
					projectID,
				)
				if err != nil {
					errs = append(errs, err)
					continue
				}

				for _, entry := range splitEntries {
					if l.cfg.DestinationProjectQuota {
						projectMapKey = projectID
					}
					if _, ok := entries[projectMapKey]; !ok {
						entries[projectMapKey] = make([]*logpb.LogEntry, 0)
					}
					entries[projectMapKey] = append(entries[projectMapKey], entry)
				}
			}
		}
	}

	return entries, errors.Join(errs...)
}

// converts attributes to a map[string]string.
// It ensures the label values are valid UTF-8, but does not sanitize keys.
func attributesToUnsanitizedLabels(attrs pcommon.Map) labels {
	ls := make(labels, attrs.Len())
	attrs.Range(func(k string, v pcommon.Value) bool {
		ls[k] = sanitizeUTF8(v.AsString())
		return true
	})
	return ls
}

func mergeLogLabels(instrumentationSource, instrumentationVersion string, resourceLabels map[string]string) map[string]string {
	labelsMap := make(map[string]string)
	// TODO(damemi): Make overwriting these labels (if they already exist) configurable
	if len(instrumentationSource) > 0 {
		labelsMap["instrumentation_source"] = instrumentationSource
	}
	if len(instrumentationVersion) > 0 {
		labelsMap["instrumentation_version"] = instrumentationVersion
	}
	return mergeLabels(labelsMap, resourceLabels)
}

func (l *LogsExporter) writeLogEntries(ctx context.Context, batch []*logpb.LogEntry) (*logpb.WriteLogEntriesResponse, error) {
	timeout := l.timeout
	if timeout <= 0 {
		timeout = 12 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

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
	logRecord plog.LogRecord,
	mr *monitoredrespb.MonitoredResource,
	logLabels map[string]string,
	processTime time.Time,
	logName string,
	projectID string,
) ([]*logpb.LogEntry, error) {
	ts := logRecord.Timestamp().AsTime()
	if logRecord.Timestamp() == 0 || ts.IsZero() {
		// if timestamp is unset, fall back to observed_time_unix_nano as recommended
		//   (see https://github.com/open-telemetry/opentelemetry-proto/blob/4abbb78/opentelemetry/proto/logs/v1/logs.proto#L176-L179)
		if logRecord.ObservedTimestamp() != 0 {
			ts = logRecord.ObservedTimestamp().AsTime()
		} else {
			// if observed_time is 0, use the current time
			ts = processTime
		}
	}

	entry := &logpb.LogEntry{
		Resource:  mr,
		Timestamp: timestamppb.New(ts),
		Labels:    logLabels,
		LogName:   fmt.Sprintf("projects/%s/logs/%s", projectID, url.PathEscape(logName)),
	}

	// build our own map off OTel attributes so we don't have to call .Get() for each special case
	// (.Get() ranges over all attributes each time)
	attrsMap := make(map[string]pcommon.Value)
	logRecord.Attributes().Range(func(k string, v pcommon.Value) bool {
		attrsMap[k] = v
		return true
	})

	// parse LogEntrySourceLocation struct from OTel attribute
	if sourceLocation, ok := attrsMap[SourceLocationAttributeKey]; ok {
		var logEntrySourceLocation logpb.LogEntrySourceLocation
		err := unmarshalAttribute(sourceLocation, &logEntrySourceLocation)
		if err != nil {
			return nil, &attributeProcessingError{Key: SourceLocationAttributeKey, Err: err}
		}
		entry.SourceLocation = &logEntrySourceLocation
		delete(attrsMap, SourceLocationAttributeKey)
	}

	// parse TraceSampled boolean from OTel attribute or IsSampled OTLP flag
	if traceSampled, ok := attrsMap[TraceSampledAttributeKey]; ok || logRecord.Flags().IsSampled() {
		entry.TraceSampled = (traceSampled.Bool() || logRecord.Flags().IsSampled())
		delete(attrsMap, TraceSampledAttributeKey)
	}

	// parse TraceID and SpanID, if present
	if traceID := logRecord.TraceID(); !traceID.IsEmpty() {
		entry.Trace = fmt.Sprintf("projects/%s/traces/%s", projectID, hex.EncodeToString(traceID[:]))
	}
	if spanID := logRecord.SpanID(); !spanID.IsEmpty() {
		entry.SpanId = hex.EncodeToString(spanID[:])
	}

	if httpRequestAttr, ok := attrsMap[HTTPRequestAttributeKey]; ok {
		httpRequest, err := l.parseHTTPRequest(httpRequestAttr)
		if err != nil {
			l.obs.log.Debug("Unable to parse httpRequest", zap.Error(err))
		}
		entry.HttpRequest = httpRequest
		delete(attrsMap, HTTPRequestAttributeKey)
	}

	if logRecord.SeverityNumber() < 0 || int(logRecord.SeverityNumber()) > len(severityMapping)-1 {
		return nil, fmt.Errorf("unknown SeverityNumber %v", logRecord.SeverityNumber())
	}
	severityNumber := logRecord.SeverityNumber()
	// Log severity levels are based on numerical values defined by Otel/GCP, which are informally mapped to generic text values such as "ALERT", "Debug", etc.
	// In some cases, a SeverityText value can be automatically mapped to a matching SeverityNumber.
	// If not (for example, when directly setting the SeverityText on a Log entry with the Transform processor), then the
	// SeverityText might be something like "ALERT" while the SeverityNumber is still "0".
	// In this case, we will attempt to map the text ourselves to one of the defined Otel SeverityNumbers.
	// We do this by checking that the SeverityText is NOT "default" (ie, it exists in our map) and that the SeverityNumber IS "0".
	// (This also excludes other unknown/custom severity text values, which may have user-defined mappings in the collector)
	if severityForText, ok := otelSeverityForText[strings.ToLower(logRecord.SeverityText())]; ok && severityNumber == 0 {
		severityNumber = severityForText
	}
	entry.Severity = severityMapping[severityNumber]

	// Parse severityNumber > 17 (error) to a GCP Error Reporting entry if enabled
	if severityNumber >= 17 && l.cfg.LogConfig.ErrorReportingType {
		if logRecord.Body().Type() != pcommon.ValueTypeMap {
			strValue := logRecord.Body().AsString()
			logRecord.Body().SetEmptyMap()
			logRecord.Body().Map().PutStr("message", strValue)
		}
		logRecord.Body().Map().PutStr(GCPTypeKey, GCPErrorReportingTypeValue)
	}

	// parse remaining OTel attributes to GCP labels
	for k, v := range attrsMap {
		// skip "gcp.*" attributes since we process these to other fields
		if strings.HasPrefix(k, "gcp.") {
			continue
		}
		if _, ok := entry.Labels[k]; !ok {
			entry.Labels[k] = v.AsString()
		}
	}

	// Handle map and bytes as JSON-structured logs if they are successfully converted.
	switch logRecord.Body().Type() {
	case pcommon.ValueTypeMap:
		s, err := structpb.NewStruct(logRecord.Body().Map().AsRaw())
		if err == nil {
			entry.Payload = &logpb.LogEntry_JsonPayload{JsonPayload: s}
			return []*logpb.LogEntry{entry}, nil
		}
		l.obs.log.Warn(fmt.Sprintf("map body cannot be converted to a json payload, exporting as raw string: %+v", err))
	case pcommon.ValueTypeBytes:
		s, err := toProtoStruct(logRecord.Body().Bytes().AsRaw())
		if err == nil {
			entry.Payload = &logpb.LogEntry_JsonPayload{JsonPayload: s}
			return []*logpb.LogEntry{entry}, nil
		}
		l.obs.log.Debug(fmt.Sprintf("bytes body cannot be converted to a json payload, exporting as base64 string: %+v", err))
	}
	// For all other ValueTypes, export as a string payload.

	// log.Body().AsString() can be expensive, and we use it several times below this, so
	// do it once and save that as a variable.
	logBodyString := logRecord.Body().AsString()
	if len(logBodyString) == 0 {
		return []*logpb.LogEntry{entry}, nil
	}

	// Calculate the size of the internal log entry so this overhead can be accounted
	// for when determining the need to split based on payload size
	// TODO(damemi): Find an appropriate estimated buffer to account for the LogSplit struct as well
	overheadBytes := proto.Size(entry)
	// Split log entries with a string payload into fewer entries
	splits := int(math.Ceil(float64(len([]byte(logBodyString))) / float64(l.maxEntrySize-overheadBytes)))
	if splits <= 1 {
		entry.Payload = &logpb.LogEntry_TextPayload{TextPayload: logBodyString}
		return []*logpb.LogEntry{entry}, nil
	}
	entries := make([]*logpb.LogEntry, splits)
	// Start by assuming all splits will be even (this may not be the case)
	startIndex := 0
	endIndex := int(math.Floor((1.0 / float64(splits)) * float64(len(logBodyString))))
	for i := 0; i < splits; i++ {
		newEntry := proto.Clone(entry).(*logpb.LogEntry)
		currentSplit := logBodyString[startIndex:endIndex]

		// If the current split is larger than the entry size, iterate until it is within the max
		// (This may happen since not all characters are exactly 1 byte)
		for len([]byte(currentSplit)) > l.maxEntrySize {
			endIndex--
			currentSplit = logBodyString[startIndex:endIndex]
		}
		newEntry.Payload = &logpb.LogEntry_TextPayload{TextPayload: currentSplit}
		newEntry.Split = &logpb.LogSplit{
			Uid:         fmt.Sprintf("%s-%s", logName, entry.Timestamp.AsTime().String()),
			Index:       int32(i),
			TotalSplits: int32(splits),
		}
		entries[i] = newEntry

		// Update slice indices to the next chunk
		startIndex = endIndex
		endIndex = int(math.Floor((float64(i+2) / float64(splits)) * float64(len(logBodyString))))
	}
	return entries, nil
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
	Status                         int32  `json:"status,string"`
	CacheLookup                    bool   `json:"cacheLookup"`
	CacheHit                       bool   `json:"cacheHit"`
	CacheValidatedWithOriginServer bool   `json:"cacheValidatedWithOriginServer"`
}

func (l logMapper) parseHTTPRequest(httpRequestAttr pcommon.Value) (*logtypepb.HttpRequest, error) {
	var parsedHTTPRequest httpRequestLog
	err := unmarshalAttribute(httpRequestAttr, &parsedHTTPRequest)
	if err != nil {
		return nil, &attributeProcessingError{Key: HTTPRequestAttributeKey, Err: err}
	}

	pb := &logtypepb.HttpRequest{
		RequestMethod:                  parsedHTTPRequest.RequestMethod,
		RequestUrl:                     fixUTF8(parsedHTTPRequest.RequestURL),
		RequestSize:                    parsedHTTPRequest.RequestSize,
		Status:                         parsedHTTPRequest.Status,
		ResponseSize:                   parsedHTTPRequest.ResponseSize,
		UserAgent:                      parsedHTTPRequest.UserAgent,
		ServerIp:                       parsedHTTPRequest.ServerIP,
		RemoteIp:                       parsedHTTPRequest.RemoteIP,
		Referer:                        parsedHTTPRequest.Referer,
		CacheHit:                       parsedHTTPRequest.CacheHit,
		CacheValidatedWithOriginServer: parsedHTTPRequest.CacheValidatedWithOriginServer,
		Protocol:                       "HTTP/1.1",
		CacheFillBytes:                 parsedHTTPRequest.CacheFillBytes,
		CacheLookup:                    parsedHTTPRequest.CacheLookup,
	}
	if parsedHTTPRequest.Latency != "" {
		latency, err := time.ParseDuration(parsedHTTPRequest.Latency)
		if err == nil && latency != 0 {
			pb.Latency = durationpb.New(latency)
		}
	}
	return pb, nil
}

// toProtoStruct converts v, which must marshal into a JSON object,
// into a Google Struct proto.
// Mostly copied from
// https://github.com/googleapis/google-cloud-go/blob/69705144832c715cf23832602ad9338b911dff9a/logging/logging.go#L577
func toProtoStruct(v any) (*structpb.Struct, error) {
	// v is a Go value that supports JSON marshaling. We want a Struct
	// protobuf. Some day we may have a more direct way to get there, but right
	// now the only way is to marshal the Go value to JSON, unmarshal into a
	// map, and then build the Struct proto from the map.
	jb, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("logging: json.Marshal: %w", err)
	}
	var m map[string]any
	err = json.Unmarshal(jb, &m)
	if err != nil {
		return nil, fmt.Errorf("logging: json.Unmarshal: %w", err)
	}
	return structpb.NewStruct(m)
}

// fixUTF8 is a helper that fixes an invalid UTF-8 string by replacing
// invalid UTF-8 runes with the Unicode replacement character (U+FFFD).
// See Issue https://github.com/googleapis/google-cloud-go/issues/1383.
// Coped from https://github.com/googleapis/google-cloud-go/blob/69705144832c715cf23832602ad9338b911dff9a/logging/logging.go#L557
func fixUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}

	// Otherwise time to build the sequence.
	buf := new(bytes.Buffer)
	buf.Grow(len(s))
	for _, r := range s {
		if utf8.ValidRune(r) {
			buf.WriteRune(r)
		} else {
			buf.WriteRune('\uFFFD')
		}
	}
	return buf.String()
}

func unmarshalAttribute(v pcommon.Value, out any) error {
	var valueBytes []byte
	switch v.Type() {
	case pcommon.ValueTypeBytes:
		valueBytes = v.Bytes().AsRaw()
	case pcommon.ValueTypeMap, pcommon.ValueTypeStr:
		valueBytes = []byte(v.AsString())
	default:
		return &unsupportedValueTypeError{ValueType: v.Type()}
	}
	// TODO: Investigate doing this without the JSON unmarshal. Getting the attribute as a map
	// instead of a slice of bytes could do, but would need a lot of type casting and checking
	// assertions with it.
	return json.Unmarshal(valueBytes, out)
}
