// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock"
)

var endpoint = flag.String("endpoint", "localhost:8080", "endpoint to host server")

func main() {
	srv, err := cloudmock.NewMetricTestServerWithEndpoint(*endpoint)
	if err != nil {
		log.Fatal(err)
	}
	go srv.Serve()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		reqs := srv.CreateTimeSeriesRequests()
		for _, req := range reqs {
			reqBytes, err := json.Marshal(req)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(string(reqBytes))
		}
	}
}
