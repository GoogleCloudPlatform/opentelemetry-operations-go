// Copyright The OpenTelemetry Authors
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
	"encoding/json"
	"fmt"
	"log"
	"time"

	"google.golang.org/api/option"

	"github.com/googleinterns/cloud-operations-api-mock/cloudmock"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	cloudtrace "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
)

func main() {
	// create new mock server
	mock := cloudmock.NewCloudMock()
	defer mock.Shutdown()
	clientOpt := []option.ClientOption{option.WithGRPCConn(mock.ClientConn())}
	tp, flush, err := cloudtrace.InstallNewPipeline(
		[]cloudtrace.Option{
			cloudtrace.WithProjectID("mock"),
			// configure exporter to connect to mock server
			cloudtrace.WithTraceClientOptions(clientOpt),
		},
		// For this example code we use sdktrace.AlwaysSample sampler to sample all traces.
		// In a production application, use sdktrace.ProbabilitySampler with a desired probability.
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
	)
	if err != nil {
		log.Fatal(err)
	}

	tracer := tp.Tracer("test-tracer")
	// This example creates one long span and multiple shorter child spans
	ctx, span := tracer.Start(context.Background(), "mock-server-example")
	numChildSpans := 3
	for i := 0; i < numChildSpans; i++ {
		_, iSpan := tracer.Start(ctx, fmt.Sprintf("Sample-%d", i))
		<-time.After(time.Second)
		iSpan.End()
	}

	span.End()
	flush()
	numSpansReceived := mock.GetNumSpans()
	fmt.Printf("Mock server received %d out of %d spans sent. Spans:\n", numSpansReceived, numChildSpans+1)
	for i := 0; i < numSpansReceived; i++ {
		// pretty print each span
		s, _ := json.MarshalIndent(mock.GetSpan(i), "", "  ")
		fmt.Println(string(s))
	}
}
