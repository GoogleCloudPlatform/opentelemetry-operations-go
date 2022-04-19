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

func newTestLogMapper() logMapper {
	obs := selfObservability{log: zap.NewNop()}
	cfg := DefaultConfig()
	return logMapper{
		cfg: cfg,
		obs: obs,
	}
}

type baseTestCase struct {
	name          string
	log           func() plog.LogRecord
	mr            func() *monitoredres.MonitoredResource
	expectError   bool
	expectedEntry logging.Entry
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

	testCases := []baseTestCase{
		{
			name: "empty log, empty monitoredresource",
			log: func() plog.LogRecord {
				return plog.NewLogRecord()
			},
			mr: func() *monitoredres.MonitoredResource {
				return nil
			},
			expectedEntry: logging.Entry{
				Timestamp: testObservedTime,
			},
		},
		{
			name: "log with json, empty monitoredresource",
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Body().SetBytesVal([]byte(`{"this": "is json"}`))
				return log
			},
			mr: func() *monitoredres.MonitoredResource {
				return nil
			},
			expectedEntry: logging.Entry{
				Payload:   []byte(`{"this": "is json"}`),
				Timestamp: testObservedTime,
			},
		},
		{
			name: "log with json and httpRequest, empty monitoredresource",
			log: func() plog.LogRecord {
				log := plog.NewLogRecord()
				log.Body().SetBytesVal([]byte(`{"message": "hello!"}`))
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
			expectedEntry: logging.Entry{
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
			expectedEntry: logging.Entry{
				Timestamp: testSampleTime,
			},
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
			expectedEntry: logging.Entry{
				Payload:   `{"message": "hello!"}`,
				Timestamp: testObservedTime,
			},
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
					"com.google.sourceLocation",
					pcommon.NewValueBytes([]byte(`{"file": "test.php", "line":100, "function":"helloWorld"}`)),
				)
				return log
			},
			expectedEntry: logging.Entry{
				SourceLocation: &logpb.LogEntrySourceLocation{
					File:     "test.php",
					Line:     100,
					Function: "helloWorld",
				},
				Timestamp: testObservedTime,
			},
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
			expectedEntry: logging.Entry{
				Trace:     testTraceID.HexString(),
				SpanID:    testSpanID.HexString(),
				Timestamp: testObservedTime,
			},
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
			expectedEntry: logging.Entry{
				Timestamp: testObservedTime,
				Severity:  logging.Critical,
			},
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
			expectError: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			log := testCase.log()
			mr := testCase.mr()
			mapper := newTestLogMapper()
			entry, err := mapper.logToEntry(
				log,
				mr,
				"",
				"",
				testObservedTime)
			if testCase.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Equal(t, testCase.expectedEntry, entry)
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
