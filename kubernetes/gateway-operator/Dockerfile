# Build the manager binary
FROM golang:1.25.1 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY kubernetes/gateway-operator/go.mod go.mod
COPY kubernetes/gateway-operator/go.sum go.sum

# Copy the gateway-controller module (needed for the go.mod replace directive)
COPY gateway/gateway-controller /gateway/gateway-controller

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY kubernetes/gateway-operator/cmd/main.go cmd/main.go
COPY kubernetes/gateway-operator/api/ api/
COPY kubernetes/gateway-operator/internal/ internal/
COPY kubernetes/gateway-operator/config/config.yaml config/config.yaml
COPY kubernetes/gateway-operator/config/gateway_values.yaml config/gateway_values.yaml

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/config/config.yaml ./config/config.yaml
COPY --from=builder /workspace/config/gateway_values.yaml ./config/gateway_values.yaml
USER 65532:65532

ENTRYPOINT ["/manager"]
