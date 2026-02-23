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

# Gateway Builder Image
FROM --platform=$BUILDPLATFORM golang:1.25.7-bookworm AS builder
ARG TARGETARCH

# Build arguments for version information
ARG VERSION=0.0.1-SNAPSHOT
ARG GIT_COMMIT=unknown

# Install build dependencies
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends \
    git \
    make \
    ca-certificates

# Set working directory
WORKDIR /workspace

# Copy common (needed for go.mod replace directive)
COPY --from=common . /workspace/common

# Pre-download Go dependencies for common
WORKDIR /workspace/common
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy SDK (needed for go.mod replace directive)
COPY --from=sdk . /workspace/sdk

# Pre-download Go dependencies for SDK
WORKDIR /workspace/sdk
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy Gateway Builder application (includes templates)
COPY . /workspace/gateway/gateway-builder

# Pre-download Go dependencies for runtime
WORKDIR /workspace/gateway/gateway-builder
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Build the Gateway Builder binary with version information
# Use explicit cross-compilation to avoid QEMU emulation issues
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${TARGETARCH} \
    go build \
    -ldflags "-s -w -X main.Version=${VERSION} \
    -X main.GitCommit=${GIT_COMMIT} \
    -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -trimpath \
    -o /usr/local/bin/gateway-builder \
    ./cmd/builder

FROM golang:1.25.7-bookworm

# Install runtime dependencies
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends \
    git \
    make \
    ca-certificates

# Copy the compiled binary from builder stage
COPY --from=builder /usr/local/bin/gateway-builder /usr/local/bin/gateway-builder

# Copy Policy Engine framework source code
COPY --from=policy-engine . /api-platform/gateway/gateway-runtime/policy-engine
COPY --from=system-policies . /api-platform/gateway/system-policies
COPY --from=common . /api-platform/common
COPY --from=sdk . /api-platform/sdk

# Set working directory
WORKDIR /workspace

# Set entrypoint to builder binary with system build lock
ENTRYPOINT [ \
    "/usr/local/bin/gateway-builder", \
    "-system-build-lock", \
    "/api-platform/gateway/system-policies/system-build-lock.yaml" \
]
