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
	"io"
	"net/http"
	"testing"
	"time"

	"cloud.google.com/go/logging"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	logpb "google.golang.org/genproto/googleapis/logging/v2"
)

func newTestLogMapper(entrySize int) logMapper {
	obs := selfObservability{log: zap.NewNop()}
	cfg := DefaultConfig()
	cfg.LogConfig.DefaultLogName = "default-log"
	return logMapper{
		cfg:          cfg,
		obs:          obs,
		maxEntrySize: entrySize,
	}
}

func TestLogMapping(t *testing.T) {
	testObservedTime, _ := time.Parse("2006-01-02", "2022-04-12")
	testSampleTime, _ := time.Parse("2006-01-02", "2021-07-10")
	testTraceID := pcommon.NewTraceID([16]byte{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
	})
	testSpanID := pcommon.NewSpanID([8]byte{
		0, 0, 0, 0, 0, 0, 0, 1,
	})

	testCases := []struct {
		name            string
		log             func() plog.LogRecord
		mr              func() *monitoredres.MonitoredResource
		expectError     bool
		expectedEntries []logging.Entry
		maxEntrySize    int
	}{
		{
			name:         "split entry size",
			maxEntrySize: 3 + 38, // 3 bytes for payload + 38 for overhead
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Body().SetStringVal("abcxyz")
				return log
			},
			expectedEntries: []logging.Entry{
				{
					Payload:   "abc",
					Timestamp: testObservedTime,
				},
				{
					Payload:   "xyz",
					Timestamp: testObservedTime,
				},
			},
			mr: func() *monitoredres.MonitoredResource {
				return nil
			},
		},
		{
			name: "empty log, empty monitoredresource",
			log: func() plog.LogRecord {
				return plog.NewLogRecord()
			},
			mr: func() *monitoredres.MonitoredResource {
				return nil
			},
			expectedEntries: []logging.Entry{{
				Timestamp: testObservedTime,
			}},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log with json, empty monitoredresource",
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Body().SetMBytesVal([]byte(`{"this": "is json"}`))
				return log
			},
			mr: func() *monitoredres.MonitoredResource {
				return nil
			},
			expectedEntries: []logging.Entry{
				{
					Payload:   []byte(`{"this": "is json"}`),
					Timestamp: testObservedTime,
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log with json and httpRequest, empty monitoredresource",
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Body().SetMBytesVal([]byte(`{"message": "hello!"}`))
				log.Attributes().Insert(HTTPRequestAttributeKey, pcommon.NewValueBytes([]byte(`{
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
					}`)))
				return log
			},
			mr: func() *monitoredres.MonitoredResource {
				return nil
			},
			expectedEntries: []logging.Entry{
				{
					Payload:   []byte(`{"message": "hello!"}`),
					Timestamp: testObservedTime,
					HTTPRequest: &logging.HTTPRequest{
						Request:                        makeExpectedHTTPReq("GET", "https://www.example.com", "https://www.example2.com", "test", nil),
						RequestSize:                    1,
						Status:                         200,
						ResponseSize:                   1,
						LocalIP:                        "192.168.0.2",
						RemoteIP:                       "192.168.0.1",
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
			name: "log with timestamp",
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.SetTimestamp(pcommon.NewTimestampFromTime(testSampleTime))
				return log
			},
			mr: func() *monitoredres.MonitoredResource {
				return nil
			},
			expectedEntries: []logging.Entry{
				{
					Timestamp: testSampleTime,
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log body with string value",
			mr: func() *monitoredres.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Body().SetStringVal("{\"message\": \"hello!\"}")
				return log
			},
			expectedEntries: []logging.Entry{
				{
					Payload:   `{"message": "hello!"}`,
					Timestamp: testObservedTime,
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			// TODO(damemi): parse/test sourceLocation from more than just bytes values
			name: "log with sourceLocation (bytes)",
			mr: func() *monitoredres.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Attributes().Insert(
					SourceLocationAttributeKey,
					pcommon.NewValueBytes([]byte(`{"file": "test.php", "line":100, "function":"helloWorld"}`)),
				)
				return log
			},
			expectedEntries: []logging.Entry{
				{
					SourceLocation: &logpb.LogEntrySourceLocation{
						File:     "test.php",
						Line:     100,
						Function: "helloWorld",
					},
					Timestamp: testObservedTime,
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log with trace and span id",
			mr: func() *monitoredres.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.SetTraceID(testTraceID)
				log.SetSpanID(testSpanID)
				return log
			},
			expectedEntries: []logging.Entry{
				{
					Trace:     testTraceID.HexString(),
					SpanID:    testSpanID.HexString(),
					Timestamp: testObservedTime,
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log with severity number",
			mr: func() *monitoredres.MonitoredResource {
				return nil
			},
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.SetSeverityNumber(plog.SeverityNumberFATAL)
				return log
			},
			expectedEntries: []logging.Entry{
				{
					Timestamp: testObservedTime,
					Severity:  logging.Critical,
				},
			},
			maxEntrySize: defaultMaxEntrySize,
		},
		{
			name: "log with invalid severity number",
			mr: func() *monitoredres.MonitoredResource {
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
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			log := testCase.log()
			mr := testCase.mr()
			mapper := newTestLogMapper(testCase.maxEntrySize)
			entries, _, err := mapper.logToSplitEntries(
				log,
				mr,
				"",
				"",
				testObservedTime)

			if testCase.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, testCase.expectedEntries, entries)
			}
		})
	}
}

func TestGetLogName(t *testing.T) {
	testCases := []struct {
		name         string
		log          func() plog.LogRecord
		expectError  bool
		expectedName string
	}{
		{
			name: "log with name attribute",
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Attributes().Insert(LogNameAttributeKey, pcommon.NewValueString("foo-log"))
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

func makeExpectedHTTPReq(method, url, referer, userAgent string, body io.Reader) *http.Request {
	req, _ := http.NewRequest(method, url, body)
	req.Header.Set("Referer", referer)
	req.Header.Set("User-Agent", userAgent)
	return req
}
