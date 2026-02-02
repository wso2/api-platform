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

# Runtime Dockerfile for Policy Engine

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

# Copy Common mod files first to cache dependencies
COPY --from=common go.mod go.sum /api-platform/common/
WORKDIR /api-platform/common
RUN go mod download

# Copy SDK mod files first to cache dependencies
COPY --from=sdk go.mod go.sum /api-platform/sdk/
WORKDIR /api-platform/sdk
RUN go mod download

# Copy Gateway Builder mod files first to cache dependencies
COPY --from=gateway-builder go.mod go.sum /api-platform/gateway/gateway-builder/
WORKDIR /api-platform/gateway/gateway-builder
RUN go mod download

# Copy full SDK and Gateway Builder and Common source code
COPY --from=common . /api-platform/common
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

# Copy Policy Engine mod files and SDK and Common mod files for dependency resolution
COPY --from=common go.mod go.sum /api-platform/common/
COPY --from=sdk go.mod go.sum /api-platform/sdk/
COPY go.mod go.sum /api-platform/gateway/policy-engine/
WORKDIR /api-platform/gateway/policy-engine
RUN go mod download


# Copy full Policy Engine and SDK and Common source code
COPY . /api-platform/gateway/policy-engine
COPY --from=common . /api-platform/common
COPY --from=sdk . /api-platform/sdk

# Copy policy implementations
COPY --from=policies . /workspace/policies/

# Copy system policy implementations
COPY --from=system-policies . /workspace/policies/

# Create output directory
RUN mkdir -p /workspace/output

# Set working directory
WORKDIR /workspace

# Run the gateway builder with manifest (VERSION and GIT_COMMIT are available as env vars)
# Set COVERAGE=true when ENABLE_COVERAGE is true for coverage-instrumented builds
RUN if [ "$ENABLE_COVERAGE" = "true" ]; then \
        COVERAGE=true /usr/local/bin/gateway-builder \
            -manifest /workspace/policies/policy-manifest.yaml \
            -system-manifest-lock /workspace/policies/system-policy-manifest-lock.yaml \
            -policy-engine-src /api-platform/gateway/policy-engine \
            -out-dir /workspace/output; \
    else \
        /usr/local/bin/gateway-builder \
            -manifest /workspace/policies/policy-manifest.yaml \
            -system-manifest-lock /workspace/policies/system-policy-manifest-lock.yaml \
            -policy-engine-src /api-platform/gateway/policy-engine \
            -out-dir /workspace/output; \
    fi

# Stage 3: Runtime - Minimal image with compiled binary
FROM alpine:3.23.3

# Re-declare ARG after FROM to use it in this stage
ARG ENABLE_COVERAGE=false

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy compiled binary from policy-compiler stage
COPY --from=policy-compiler /workspace/output/policy-engine/policy-engine /app/policy-engine

# Create coverage output directory if coverage is enabled
RUN if [ "$ENABLE_COVERAGE" = "true" ]; then mkdir -p /coverage; fi

# Set GOCOVERDIR for coverage data collection (only effective when binary is built with -cover)
ENV GOCOVERDIR=/coverage

# Expose ports
EXPOSE 9001 9002

# Run the binary (config must be provided via command line)
ENTRYPOINT ["/app/policy-engine"]
