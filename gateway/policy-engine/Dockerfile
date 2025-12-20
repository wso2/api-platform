# Runtime Dockerfile for Policy Engine

# Stage 1: Builder Base - Contains framework source and builder tooling
FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine3.23 AS builder-base
ARG BUILDARCH

# Build arguments for version information
ARG VERSION=0.0.1-SNAPSHOT
ARG GIT_COMMIT=unknown

# Install build dependencies
RUN apk add --no-cache \
    git \
    make \
    ca-certificates

# Set working directory
WORKDIR /workspace

# Copy SDK components
COPY --from=sdk . /api-platform/sdk

# Copy Gateway Builder application
COPY --from=gateway-builder . /api-platform/gateway/gateway-builder

# Pre-download Go dependencies for SDK
WORKDIR /api-platform/sdk
RUN go mod download

# Pre-download Go dependencies for gateway-builder
WORKDIR /api-platform/gateway/gateway-builder
RUN go mod download

# Build the Gateway Builder binary (CGO disabled for static binary)
RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${BUILDARCH} \
    go build \
    -a -installsuffix cgo \
    -o /usr/local/bin/gateway-builder \
    ./cmd/builder

# Stage 2: Policy Compilation - Run builder to compile policies
FROM builder-base AS policy-compiler

# Pass build arguments as environment variables
ARG VERSION
ARG GIT_COMMIT
ENV VERSION=${VERSION}
ENV GIT_COMMIT=${GIT_COMMIT}

# Copy Policy Engine framework source code
COPY --from=policy-engine . /api-platform/gateway/policy-engine
COPY --from=sdk . /api-platform/sdk

# Pre-download Go dependencies for runtime
WORKDIR /api-platform/gateway/policy-engine
RUN go mod download

# Copy policy implementations
COPY --from=policies . /workspace/policies/

# Create output directory
RUN mkdir -p /workspace/output

# Set working directory
WORKDIR /workspace

# Run the gateway builder with manifest (VERSION and GIT_COMMIT are available as env vars)
RUN /usr/local/bin/gateway-builder \
    -manifest-lock /workspace/policies/policy-manifest-lock.yaml \
    -policy-engine-src /api-platform/gateway/policy-engine \
    -out-dir /workspace/output

# Stage 3: Runtime - Minimal image with compiled binary
FROM alpine:3.23.2

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy compiled binary from policy-compiler stage
COPY --from=policy-compiler /workspace/output/policy-engine/policy-engine /app/policy-engine

# Copy config files
COPY configs/ /configs/

# Expose ports
EXPOSE 9001 9002

# Run the binary
ENTRYPOINT ["/app/policy-engine"]
CMD ["-config=/configs/config.yaml"]
