package endtoendserver

import (
	"log"
	"os"
)

const (
	instrumentingModuleName = "opentelemetry-ops-e2e-test-server"
	scenarioKey             = "scenario"
	testIDKey               = "test_id"
	statusCode              = "status_code"
)

var (
	subscriptionMode        = mustGetenv("SUBSCRIPTION_MODE")
	projectID               = mustGetenv("PROJECT_ID")
	requestSubscriptionName = mustGetenv("REQUEST_SUBSCRIPTION_NAME")
	responseTopicName       = mustGetenv("RESPONSE_TOPIC_NAME")
)

func mustGetenv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("environment variable %q must be set", key)
	}
	return v
}
