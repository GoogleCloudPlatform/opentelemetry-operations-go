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
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"cloud.google.com/go/logging"
	loggingv2 "cloud.google.com/go/logging/apiv2"
	logpb "cloud.google.com/go/logging/apiv2/loggingpb"
	"github.com/golang/protobuf/ptypes"
	"github.com/googleapis/gax-go/v2"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	logtypepb "google.golang.org/genproto/googleapis/logging/type"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

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
	obs            selfObservability
	cfg            Config
	maxEntrySize   int
	maxRequestSize int
}

func NewGoogleCloudLogsExporter(
	ctx context.Context,
	cfg Config,
	log *zap.Logger,
) (*LogsExporter, error) {
	clientOpts, err := generateClientOptions(ctx, &cfg.LogConfig.ClientConfig, &cfg, loggingv2.DefaultAuthScopes())
	if err != nil {
		return nil, err
	}

	loggingClient, err := loggingv2.NewClient(ctx, clientOpts...)
	if err != nil {
		return nil, err
	}

	if cfg.LogConfig.ClientConfig.Compression == gzip.Name {
		loggingClient.CallOptions.WriteLogEntries = append(loggingClient.CallOptions.WriteLogEntries,
			gax.WithGRPCOptions(grpc.UseCompressor(gzip.Name)))
	}

	obs := selfObservability{
		log: log,
	}

	return &LogsExporter{
		cfg: cfg,
		obs: obs,
		mapper: logMapper{
			obs:            obs,
			cfg:            cfg,
			maxEntrySize:   defaultMaxEntrySize,
			maxRequestSize: defaultMaxRequestSize,
		},

		loggingClient: loggingClient,
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

func (l *LogsExporter) Shutdown(ctx context.Context) error {
	return l.loggingClient.Close()
}

func (l *LogsExporter) PushLogs(ctx context.Context, ld plog.Logs) error {
	projectEntries, err := l.mapper.createEntries(ld)
	if err != nil {
		return err
	}

	errors := []error{}
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
			if err != nil {
				errors = append(errors, err)
			}

			entries = entries[entry:]
			entry = 0
			currentBatchSize = 0
		}
	}

	if len(errors) > 0 {
		return multierr.Combine(errors...)
	}
	return nil
}

