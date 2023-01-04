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
type Server interface {
	Run(context.Context) error
	Shutdown(ctx context.Context) error
}

// New instantiates a new end-to-end test service.
func New() (Server, error) {
	switch subscriptionMode {
	case "pull":
		return NewPullServer()
	case "push":
		return NewPushServer()
	default:
		return nil, fmt.Errorf("server does not support subscription mode %v", subscriptionMode)
	}
}

func newTracerProvider(res *resource.Resource) (*sdktrace.TracerProvider, error) {
	exporter, err := texporter.New(texporter.WithProjectID(projectID))
	if err != nil {
		return nil, err
	}

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(traceBatchTimeout)),
		sdktrace.WithResource(res))

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

func handleMessage(ctx context.Context, client *pubsub.Client, m *pubsub.Message) {
	testID := m.Attributes[testIDKey]
	scenario := m.Attributes[scenarioKey]
	if scenario == "" {
		log.Printf("could not find required attribute %q in message %+v", scenarioKey, m)
		err := respond(ctx, client, testID, &response{
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
		handler = &unimplementedHandler{}
	}

	tracerProvider, err := handler.tracerProvider()
	if err != nil {
		log.Printf("could not initialize a tracer-provider: %v", err)
		return
	}

	req := request{
		scenario: scenario,
		testID:   testID,
	}

	res := handler.handle(ctx, req, tracerProvider)

	if err := shutdownTraceProvider(ctx, tracerProvider); err != nil {
		log.Printf("could not shutdown tracer-provider: %v", err)
		if respondErr := respond(ctx, client, testID, &response{
			statusCode: code.Code_INTERNAL,
			data:       []byte(fmt.Sprintf("could not shutdown tracer-provider: %v", err)),
		}); respondErr != nil {
			log.Printf("could not publish response: %v", respondErr)
		}
		return
	}

	if err := respond(ctx, client, testID, res); err != nil {
		log.Printf("could not publish response: %v", err)
	}
}

// respond to the test runner that we finished executing the scenario by sending
// a message to the response pubsub topic.
func respond(ctx context.Context, client *pubsub.Client, testID string, res *response) error {
	m := &pubsub.Message{
		Data: res.data,
		Attributes: map[string]string{
			testIDKey:     testID,
			statusCodeKey: strconv.Itoa(int(res.statusCode)),
			traceIDKey:    res.traceID.String(),
		},
	}
	publishResult := client.Topic(responseTopicName).Publish(ctx, m)
	_, err := publishResult.Get(ctx)
	return err
}
