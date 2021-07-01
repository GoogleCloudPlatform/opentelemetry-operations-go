FROM golang:1.16 as builder

WORKDIR /workspace

COPY go.mod go.sum ./
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
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