func (l logMapper) createEntries(ld plog.Logs) (map[string][]*logpb.LogEntry, error) {
	// if destination_project_quota is enabled, projectMapKey will be the name of the project for each batch of entries
	// otherwise, we can mix project entries for more efficient batching and store all entries in a single list
	projectMapKey := ""
	errors := []error{}
	entries := make(map[string][]*logpb.LogEntry)
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		mr := defaultResourceToMonitoredResource(rl.Resource())
		extraResourceLabels := resourceToLabels(rl.Resource(), l.cfg.LogConfig.ServiceResourceLabels, l.cfg.LogConfig.ResourceFilters, l.obs.log)
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
					errors = append(errors, err)
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
					errors = append(errors, err)
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

	return entries, multierr.Combine(errors...)
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

func (l logMapper) logEntryToInternal(
	entry logging.Entry,
	logName string,
	projectID string,
	mr *monitoredrespb.MonitoredResource,
	splits int,
	splitIndex int,
) (*logpb.LogEntry, error) {
	internalLogEntry, err := toLogEntry(entry, fmt.Sprintf("projects/%s", projectID))
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

func toLogEntry(e logging.Entry, parent string) (*logpb.LogEntry, error) {
	parent, err := makeParent(parent)
	if err != nil {
		return nil, err
	}
	return toLogEntryInternal(e, parent, 1)
}

func toLogEntryInternal(e logging.Entry, parent string, skipLevels int) (*logpb.LogEntry, error) {
	if e.LogName != "" {
		return nil, errors.New("logging: Entry.LogName should be not be set when writing")
	}
	// t := e.Timestamp
	// if t.IsZero() {
	// 	t = time.Now()
	// }
	// ts := timestamppb.New(t)
	if e.Trace == "" {
		populateTraceInfo(&e, nil)
		// format trace
		if e.Trace != "" && !strings.Contains(e.Trace, "/traces/") {
			e.Trace = fmt.Sprintf("%s/traces/%s", parent, e.Trace)
		}
	}
	req, err := fromHTTPRequest(e.HTTPRequest)
	if err != nil {
		return nil, err
	}
	ent := &logpb.LogEntry{
		// Timestamp:      ts,
		Severity:    logtypepb.LogSeverity(e.Severity),
		InsertId:    e.InsertID,
		HttpRequest: req,
		Operation:   e.Operation,
		Labels:      e.Labels,
		// Trace:       e.Trace,
		// SpanId:      e.SpanID,
		// Resource:       e.Resource,
		SourceLocation: e.SourceLocation,
		TraceSampled:   e.TraceSampled,
	}
	switch p := e.Payload.(type) {
	case string:
		ent.Payload = &logpb.LogEntry_TextPayload{TextPayload: p}
	case *anypb.Any:
		ent.Payload = &logpb.LogEntry_ProtoPayload{ProtoPayload: p}
	default:
		s, err := toProtoStruct(p)
		if err != nil {
			return nil, err
		}
		ent.Payload = &logpb.LogEntry_JsonPayload{JsonPayload: s}
	}
	return ent, nil
}

// toProtoStruct converts v, which must marshal into a JSON object,
// into a Google Struct proto.
func toProtoStruct(v interface{}) (*structpb.Struct, error) {
	// Fast path: if v is already a *structpb.Struct, nothing to do.
	if s, ok := v.(*structpb.Struct); ok {
		return s, nil
	}
	// v is a Go value that supports JSON marshalling. We want a Struct
	// protobuf. Some day we may have a more direct way to get there, but right
	// now the only way is to marshal the Go value to JSON, unmarshal into a
	// map, and then build the Struct proto from the map.
	var jb []byte
	var err error
	if raw, ok := v.(json.RawMessage); ok { // needed for Go 1.7 and below
		jb = []byte(raw)
	} else {
		jb, err = json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("logging: json.Marshal: %w", err)
		}
	}
	var m map[string]interface{}
	err = json.Unmarshal(jb, &m)
	if err != nil {
		// return nil, fmt.Errorf("logging: json.Unmarshal: %w", err)
		return nil, fmt.Errorf("logging: json.Unmarshal(%v): %w", jb, err)
	}
	return jsonMapToProtoStruct(m), nil
}

func jsonMapToProtoStruct(m map[string]interface{}) *structpb.Struct {
	fields := map[string]*structpb.Value{}
	for k, v := range m {
		fields[k] = jsonValueToStructValue(v)
	}
	return &structpb.Struct{Fields: fields}
}

func jsonValueToStructValue(v interface{}) *structpb.Value {
	switch x := v.(type) {
	case bool:
		return &structpb.Value{Kind: &structpb.Value_BoolValue{BoolValue: x}}
	case float64:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: x}}
	case string:
		return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: x}}
	case nil:
		return &structpb.Value{Kind: &structpb.Value_NullValue{}}
	case map[string]interface{}:
		return &structpb.Value{Kind: &structpb.Value_StructValue{StructValue: jsonMapToProtoStruct(x)}}
	case []interface{}:
		var vals []*structpb.Value
		for _, e := range x {
			vals = append(vals, jsonValueToStructValue(e))
		}
		return &structpb.Value{Kind: &structpb.Value_ListValue{ListValue: &structpb.ListValue{Values: vals}}}
	default:
		return &structpb.Value{Kind: &structpb.Value_NullValue{}}
	}
}

