// Copyright 2020, Google Inc.
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
	"log"
	"sync"

	mexporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric"

	"go.opentelemetry.io/otel/api/kv"
	"go.opentelemetry.io/otel/api/metric"
)

func main() {
	// Initialization. In order to pass the credentials to the exporter,
	// prepare credential file following the instruction described in this doc.
	// https://pkg.go.dev/golang.org/x/oauth2/google?tab=doc#FindDefaultCredentials
	opts := []mexporter.Option{}
	pusher, err := mexporter.InstallNewPipeline(opts)
	if err != nil {
		log.Fatalf("Failed to establish pipeline: %v", err)
	}
	defer pusher.Stop()

	block := make(chan bool)

	// Start meter
	ctx := context.Background()
	meter := pusher.Meter("cloudmonitoring/example")

	// Register counter value
	counter := metric.Must(meter).NewInt64Counter("counter-a")
	labels := []kv.KeyValue{kv.Key("key").String("value")}
	counter.Add(ctx, 100, labels...)

	// Register observer value
	observerMu := new(sync.RWMutex)
	observerV := new(float64)
	observerL := &[]kv.KeyValue{}
	callback := func(result metric.Float64ObserverResult) {
		(*observerMu).RLock()
		v := observerV
		l := observerL
		(*observerMu).RUnlock()
		result.Observe(*v, *l...)
	}

	lables := []kv.KeyValue{
		kv.String("foo", "Tokyo"),
		kv.String("bar", "Sushi"),
	}
	metric.Must(meter).RegisterFloat64Observer("observer-a", callback)
	(*observerMu).Lock()
	*observerV = 12.34
	*observerL = lables
	(*observerMu).Unlock()

	<-block
}
