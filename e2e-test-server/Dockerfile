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

FROM golang:1.16 as builder

WORKDIR /workspace

COPY go.mod go.sum ./
# cache deps before copying source so that source changes don't invalidate our
# downloaded layer and we don't need to re-download as much.
RUN go mod download

# Copy the go source
COPY main.go ./
COPY endtoendserver/ endtoendserver/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o end-to-end-test-server main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static
WORKDIR /
COPY --from=builder /workspace/end-to-end-test-server .

ENTRYPOINT ["/end-to-end-test-server"]
