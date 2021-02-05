// Copyright 2021 Google LLC. All Rights Reserved.
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

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	mexporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric"

	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/resource"
)

func main() {
	// Initialization. In order to pass the credentials to the exporter,
	// prepare credential file following the instruction described in this doc.
	// https://pkg.go.dev/golang.org/x/oauth2/google?tab=doc#FindDefaultCredentials
	opts := []mexporter.Option{
		mexporter.WithMetricDescriptorTypeFormatter(func(desc *metric.Descriptor) string {
			return fmt.Sprintf("kubernetes.io/%s", desc.Name())
		}),
		mexporter.WithInterval(10 * time.Second),
	}

	// These are the semantic conventions for containers that map to the
	// k8s_container resource.  Set them manually so that we can "pretend" we
	// are running in a GKE container from a dev environment
	resOpt := basic.WithResource(resource.NewWithAttributes(
		label.String("cloud.zone", "us-central1-c"),
		label.String("cloud.provider", "gcp"),
		label.String("k8s.cluster.name", "fake-cluster"),
		label.String("k8s.namespace.name", "default"),
		label.String("k8s.pod.name", "fake"),
		label.String("container.name", "fake"),
	))
	pusher, err := mexporter.InstallNewPipeline(opts, resOpt)
	if err != nil {
		log.Fatalf("Failed to establish pipeline: %v", err)
	}
	ctx := context.Background()
	defer pusher.Stop(ctx)

	// Recorder for ephemeral storage metric
	vr, err := pusher.MeterProvider().Meter("cloudmonitoring/example").
		NewInt64ValueRecorder("container/ephemeral_storage/used_bytes")
	if err != nil {
		log.Fatalf("failed to establish pipeline: %v", err)
	}
	valuerecorder := vr.Bind()
	defer valuerecorder.Unbind()

	// Add measurement once every 10 seconds.
	timer := time.NewTicker(10 * time.Second)
	for range timer.C {
		rand.Seed(time.Now().UnixNano())

		r := rand.Int63n(100)
		valuerecorder.Record(ctx, r)
		log.Printf("Most recent data: vr %v", r)
	}
}