func populateTraceInfo(e *logging.Entry, req *http.Request) bool {
	if req == nil {
		if e.HTTPRequest != nil && e.HTTPRequest.Request != nil {
			req = e.HTTPRequest.Request
		} else {
			return false
		}
	}
	header := req.Header.Get("Traceparent")
	if header != "" {
		// do not use traceSampled flag defined by traceparent because
		// flag's definition differs from expected by Cloud Tracing
		traceID, spanID, _ := deconstructTraceParent(header)
		if traceID != "" {
			e.Trace = traceID
			e.SpanID = spanID
			return true
		}
	}
	header = req.Header.Get("X-Cloud-Trace-Context")
	if header != "" {
		traceID, spanID, traceSampled := deconstructXCloudTraceContext(header)
		if traceID != "" {
			e.Trace = traceID
			e.SpanID = spanID
			// enforce sampling if required
			e.TraceSampled = e.TraceSampled || traceSampled
			return true
		}
	}
	return false
}

var validXCloudTraceContext = regexp.MustCompile(
	// Matches on "TRACE_ID"
	`([a-f\d]+)?` +
		// Matches on "/SPAN_ID"
		`(?:/([a-f\d]+))?` +
		// Matches on ";0=TRACE_TRUE"
		`(?:;o=(\d))?`)

func deconstructXCloudTraceContext(s string) (traceID, spanID string, traceSampled bool) {
	// As per the format described at https://cloud.google.com/trace/docs/setup#force-trace
	//    "X-Cloud-Trace-Context: TRACE_ID/SPAN_ID;o=TRACE_TRUE"
	// for example:
	//    "X-Cloud-Trace-Context: 105445aa7843bc8bf206b120001000/1;o=1"
	//
	// We expect:
	//   * traceID (optional): 			"105445aa7843bc8bf206b120001000"
	//   * spanID (optional):       	"1"
	//   * traceSampled (optional): 	true
	matches := validXCloudTraceContext.FindStringSubmatch(s)

	if matches != nil {
		traceID, spanID, traceSampled = matches[1], matches[2], matches[3] == "1"
	}

	if spanID == "0" {
		spanID = ""
	}

	return
}

// As per format described at https://www.w3.org/TR/trace-context/#traceparent-header-field-values
var validTraceParentExpression = regexp.MustCompile(`^(00)-([a-fA-F\d]{32})-([a-f\d]{16})-([a-fA-F\d]{2})$`)

func deconstructTraceParent(s string) (traceID, spanID string, traceSampled bool) {
	matches := validTraceParentExpression.FindStringSubmatch(s)
	if matches != nil {
		// regexp package does not support negative lookahead preventing all 0 validations
		if matches[2] == "00000000000000000000000000000000" || matches[3] == "0000000000000000" {
			return
		}
		flags, err := strconv.ParseInt(matches[4], 16, 16)
		if err == nil {
			traceSampled = (flags & 0x01) == 1
		}
		traceID, spanID = matches[2], matches[3]
	}
	return
}

func fromHTTPRequest(r *logging.HTTPRequest) (*logtypepb.HttpRequest, error) {
	if r == nil {
		return nil, nil
	}
	if r.Request == nil {
		return nil, errors.New("logging: HTTPRequest must have a non-nil Request")
	}
	u := *r.Request.URL
	u.Fragment = ""
	pb := &logtypepb.HttpRequest{
		RequestMethod:                  r.Request.Method,
		RequestUrl:                     fixUTF8(u.String()),
		RequestSize:                    r.RequestSize,
		Status:                         int32(r.Status),
		ResponseSize:                   r.ResponseSize,
		UserAgent:                      r.Request.UserAgent(),
		ServerIp:                       r.LocalIP,
		RemoteIp:                       r.RemoteIP, // TODO(jba): attempt to parse http.Request.RemoteAddr?
		Referer:                        r.Request.Referer(),
		CacheHit:                       r.CacheHit,
		CacheValidatedWithOriginServer: r.CacheValidatedWithOriginServer,
		Protocol:                       r.Request.Proto,
		CacheFillBytes:                 r.CacheFillBytes,
		CacheLookup:                    r.CacheLookup,
	}
	if r.Latency != 0 {
		pb.Latency = ptypes.DurationProto(r.Latency)
	}
	return pb, nil
}

// fixUTF8 is a helper that fixes an invalid UTF-8 string by replacing
// invalid UTF-8 runes with the Unicode replacement character (U+FFFD).
// See Issue https://github.com/googleapis/google-cloud-go/issues/1383.
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

