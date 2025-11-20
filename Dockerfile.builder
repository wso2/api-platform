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

# Copy Policy Engine framework source code (src/)
COPY src/ /workspace/src/

# Copy SDK components
COPY sdk/ /workspace/sdk/

# Copy Builder Go application and templates
COPY build/ /workspace/build/
COPY templates/ /workspace/templates/

# Pre-download Go dependencies for framework
WORKDIR /workspace/src
RUN go mod download

# Pre-download Go dependencies for SDK
WORKDIR /workspace/sdk
RUN go mod download

# Pre-download Go dependencies for builder
WORKDIR /workspace/build
RUN go mod download || true

# Build the Builder Go binary
RUN go build -o /usr/local/bin/policy-engine-builder ./cmd/builder

# Set working directory back to workspace
WORKDIR /workspace

# Set environment variables for default paths
ENV POLICIES_DIR=/policies
ENV OUTPUT_DIR=/output
ENV SRC_DIR=/workspace/src

# Set entrypoint to builder binary
ENTRYPOINT ["/usr/local/bin/policy-engine-builder"]

# Usage:
# 1. Manifest-based (recommended):
#    docker run --rm \
#      -v $(pwd)/policies.yaml:/policies.yaml \
#      -v $(pwd)/policies:/policies \
#      -v $(pwd)/output:/output \
#      policy-engine-builder:v1.0.0 --manifest /policies.yaml
#
# 2. Directory-based (legacy):
#    docker run --rm \
#      -v $(pwd)/policies:/policies \
#      -v $(pwd)/output:/output \
#      policy-engine-builder:v1.0.0
#
# Expected mounts:
# - /policies.yaml (optional: policy manifest file - recommended)
# - /policies (user's custom policy implementations)
# - /output (where compiled binary and Dockerfile will be generated)
