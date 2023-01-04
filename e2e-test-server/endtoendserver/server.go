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

	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
)

// Server is an end-to-end test service.
type Server interface {
	Run(context.Context) error
	Shutdown(ctx context.Context)
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
