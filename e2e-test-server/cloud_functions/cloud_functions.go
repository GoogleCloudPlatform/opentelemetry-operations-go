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

package cloudfunctions

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/pubsub"
	// funcframework is required to make this build on cloud functions.  See
	// https://github.com/GoogleCloudPlatform/functions-framework-go/issues/78
	_ "github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/e2e-test-server/scenarios"
)

var projectID string

func init() {
	// Register a CloudEvent function with the Functions Framework
	functions.CloudEvent("HandleCloudFunction", handleCloudFunction)
	projectID = os.Getenv("PROJECT_ID")
	if projectID == "" {
		log.Fatalf("environment variable PROJECT_ID must be set")
	}
}

// PubSubMessage is the payload of a Pub/Sub event.
type PubSubMessage struct {
	Message struct {
		Attributes map[string]string `json:"attributes,omitempty"`
	} `json:"message"`
}

// handleCloudFunction accepts and handles a CloudEvent object. It decodes the
// pubsub message contained within the event and handles it.
func handleCloudFunction(ctx context.Context, e event.Event) error {
	var m PubSubMessage
	if err := json.Unmarshal(e.Data(), &m); err != nil {
		return fmt.Errorf("json.Unmarshal: %w", err)
	}
	pubsubClient, err := pubsub.NewClient(context.Background(), projectID)
	if err != nil {
		return fmt.Errorf("error creating pubsub client: %v", err)
	}
	defer pubsubClient.Close()
	return scenarios.HandleMessage(ctx, pubsubClient, m.Message.Attributes)
}
