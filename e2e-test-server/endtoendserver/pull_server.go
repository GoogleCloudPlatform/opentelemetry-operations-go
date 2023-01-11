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

package endtoendserver

import (
	"context"
	"log"

	"cloud.google.com/go/pubsub"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/e2e-test-server/scenarios"
)

// pullServer is an end-to-end test service.
type pullServer struct {
	pubsubClient *pubsub.Client
}

// New instantiates a new end-to-end test service.
func NewPullServer() (Server, error) {
	pubsubClient, err := pubsub.NewClient(context.Background(), projectID)
	if err != nil {
		return nil, err
	}

	return &pullServer{
		pubsubClient: pubsubClient,
	}, nil
}

// Run the end-to-end test service. This method will block until the context is
// cancel, or an unrecoverable error is encountered.
func (s *pullServer) Run(ctx context.Context) error {
	sub := s.pubsubClient.Subscription(requestSubscriptionName)
	log.Printf("End-to-end test service listening on %s", sub)
	return sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) { s.onReceive(ctx, m) })
}

// Shutdown gracefully shuts down the service, flushing and closing resources as
// appropriate.
func (s *pullServer) Shutdown(ctx context.Context) error {
	return s.pubsubClient.Close()
}

// onReceive executes a scenario based on the incoming message from the test runner.
func (s *pullServer) onReceive(ctx context.Context, m *pubsub.Message) {
	defer m.Ack()
	if err := scenarios.HandleMessage(ctx, s.pubsubClient, m.Attributes); err != nil {
		log.Println(err)
	}
}
