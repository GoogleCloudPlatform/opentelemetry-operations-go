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
	"errors"
	"log"
	"net/http"

	"cloud.google.com/go/pubsub/v2"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/e2e-test-server/scenarios"
)

// pullServer is an end-to-end test service.
type pullServer struct {
	pubsubClient *pubsub.Client
	srv          *http.Server
}

func newHealthCheckServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/alive", handleAlive)
	mux.HandleFunc("/ready", handleReady)
	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}
}

// NewPullServer instantiates a new end-to-end test service.
func NewPullServer() (Server, error) {
	pubsubClient, err := pubsub.NewClient(context.Background(), projectID)
	if err != nil {
		return nil, err
	}

	var srv *http.Server
	if port != "" {
		srv = newHealthCheckServer(":" + port)
	}

	return &pullServer{
		pubsubClient: pubsubClient,
		srv:          srv,
	}, nil
}

// Run the end-to-end test service. This method will block until the context is
// cancel, or an unrecoverable error is encountered.
func (s *pullServer) Run(ctx context.Context) error {
	if s.srv != nil {
		go func() {
			if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("HTTP server ListenAndServe: %v", err)
			}
		}()
	}

	sub := s.pubsubClient.Subscriber(requestSubscriptionName)
	log.Printf("End-to-end test service listening on %s", sub)
	return sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) { s.onReceive(ctx, m) })
}

// Shutdown gracefully shuts down the service, flushing and closing resources as
// appropriate.
func (s *pullServer) Shutdown(ctx context.Context) error {
	var errs []error
	if s.srv != nil {
		if err := s.srv.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if s.pubsubClient != nil {
		if err := s.pubsubClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// onReceive executes a scenario based on the incoming message from the test runner.
func (s *pullServer) onReceive(ctx context.Context, m *pubsub.Message) {
	defer m.Ack()
	if err := scenarios.HandleMessage(ctx, s.pubsubClient, m.Attributes); err != nil {
		log.Println(err)
	}
}
