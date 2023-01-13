// Copyright 2023 Google LLC
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

package scenarios

import (
	"log"
	"os"
	"time"
)

const (
	instrumentingModuleName = "opentelemetry-ops-e2e-test-server"
	scenarioKey             = "scenario"
	testIDKey               = "test_id"
	statusCodeKey           = "status_code"
	traceIDKey              = "trace_id"

	// This is set small to reduce the latency in sending traces so that the tests finish faster.
	traceBatchTimeout = 100 * time.Millisecond
)

var (
	projectID         string
	responseTopicName string
)

func init() {
	projectID = os.Getenv("PROJECT_ID")
	if projectID == "" {
		log.Fatalf("environment variable PROJECT_ID must be set")
	}
	responseTopicName = os.Getenv("RESPONSE_TOPIC_NAME")
	if responseTopicName == "" {
		log.Fatalf("environment variable RESPONSE_TOPIC_NAME must be set")
	}
}