func makeParent(parent string) (string, error) {
	if !strings.ContainsRune(parent, '/') {
		return "projects/" + parent, nil
	}
	prefix := strings.Split(parent, "/")[0]
	if prefix != "projects" && prefix != "folders" && prefix != "billingAccounts" && prefix != "organizations" {
		return parent, fmt.Errorf("parent parameter must start with 'projects/' 'folders/' 'billingAccounts/' or 'organizations/'")
	}
	return parent, nil
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
	mr *monitoredrespb.MonitoredResource,
	logLabels map[string]string,
	processTime time.Time,
	logName string,
	projectID string,
) ([]*logpb.LogEntry, error) {
	// make a copy in case we mutate the record
	logRecord := plog.NewLogRecord()
	log.CopyTo(logRecord)

	entry := &logpb.LogEntry{
		Resource: mr,
	}

	ts := logRecord.Timestamp().AsTime()
	if logRecord.Timestamp() == 0 {
		// if timestamp is unset, fall back to observed_time_unix_nano as recommended
		//   (see https://github.com/open-telemetry/opentelemetry-proto/blob/4abbb78/opentelemetry/proto/logs/v1/logs.proto#L176-L179)
		if logRecord.ObservedTimestamp() != 0 {
			ts = logRecord.ObservedTimestamp().AsTime()
		} else {
			// if observed_time is 0, use the current time
			ts = processTime
		}
	}
	if ts.IsZero() {
		ts = time.Now()
	}
	entry.Timestamp = timestamppb.New(ts)

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
		err := json.Unmarshal(sourceLocation.Bytes().AsRaw(), &logEntrySourceLocation)
		if err != nil {
			return nil, err
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
		var httpRequest *logging.HTTPRequest
		httpRequest, err := l.parseHTTPRequest(httpRequestAttr)
		if err != nil {
			l.obs.log.Debug("Unable to parse httpRequest", zap.Error(err))
		}
		entry.HTTPRequest = httpRequest
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

	entry.Labels = logLabels

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
		return nil, err
	}
	// make a copy so the proto initialization doesn't modify the original entry
	overheadClone := proto.Clone(logOverhead)
	overheadBytes := proto.Size(overheadClone)

	payload, splits, err := parseEntryPayload(logRecord.Body(), l.maxEntrySize-overheadBytes)
	if err != nil {
		return nil, err
	}

	// Split log entries with a string payload into fewer entries
	if splits > 1 {
		entries := make([]*logpb.LogEntry, splits)
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
			entryCopy := entry
			entryCopy.Payload = currentSplit
			e, err := l.logEntryToInternal(entryCopy, logName, projectID, mr, splits, i-1)
			if err != nil {
				return nil, err
			}
			entries[i-1] = e

			// Update slice indices to the next chunk
			startIndex = endIndex
			endIndex = int(math.Floor((float64(i+1) / float64(splits)) * float64(len(payloadString))))
		}
		return entries, nil
	}

	entry.Payload = payload
	e, err := l.logEntryToInternal(entry, logName, projectID, mr, 1, 0)
	if err != nil {
		return nil, err
	}
	return []*logpb.LogEntry{e}, nil
}

func parseEntryPayload(logBody pcommon.Value, maxEntrySize int) (interface{}, int, error) {
	if len(logBody.AsString()) == 0 {
		return nil, 0, nil
	}
	switch logBody.Type() {
	case pcommon.ValueTypeBytes:
		return logBody.Bytes().AsRaw(), 1, nil
	case pcommon.ValueTypeStr:
		return logBody.AsString(), int(math.Ceil(float64(len([]byte(logBody.AsString()))) / float64(maxEntrySize))), nil
	case pcommon.ValueTypeMap:
		return logBody.Map().AsRaw(), 1, nil

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
		bytes = httpRequestAttr.Bytes().AsRaw()
	case pcommon.ValueTypeStr, pcommon.ValueTypeMap:
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
