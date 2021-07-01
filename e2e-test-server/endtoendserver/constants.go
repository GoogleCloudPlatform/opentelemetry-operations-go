// Copyright 2019 OpenTelemetry Authors
// Copyright 2021 Google LLC
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

package endtoendserver

import (
	"log"
	"os"
)

const (
	instrumentingModuleName = "opentelemetry-ops-e2e-test-server"
	scenarioKey             = "scenario"
	testIDKey               = "test_id"
	statusCodeKey           = "status_code"
)

var (
	subscriptionMode        = mustGetenv("SUBSCRIPTION_MODE")
	projectID               = mustGetenv("PROJECT_ID")
	requestSubscriptionName = mustGetenv("REQUEST_SUBSCRIPTION_NAME")
	responseTopicName       = mustGetenv("RESPONSE_TOPIC_NAME")
)

func mustGetenv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("environment variable %q must be set", key)
	}
	return v
}
