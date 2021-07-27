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
	"context"
	"fmt"
	"log"
	"strconv"

	"cloud.google.com/go/pubsub"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/genproto/googleapis/rpc/code"

	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
)

// Server is an end-to-end test service.
type Server struct {
	pubsubClient *pubsub.Client
	// traceProvider *sdktrace.TracerProvider
}

// New instantiates a new end-to-end test service.
func New() (*Server, error) {
	if subscriptionMode != "pull" {
		return nil, fmt.Errorf("server does not support subscription mode %v", subscriptionMode)
	}

	pubsubClient, err := pubsub.NewClient(context.Background(), projectID)
	if err != nil {
		return nil, err
	}

	return &Server{
		pubsubClient: pubsubClient,
		// traceProvider: traceProvider,
	}, nil
}

// Run the end-to-end test service. This method will block until the context is
// cancel, or an unrecoverable error is encountered.
func (s *Server) Run(ctx context.Context) error {
	sub := s.pubsubClient.Subscription(requestSubscriptionName)
	log.Printf("End-to-end test service listening on %s", sub)
	return sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) { s.onReceive(ctx, m) })
}

// Shutdown gracefully shuts down the service, flushing and closing resources as
// appropriate.
func (s *Server) Shutdown(ctx context.Context) {
	if err := s.pubsubClient.Close(); err != nil {
		log.Printf("pubsubClient.Close(): %v", err)
	}
}

// onReceive executes a scenario based on the incoming message from the test runner.
func (s *Server) onReceive(ctx context.Context, m *pubsub.Message) {
	defer m.Ack()

	tracerProvider, err := newTraceProvider()
	if err != nil {
		log.Printf("could not initialize a tracer-provider: %v", err)
		return
	}

	testID := m.Attributes[testIDKey]
	scenario := m.Attributes[scenarioKey]
	if scenario == "" {
		log.Printf("could not find required attribute %q in message %+v", scenarioKey, m)
		err := s.respond(ctx, testID, &response{
			statusCode: code.Code_INVALID_ARGUMENT,
			data:       []byte(fmt.Sprintf("required %q is missing", scenarioKey)),
		})
		if err != nil {
			log.Printf("could not publish response: %v", err)
		}
		return
	}

	handler := scenarioHandlers[scenario]
	if handler == nil {
		handler = (*Server).unimplementedHandler
	}

	req := request{
		scenario: scenario,
		testID:   testID,
	}

	res := handler(s, ctx, req, tracerProvider)

	if err := shutdownTraceProvider(ctx, tracerProvider); err != nil {
		log.Printf("could not shutdown tracer-provider: %v", err)
		s.respond(ctx, testID, &response{
			statusCode: code.Code_INTERNAL,
			data:       []byte(fmt.Sprintf("could not shutdown tracer-provider: %v", err)),
		})
		return
	}

	if err := s.respond(ctx, testID, res); err != nil {
		log.Printf("could not publish response: %v", err)
	}
}

// respond to the test runner that we finished executing the scenario by sending
// a message to the response pubsub topic.
func (s *Server) respond(ctx context.Context, testID string, res *response) error {
	m := &pubsub.Message{
		Data: res.data,
		Attributes: map[string]string{
			testIDKey:     testID,
			statusCodeKey: strconv.Itoa(int(res.statusCode)),
			traceIDKey:    res.traceID.String(),
		},
	}
	publishResult := s.pubsubClient.Topic(responseTopicName).Publish(ctx, m)
	_, err := publishResult.Get(ctx)
	return err
}

func newTraceProvider() (*sdktrace.TracerProvider, error) {
	exporter, err := texporter.New(texporter.WithProjectID(projectID))
	if err != nil {
		return nil, err
	}

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(traceBatchTimeout)),
		// Set resource to empty so we don't add labels that the
		// e2e test runner doesn't expect. See b/192584837.
		sdktrace.WithResource(resource.Empty()))

	return traceProvider, nil
}

func shutdownTraceProvider(ctx context.Context, tracerProvider *sdktrace.TracerProvider) error {
	if err := tracerProvider.ForceFlush(ctx); err != nil {
		return fmt.Errorf("traceProvider.ForceFlush(): %v", err)
	}
	if err := tracerProvider.Shutdown(ctx); err != nil {
		return fmt.Errorf("traceProvider.Shutdown(): %v", err)
	}
	return nil
}
