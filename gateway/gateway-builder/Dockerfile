# Gateway Builder Image
FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine AS builder-base
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

# Copy Policy Engine framework source code (policy-engine/)
COPY gateway/policy-engine/ /workspace/policy-engine/

# Copy Gateway Builder application (includes templates)
COPY gateway/gateway-builder/ /workspace/gateway-builder/

# Pre-download Go dependencies for runtime
WORKDIR /workspace/policy-engine/policy-engine
RUN go mod download

# Copy SDK (needed for go.mod replace directive)
COPY sdk/ /workspace/sdk/

# Pre-download Go dependencies for SDK
WORKDIR /workspace/sdk
RUN go mod download

# Pre-download Go dependencies for gateway-builder
WORKDIR /workspace/gateway-builder
RUN go mod download || true

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

FROM golang:1.24-alpine

# Install runtime dependencies
RUN apk add --no-cache \
    git \
    make \
    upx \
    ca-certificates

# Copy the compiled binary from builder stage
COPY --from=builder-base /usr/local/bin/gateway-builder /usr/local/bin/gateway-builder

# Copy framework source code needed at runtime
COPY --from=builder-base /workspace/policy-engine/policy-engine /workspace/policy-engine
COPY --from=builder-base /workspace/sdk /workspace/sdk
COPY --from=builder-base /workspace/gateway-builder /workspace/gateway-builder

# Set working directory
WORKDIR /workspace

# Set environment variables for default paths
ENV POLICIES_DIR=/policies
ENV OUTPUT_DIR=/output
ENV RUNTIME_DIR=/workspace/policy-engine

# Set entrypoint to builder binary
ENTRYPOINT ["/usr/local/bin/gateway-builder"]
