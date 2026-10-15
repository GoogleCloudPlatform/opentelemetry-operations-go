#!/bin/bash
#
# Copyright 2025 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# A script to run the Google Cloud OTel Collector Docker image locally,
# mounting a local YAML configuration file.
#
# Usage: ./run_collector.sh <path_to_your_config.yaml>
#

# --- Configuration ---
# The full name of the Docker image to run.
DOCKER_IMAGE="us-docker.pkg.dev/cloud-ops-agents-artifacts/google-cloud-opentelemetry-collector/otelcol-google:0.129.0"
# The path inside the container where the config file will be mounted.
CONFIG_FILE_CONTAINER="/tmp/otelcol-custom/config.yaml"

# --- Script ---
# Exit immediately if a command fails.
set -e

# Store the path to the host's config file from the first script argument.
CONFIG_FILE_HOST="$1"

# 1. Validate that a config file was provided.
if [ -z "${CONFIG_FILE_HOST}" ]; then
  echo "❌ Error: No config file supplied."
  echo "Usage: $0 <path_to_your_config.yaml>"
  exit 1
fi

# 2. Validate that the provided file exists.
if [ ! -f "${CONFIG_FILE_HOST}" ]; then
  echo "❌ Error: File not found at '${CONFIG_FILE_HOST}'"
  exit 1
fi

# 3. Validate if the required environment variables were set.
if [[ -z "${GOOGLE_CLOUD_PROJECT}" ]]; then
  echo "❌ Error: GOOGLE_CLOUD_PROJECT environment variable not set"
  exit 1
fi

if [[ -z "${GOOGLE_APPLICATION_CREDENTIALS}" ]]; then
  echo "❌ Error: GOOGLE_APPLICATION_CREDENTIALS environment variable not set"
  exit 1
fi

echo "🚀 Starting OpenTelemetry Collector..."
echo "   Environment variables verified"
echo "   Image: ${DOCKER_IMAGE}"
echo "   Config File (Host): $(readlink -f "${CONFIG_FILE_HOST}")"
echo "   Application Creds (Host): $(readlink -f "${GOOGLE_APPLICATION_CREDENTIALS}")"
echo "   Google Project ID (Host): ${GOOGLE_CLOUD_PROJECT}"
echo "   Press Ctrl+C to stop the collector."

# 3. Run the Docker container.
#    --rm      : Automatically remove the container when it exits.
#    -it       : Run in interactive mode to see logs and allow Ctrl+C to stop it.
#    -v        : Mount the local config file into the container.
#    -e        : Set the required environment variables in the container.
#    --config  : Pass the path of the mounted config file to the collector's command.
docker run --rm -it \
  --user "$(id -u)":"$(id -g)" \
  -e "GOOGLE_CLOUD_PROJECT=${GOOGLE_CLOUD_PROJECT}" \
  -e "GOOGLE_APPLICATION_CREDENTIALS=${GOOGLE_APPLICATION_CREDENTIALS}" \
  -v "$(readlink -f "${CONFIG_FILE_HOST}")":"${CONFIG_FILE_CONTAINER}":ro \
  -v "${GOOGLE_APPLICATION_CREDENTIALS}:${GOOGLE_APPLICATION_CREDENTIALS}:ro" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -p 4318:4318 \
  -p 4317:4317 \
  "${DOCKER_IMAGE}" \
  --config "${CONFIG_FILE_CONTAINER}"
