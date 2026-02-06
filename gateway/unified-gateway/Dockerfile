# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

# Unified Gateway Dockerfile
# Combines Envoy Router and Policy Engine into a single container

# Stage 1: Builder Base - Contains framework source and builder tooling
FROM --platform=$BUILDPLATFORM golang:1.25.6-alpine3.23 AS builder-base
ARG BUILDARCH

# Build arguments for version information
ARG VERSION=0.0.1-SNAPSHOT
ARG GIT_COMMIT=unknown
ARG BUILD_DATE

# Coverage build argument - set to "true" for coverage-instrumented builds
ARG ENABLE_COVERAGE=false

# Install build dependencies
RUN apk add --no-cache \
    git \
    make \
    ca-certificates

# Set working directory
WORKDIR /workspace

# Copy SDK mod files first to cache dependencies
COPY --from=sdk go.mod go.sum /api-platform/sdk/
WORKDIR /api-platform/sdk
RUN go mod download

# Copy Gateway Builder mod files first to cache dependencies
COPY --from=gateway-builder go.mod go.sum /api-platform/gateway/gateway-builder/
WORKDIR /api-platform/gateway/gateway-builder
RUN go mod download

# Copy full SDK and Gateway Builder source code
COPY --from=sdk . /api-platform/sdk
COPY --from=gateway-builder . /api-platform/gateway/gateway-builder

# Build the Gateway Builder binary (CGO disabled for static binary)
WORKDIR /api-platform/gateway/gateway-builder
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
ARG BUILD_DATE
ARG ENABLE_COVERAGE
ENV VERSION=${VERSION}
ENV GIT_COMMIT=${GIT_COMMIT}
ENV BUILD_DATE=${BUILD_DATE}

# Copy Policy Engine mod files and SDK mod files for dependency resolution
COPY --from=sdk go.mod go.sum /api-platform/sdk/
RUN go mod download

WORKDIR /api-platform/gateway/policy-engine
COPY --from=policy-engine go.mod go.sum ./
RUN go mod download

# Copy full Policy Engine and SDK source code
COPY --from=policy-engine . /api-platform/gateway/policy-engine
COPY --from=sdk . /api-platform/sdk

# Copy system policy implementations
COPY --from=system-policies . /workspace/system-policies/

# Copy dev policy implementations (local file-path policies added via build.yaml)
COPY --from=dev-policies . /workspace/policies/dev-policies/

# Copy policy manifest from gateway build.yaml
COPY --from=target build.yaml /workspace/policies/policy-manifest.yaml

# Create output directory
RUN mkdir -p /workspace/output

# Set working directory
WORKDIR /workspace

# Run the gateway builder with manifest (VERSION and GIT_COMMIT are available as env vars)
# Set COVERAGE=true when ENABLE_COVERAGE is true for coverage-instrumented builds
RUN if [ "$ENABLE_COVERAGE" = "true" ]; then \
        COVERAGE=true /usr/local/bin/gateway-builder \
            -manifest /workspace/policies/policy-manifest.yaml \
            -system-manifest-lock /workspace/system-policies/system-policy-manifest-lock.yaml \
            -policy-engine-src /api-platform/gateway/policy-engine \
            -out-dir /workspace/output; \
    else \
        /usr/local/bin/gateway-builder \
            -manifest /workspace/policies/policy-manifest.yaml \
            -system-manifest-lock /workspace/system-policies/system-policy-manifest-lock.yaml \
            -policy-engine-src /api-platform/gateway/policy-engine \
            -out-dir /workspace/output; \
    fi

# Stage 3: Runtime - Envoy + Policy Engine unified container
FROM envoyproxy/envoy:v1.35.3

# Re-declare ARG after FROM to use it in this stage
ARG ENABLE_COVERAGE=false

# Install runtime dependencies
USER root
RUN apt-get update && apt-get install -y --no-install-recommends \
    tini \
    gettext-base \
    ca-certificates \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Create directories
RUN mkdir -p /app /var/run /coverage

# Copy compiled Policy Engine binary from policy-compiler stage
COPY --from=policy-compiler /workspace/output/policy-engine/policy-engine /app/policy-engine
RUN chmod +x /app/policy-engine

# Copy Envoy configuration files from router context
COPY --from=router config/envoy-bootstrap.yaml /etc/envoy/envoy.yaml
COPY --from=router config/config-override.yaml /etc/envoy/config-override.yaml

# Copy entrypoint script
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Set GOCOVERDIR for coverage data collection (only effective when binary is built with -cover)
ENV GOCOVERDIR=/coverage

# Environment variables with defaults
ENV XDS_SERVER_HOST=gateway-controller \
    XDS_SERVER_PORT=18000 \
    LOG_LEVEL=info \
    POLICY_ENGINE_SOCKET=/var/run/policy-engine.sock \
    POLICY_ENGINE_CONFIG=/etc/policy-engine/config.toml

# Expose ports:
# 8080  - HTTP traffic (Envoy)
# 8443  - HTTPS traffic (Envoy)
# 9901  - Envoy admin
# 9002  - Policy Engine admin
# 9003  - Policy Engine metrics
EXPOSE 8080 8443 9901 9002 9003

# Use tini as init process for proper signal handling and zombie reaping
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/docker-entrypoint.sh"]
