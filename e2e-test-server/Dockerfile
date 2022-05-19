# Copyright 2021 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:1.17 as builder

WORKDIR /workspace/e2e-test-server/

# In-repo dependencies
COPY exporter/trace/ ../exporter/trace/
COPY internal/resourcemapping/ ../internal/resourcemapping
COPY detectors/gcp/ ../detectors/gcp/

# cache deps before copying source so that source changes don't invalidate our
# downloaded layer and we don't need to re-download as much.
COPY e2e-test-server/go.mod e2e-test-server/go.sum ./
RUN go mod download

# Copy the go source
COPY e2e-test-server/app.go ./
COPY e2e-test-server/endtoendserver/ endtoendserver/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o app app.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static
WORKDIR /
COPY --from=builder /workspace/e2e-test-server/app ./

ENTRYPOINT ["/app"]
