# Policy Engine Builder Image
# This image CONTAINS the Policy Engine framework source code and Builder Go application
# Users ONLY mount their custom policy implementations - NOT the framework source

FROM golang:1.23-alpine AS builder-base

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

# Copy Builder Go application and templates
COPY build/ /workspace/build/
COPY templates/ /workspace/templates/

# Pre-download Go dependencies for framework
WORKDIR /workspace/src
RUN go mod download

# Pre-download Go dependencies for builder
WORKDIR /workspace/build
RUN go mod download || true

# Build the Builder Go binary
RUN go build -o /usr/local/bin/policy-engine-builder ./cmd/builder/main.go

# Set entrypoint to builder binary
WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/policy-engine-builder"]

# Default command shows help
CMD ["--help"]

# Expected mounts:
# - /policies (user's custom policy implementations)
# - /output (where compiled binary and Dockerfile will be generated)
