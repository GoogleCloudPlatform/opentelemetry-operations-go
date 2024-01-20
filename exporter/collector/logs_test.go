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
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"cloud.google.com/go/logging"
	logpb "cloud.google.com/go/logging/apiv2/loggingpb"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	logtypepb "google.golang.org/genproto/googleapis/logging/type"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Option func(*Config)

func newTestLogMapper(entrySize int, opts ...Option) logMapper {
	obs := selfObservability{log: zap.NewNop()}
	cfg := DefaultConfig()
	cfg.LogConfig.DefaultLogName = "default-log"
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return logMapper{
		cfg:          cfg,
		obs:          obs,
		maxEntrySize: entrySize,
	}
}

func TestLogMapping(t *testing.T) {
	testObservedTime, _ := time.Parse("2006-01-02", "2022-04-12")
	testSampleTime, _ := time.Parse("2006-01-02", "2021-07-10")
	testTraceID := pcommon.TraceID([16]byte{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
	})
	testSpanID := pcommon.SpanID([8]byte{
		0, 0, 0, 0, 0, 0, 0, 1,
	})
	logName := "projects/fakeprojectid/logs/default-log"

	testCases := []struct {
		expectedError   error
		log             func() plog.LogRecord
		mr              func() *monitoredrespb.MonitoredResource
		config          Option
		name            string
		expectedEntries []*logpb.LogEntry
		expectError     bool
		maxEntrySize    int
	}{
		{
			name:         "split entry size",
			maxEntrySize: 3 + 51, // 3 bytes for payload + 51 for overhead
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Body().SetStr("abcxyz")
				return log
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Payload:   &logpb.LogEntry_TextPayload{TextPayload: "abc"},
					Timestamp: timestamppb.New(testObservedTime),
					Split: &logpb.LogSplit{
						Uid:         fmt.Sprintf("default-log-%s", testObservedTime.String()),
						Index:       0,
						TotalSplits: 2,
					},
				},
				{
					LogName:   logName,
					Payload:   &logpb.LogEntry_TextPayload{TextPayload: "xyz"},
					Timestamp: timestamppb.New(testObservedTime),
					Split: &logpb.LogSplit{
						Uid:         fmt.Sprintf("default-log-%s", testObservedTime.String()),
						Index:       1,
						TotalSplits: 2,
					},
				},
			},
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
		},
		{
			name: "empty log, empty monitoredresource",
			log: func() plog.LogRecord {
				return plog.NewLogRecord()
			},
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			expectedEntries: []*logpb.LogEntry{{
				LogName:   logName,
				Timestamp: timestamppb.New(testObservedTime),
			}},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log with json, empty monitoredresource",
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Body().SetEmptyMap().PutStr("this", "is json")
				return log
			},
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Timestamp: timestamppb.New(testObservedTime),
					Payload: &logpb.LogEntry_JsonPayload{JsonPayload: &structpb.Struct{Fields: map[string]*structpb.Value{
						"this": {Kind: &structpb.Value_StringValue{StringValue: "is json"}},
					}}},
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		}, {
			name: "log with invalid json byte body returns raw byte string",
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Body().SetEmptyBytes().FromRaw([]byte(`"this is not json"`))
				return log
			},
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Payload:   &logpb.LogEntry_TextPayload{TextPayload: "InRoaXMgaXMgbm90IGpzb24i"},
					Timestamp: timestamppb.New(testObservedTime),
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log with json and httpRequest, empty monitoredresource",
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Body().SetEmptyMap().PutStr("message", "hello!")
				log.Attributes().PutEmptyBytes(HTTPRequestAttributeKey).FromRaw([]byte(`{
						"requestMethod": "GET",
						"requestURL": "https://www.example.com",
						"requestSize": "1",
						"status": "200",
						"responseSize": "1",
						"userAgent": "test",
						"remoteIP": "192.168.0.1",
						"serverIP": "192.168.0.2",
						"referer": "https://www.example2.com",
						"cacheHit": false,
						"cacheValidatedWithOriginServer": false,
						"cacheFillBytes": "1",
						"protocol": "HTTP/2"
					}`))
				return log
			},
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Timestamp: timestamppb.New(testObservedTime),
					Payload: &logpb.LogEntry_JsonPayload{JsonPayload: &structpb.Struct{Fields: map[string]*structpb.Value{
						"message": {Kind: &structpb.Value_StringValue{StringValue: "hello!"}},
					}}},
					HttpRequest: &logtypepb.HttpRequest{
						RequestMethod:                  "GET",
						UserAgent:                      "test",
						Referer:                        "https://www.example2.com",
						RequestUrl:                     "https://www.example.com",
						Protocol:                       "HTTP/1.1",
						RequestSize:                    1,
						Status:                         200,
						ResponseSize:                   1,
						ServerIp:                       "192.168.0.2",
						RemoteIp:                       "192.168.0.1",
						CacheHit:                       false,
						CacheValidatedWithOriginServer: false,
						CacheFillBytes:                 1,
						CacheLookup:                    false,
					},
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log with httpRequest attribute unsupported type",
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Body().SetEmptyMap().PutStr("message", "hello!")
				log.Attributes().PutBool(HTTPRequestAttributeKey, true)
				return log
			},
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Timestamp: timestamppb.New(testObservedTime),
					Payload: &logpb.LogEntry_JsonPayload{JsonPayload: &structpb.Struct{Fields: map[string]*structpb.Value{
						"message": {Kind: &structpb.Value_StringValue{StringValue: "hello!"}},
					}}},
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log with timestamp",
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.SetTimestamp(pcommon.NewTimestampFromTime(testSampleTime))
				return log
			},
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Timestamp: timestamppb.New(testSampleTime),
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log with empty timestamp, but set observed_time",
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.SetObservedTimestamp(pcommon.NewTimestampFromTime(testSampleTime))
				return log
			},
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Timestamp: timestamppb.New(testSampleTime),
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log body with string value",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Body().SetStr("{\"message\": \"hello!\"}")
				return log
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Timestamp: timestamppb.New(testObservedTime),
					Payload:   &logpb.LogEntry_TextPayload{TextPayload: `{"message": "hello!"}`},
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log body with string value and error converted to json payload",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.SetSeverityNumber(18)
				log.Body().SetStr("{\"message\": \"hello!\"}")
				return log
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Timestamp: timestamppb.New(testObservedTime),
					Severity:  logtypepb.LogSeverity(logging.Error),
					Payload: &logpb.LogEntry_JsonPayload{JsonPayload: &structpb.Struct{Fields: map[string]*structpb.Value{
						GCPTypeKey: {Kind: &structpb.Value_StringValue{StringValue: GCPErrorReportingTypeValue}},
						"message":  {Kind: &structpb.Value_StringValue{StringValue: `{"message": "hello!"}`}},
					}}},
				},
			},
			maxEntrySize: defaultMaxEntrySize,
			config: func(cfg *Config) {
				cfg.LogConfig.ErrorReportingType = true
			},
		},
		{
			name: "log body with simple string value and error converted to json payload",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.SetSeverityNumber(18)
				log.Body().SetStr("test string message")
				return log
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Timestamp: timestamppb.New(testObservedTime),
					Severity:  logtypepb.LogSeverity(logging.Error),
					Payload: &logpb.LogEntry_JsonPayload{JsonPayload: &structpb.Struct{Fields: map[string]*structpb.Value{
						GCPTypeKey: {Kind: &structpb.Value_StringValue{StringValue: GCPErrorReportingTypeValue}},
						"message":  {Kind: &structpb.Value_StringValue{StringValue: "test string message"}},
					}}},
				},
			},
			maxEntrySize: defaultMaxEntrySize,
			config: func(cfg *Config) {
				cfg.LogConfig.ErrorReportingType = true
			},
		},
		{
			name: "log body with simple string value and non-error is not converted to json payload",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.SetSeverityNumber(16)
				log.Body().SetStr("test string message")
				return log
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Timestamp: timestamppb.New(testObservedTime),
					Severity:  logtypepb.LogSeverity(logging.Warning),
					Payload:   &logpb.LogEntry_TextPayload{TextPayload: "test string message"},
				},
			},
			maxEntrySize: defaultMaxEntrySize,
			config: func(cfg *Config) {
				cfg.LogConfig.ErrorReportingType = true
			},
		},
		{
			name: "log body with map value and error converted to json payload",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.SetSeverityNumber(18)
				log.Body().SetEmptyMap().PutStr("msg", "test map value")
				return log
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Timestamp: timestamppb.New(testObservedTime),
					Severity:  logtypepb.LogSeverity(logging.Error),
					Payload: &logpb.LogEntry_JsonPayload{JsonPayload: &structpb.Struct{Fields: map[string]*structpb.Value{
						GCPTypeKey: {Kind: &structpb.Value_StringValue{StringValue: GCPErrorReportingTypeValue}},
						"msg":      {Kind: &structpb.Value_StringValue{StringValue: "test map value"}},
					}}},
				},
			},
			maxEntrySize: defaultMaxEntrySize,
			config: func(cfg *Config) {
				cfg.LogConfig.ErrorReportingType = true
			},
		},
		{
			name: "log with valid sourceLocation (bytes)",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Attributes().PutEmptyBytes(SourceLocationAttributeKey).FromRaw(
					[]byte(`{"file": "test.php", "line":100, "function":"helloWorld"}`),
				)
				return log
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Timestamp: timestamppb.New(testObservedTime),
					SourceLocation: &logpb.LogEntrySourceLocation{
						File:     "test.php",
						Line:     100,
						Function: "helloWorld",
					},
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log with invalid sourceLocation (bytes)",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Attributes().PutEmptyBytes(SourceLocationAttributeKey).FromRaw(
					[]byte(`{"file": 100}`),
				)
				return log
			},
			maxEntrySize: defaultMaxEntrySize,
			expectError:  true,
		},
		{
			name: "log with valid sourceLocation (map)",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				sourceLocationMap := log.Attributes().PutEmptyMap(SourceLocationAttributeKey)
				sourceLocationMap.PutStr("file", "test.php")
				sourceLocationMap.PutInt("line", 100)
				sourceLocationMap.PutStr("function", "helloWorld")
				return log
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Timestamp: timestamppb.New(testObservedTime),
					SourceLocation: &logpb.LogEntrySourceLocation{
						File:     "test.php",
						Line:     100,
						Function: "helloWorld",
					},
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log with invalid sourceLocation (map)",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				sourceLocationMap := log.Attributes().PutEmptyMap(SourceLocationAttributeKey)
				sourceLocationMap.PutStr("line", "100")
				return log
			},
			maxEntrySize: defaultMaxEntrySize,
			expectError:  true,
		},
		{
			name: "log with valid sourceLocation (string)",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Attributes().PutStr(
					SourceLocationAttributeKey,
					`{"file": "test.php", "line":100, "function":"helloWorld"}`,
				)
				return log
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Timestamp: timestamppb.New(testObservedTime),
					SourceLocation: &logpb.LogEntrySourceLocation{
						File:     "test.php",
						Line:     100,
						Function: "helloWorld",
					},
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log with invalid sourceLocation (string)",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Attributes().PutStr(
					SourceLocationAttributeKey,
					`{"file": 100}`,
				)
				return log
			},
			maxEntrySize: defaultMaxEntrySize,
			expectError:  true,
		},
		{
			name: "log with unsupported sourceLocation type",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Attributes().PutBool(SourceLocationAttributeKey, true)
				return log
			},
			maxEntrySize: defaultMaxEntrySize,
			expectError:  true,
			expectedError: &AttributeProcessingError{
				Key: SourceLocationAttributeKey,
				Err: &UnsupportedValueTypeError{ValueType: pcommon.ValueTypeBool},
			},
		},
		{
			name: "log with traceSampled (bool)",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Attributes().PutBool(TraceSampledAttributeKey, true)
				return log
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:      logName,
					Timestamp:    timestamppb.New(testObservedTime),
					TraceSampled: true,
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log with traceSampled (unsupported type)",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Attributes().PutStr(TraceSampledAttributeKey, "hi!")
				return log
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Timestamp: timestamppb.New(testObservedTime),
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log with trace and span id",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.SetTraceID(testTraceID)
				log.SetSpanID(testSpanID)
				return log
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Timestamp: timestamppb.New(testObservedTime),
					Trace:     fmt.Sprintf("projects/fakeprojectid/traces/%s", hex.EncodeToString(testTraceID[:])),
					SpanId:    hex.EncodeToString(testSpanID[:]),
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log with severity number",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.SetSeverityNumber(plog.SeverityNumberFatal)
				return log
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Timestamp: timestamppb.New(testObservedTime),
					Severity:  logtypepb.LogSeverity(logging.Critical),
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log with invalid severity number",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.SetSeverityNumber(plog.SeverityNumber(100))
				return log
			},
			maxEntrySize: defaultMaxEntrySize,
			expectError:  true,
		},
		{
			name: "log with severity text",
			mr: func() *monitoredrespb.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.SetSeverityText("fatal3")
				return log
			},
			expectedEntries: []*logpb.LogEntry{
				{
					LogName:   logName,
					Timestamp: timestamppb.New(testObservedTime),
					Severity:  logtypepb.LogSeverity(logging.Alert),
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			log := testCase.log()
			mr := testCase.mr()
			mapper := newTestLogMapper(testCase.maxEntrySize, testCase.config)
			logName, _ := mapper.getLogName(log)
			entries, err := mapper.logToSplitEntries(
				log,
				mr,
				nil,
				testObservedTime,
				logName,
				"fakeprojectid",
			)

			if testCase.expectError {
				assert.NotNil(t, err)
				if testCase.expectedError != nil {
					assert.Equal(t, err.Error(), testCase.expectedError.Error())
				}
			} else {
				assert.Nil(t, err)
				assert.Equal(t, len(testCase.expectedEntries), len(entries))
				for i := range testCase.expectedEntries {
					if !proto.Equal(testCase.expectedEntries[i], entries[i]) {
						assert.Equal(t, testCase.expectedEntries[i], entries[i])
					}
				}
			}
		})
	}
}

func TestGetLogName(t *testing.T) {
	testCases := []struct {
		log          func() plog.LogRecord
		name         string
		expectedName string
		expectError  bool
	}{
		{
			name: "log with name attribute",
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Attributes().PutStr(LogNameAttributeKey, "foo-log")
				return log
			},
			expectedName: "foo-log",
		},
		{
			name: "log with no name attribute",
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				return log
			},
			expectedName: "default-log",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			log := testCase.log()
			mapper := newTestLogMapper(defaultMaxEntrySize)
			name, err := mapper.getLogName(log)
			if testCase.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, testCase.expectedName, name)
			}
		})
	}
}
