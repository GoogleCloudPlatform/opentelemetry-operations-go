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
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"cloud.google.com/go/logging"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/api/monitoredres"
)

func newTestLogMapper(loggingConfig LoggingConfig) logMapper {
	obs := selfObservability{log: zap.NewNop()}
	cfg := DefaultConfig()
	cfg.LoggingConfig = loggingConfig
	return logMapper{
		cfg: cfg,
		obs: obs,
	}
}

type baseTestCase struct {
	name          string
	loggingConfig LoggingConfig
	log           func() pdata.LogRecord
	mr            func() *monitoredres.MonitoredResource
	expectError   bool
	expectedEntry logging.Entry
}

func TestLogMapping(t *testing.T) {
	testCases := []baseTestCase{
		{
			name: "empty log, empty monitoredresource",
			log: func() pdata.LogRecord {
				return pdata.NewLogRecord()
			},
			mr: func() *monitoredres.MonitoredResource {
				return nil
			},
			expectedEntry: logging.Entry{},
		},
		{
			name: "log with json, empty monitoredresource",
			log: func() pdata.LogRecord {
				log := pdata.NewLogRecord()
				log.Body().SetBytesVal([]byte(`{"this": "is json"}`))
				return log
			},
			mr: func() *monitoredres.MonitoredResource {
				return nil
			},
			expectedEntry: logging.Entry{Payload: json.RawMessage(`{"this": "is json"}`)},
		},
		{
			name: "log with json and httpRequest, empty monitoredresource",
			loggingConfig: LoggingConfig{
				ParseHTTPRequest: true,
			},
			log: func() pdata.LogRecord {
				log := pdata.NewLogRecord()
				log.Body().SetBytesVal([]byte(`{
					"httpRequest": {
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
					},
					"message": "hello!"
				}`))
				return log
			},
			mr: func() *monitoredres.MonitoredResource {
				return nil
			},
			expectedEntry: logging.Entry{
				Payload: json.RawMessage(`{"message":"hello!"}`),
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
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			log := testCase.log()
			mr := testCase.mr()
			mapper := newTestLogMapper(testCase.loggingConfig)
			entry, err := mapper.logToEntry(log, mr)
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
