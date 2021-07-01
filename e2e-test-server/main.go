package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/e2e-test-server/endtoendserver"
)

func main() {
	runCtx, stopRunningFn := context.WithCancel(context.Background())

	// Gracefully close on SIGINT
	interupted := make(chan os.Signal, 1)
	signal.Notify(interupted, os.Interrupt)
	go func() {
		<-interupted
		log.Println("Received interrupt signal, shutting down.")
		stopRunningFn()
	}()

	server, err := endtoendserver.New()
	if err != nil {
		log.Fatalf("Could not initialize server: %v", err)
	}

	err = server.Run(runCtx)
	if err != nil {
		log.Printf("Shutting down due to unexpected error: %v", err)
	}

	shutdownCtx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	server.Shutdown(shutdownCtx)
}
