package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/cloudmock"
)

func main() {
	srv, err := cloudmock.NewMetricTestServerWithEndpoint("localhost:8080")
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
