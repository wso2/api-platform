# Policy Engine Builder Image
# This image CONTAINS the Policy Engine framework source code and Builder Go application
# Users ONLY mount their custom policy implementations - NOT the framework source

FROM golang:1.24-alpine AS builder-base

# Install build dependencies
RUN apk add --no-cache \
    git \
    make \
    upx \
    ca-certificates

# Set working directory
WORKDIR /workspace

# Copy Policy Engine framework source code (policy-engine/)
COPY policy-engine/ /workspace/policy-engine/

# Copy SDK components
COPY sdk/ /workspace/sdk/

# Copy Policy Builder application (includes templates)
COPY policy-builder/ /workspace/policy-builder/

# Pre-download Go dependencies for runtime
WORKDIR /workspace/policy-engine
RUN go mod download

# Pre-download Go dependencies for SDK
WORKDIR /workspace/sdk
RUN go mod download

# Pre-download Go dependencies for policy-builder
WORKDIR /workspace/policy-builder
RUN go mod download || true

# Build the Policy Builder binary
RUN go build -o /usr/local/bin/policy-engine-builder ./cmd/builder

# Set working directory back to workspace
WORKDIR /workspace

# Set environment variables for default paths
ENV POLICIES_DIR=/policies
ENV OUTPUT_DIR=/output
ENV RUNTIME_DIR=/workspace/policy-engine

# Set entrypoint to builder binary
ENTRYPOINT ["/usr/local/bin/policy-engine-builder"]
