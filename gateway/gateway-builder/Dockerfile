# Gateway Builder Image
FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine3.23 AS builder
ARG TARGETARCH

# Build arguments for version information
ARG VERSION=0.0.1-SNAPSHOT
ARG GIT_COMMIT=unknown

# Install build dependencies
RUN apk add --no-cache \
    git \
    make \
    upx \
    ca-certificates

# Set working directory
WORKDIR /workspace

# Copy SDK (needed for go.mod replace directive)
COPY --from=sdk . /workspace/sdk

# Pre-download Go dependencies for SDK
WORKDIR /workspace/sdk
RUN go mod download

# Copy Gateway Builder application (includes templates)
COPY . /workspace/gateway/gateway-builder

# Pre-download Go dependencies for runtime
WORKDIR /workspace/gateway/gateway-builder
RUN go mod download

# Build the Gateway Builder binary with version information
# Use explicit cross-compilation to avoid QEMU emulation issues
RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${TARGETARCH} \
    go build \
    -ldflags "-s -w -X main.Version=${VERSION} \
              -X main.GitCommit=${GIT_COMMIT} \
              -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -trimpath \
    -o /usr/local/bin/gateway-builder \
    ./cmd/builder

FROM golang:1.25.5-alpine3.23

# Install runtime dependencies
RUN apk add --no-cache \
    git \
    make \
    upx \
    ca-certificates

# Copy the compiled binary from builder stage
COPY --from=builder /usr/local/bin/gateway-builder /usr/local/bin/gateway-builder

# Copy Policy Engine framework source code
COPY --from=policy-engine . /workspace/gateway/policy-engine
COPY --from=sdk . /workspace/sdk
# Set working directory
WORKDIR /workspace

# Set entrypoint to builder binary
ENTRYPOINT ["/usr/local/bin/gateway-builder"]
