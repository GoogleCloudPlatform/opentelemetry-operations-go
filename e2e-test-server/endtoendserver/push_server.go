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
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"cloud.google.com/go/pubsub"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/e2e-test-server/scenarios"
)

// pushServer is an end-to-end test service.
type pushServer struct {
	pubsubClient *pubsub.Client
	srv          http.Server
}

// New instantiates a new push end-to-end test service.
func NewPushServer() (Server, error) {
	pubsubClient, err := pubsub.NewClient(context.Background(), projectID)
	if err != nil {
		return nil, err
	}

	return &pushServer{
		srv:          http.Server{Addr: ":" + port},
		pubsubClient: pubsubClient,
	}, nil
}

// Run the end-to-end test service. This method will block until the context is
// cancel, or an unrecoverable error is encountered.
func (s *pushServer) Run(ctx context.Context) error {
	http.HandleFunc("/", s.handle)
	http.HandleFunc("/ready", handleReady)
	http.HandleFunc("/alive", handleAlive)
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Error starting or closing listener:
			log.Printf("HTTP server ListenAndServe: %v", err)
		}
	}()
	<-ctx.Done()
	return nil
}

// Shutdown gracefully shuts down the service, flushing and closing resources as
// appropriate.
func (s *pushServer) Shutdown(ctx context.Context) error {
	pubsubErr := s.pubsubClient.Close()
	if err := s.srv.Shutdown(ctx); err != nil {
		return err
	}
	return pubsubErr
}

// PubSubMessage is the payload of a Pub/Sub event.
type PubSubMessage struct {
	Message struct {
		Attributes map[string]string `json:"attributes,omitempty"`
	} `json:"message"`
}

// handle receives and processes a Pub/Sub push message.
func (s *pushServer) handle(w http.ResponseWriter, r *http.Request) {
	var m PubSubMessage
	if r.Method == "POST" {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("io.ReadAll: %v\n", err)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		// byte slice unmarshalling handles base64 decoding.
		if err := json.Unmarshal(body, &m); err != nil {
			log.Printf("json.Unmarshal: %v\n", err)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		if err := scenarios.HandleMessage(r.Context(), s.pubsubClient, m.Message.Attributes); err != nil {
			log.Println(err)
		}
	}
	// Ack the message
	fmt.Fprint(w, "OK")
}

func handleReady(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Server Ready")
}

func handleAlive(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Server Alive")
}
