# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

# Event Gateway Runtime Dockerfile
# Uses event-gateway-builder to compile the event-gateway binary with embedded policies

# Stage 1: Builder Base — compile the event-gateway-builder tool
FROM --platform=$BUILDPLATFORM golang:1.26.2-bookworm AS builder-base
ARG BUILDARCH
ARG VERSION=unknown
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

# Download event-gateway-builder dependencies (cached by go.mod/go.sum changes)
COPY --from=gateway-builder go.mod go.sum /api-platform/event-gateway/gateway-builder/
WORKDIR /api-platform/event-gateway/gateway-builder
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy full source and build the event-gateway-builder binary
COPY --from=gateway-builder . /api-platform/event-gateway/gateway-builder
WORKDIR /api-platform/event-gateway/gateway-builder
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${BUILDARCH} \
    go build \
    -o /usr/local/bin/event-gateway-builder \
    ./cmd/builder

# Stage 2: Policy Compilation — run event-gateway-builder to produce the event-gateway binary
FROM builder-base AS policy-compiler

ARG VERSION
ARG GIT_COMMIT
ARG BUILD_DATE
ARG TARGETARCH
ENV VERSION=${VERSION}
ENV GIT_COMMIT=${GIT_COMMIT}
ENV BUILD_DATE=${BUILD_DATE}
ENV TARGETARCH=${TARGETARCH}

# Copy sdk-core for go.mod replace directive
COPY --from=sdk-core go.mod /api-platform/sdk/core/

# Copy analytics go.mod for policy-engine replace directive
COPY --from=analytics go.mod /api-platform/gateway/system-policies/analytics/

# Download common dependencies (cached by go.mod/go.sum changes)
COPY --from=common go.mod go.sum /api-platform/common/
WORKDIR /api-platform/common
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Download policy-engine dependencies (cached by go.mod/go.sum changes)
COPY --from=policy-engine go.mod go.sum /api-platform/gateway/gateway-runtime/policy-engine/
WORKDIR /api-platform/gateway/gateway-runtime/policy-engine
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Download event-gateway runtime dependencies (cached by go.mod/go.sum changes)
WORKDIR /api-platform/event-gateway/gateway-runtime
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy all source files
COPY . /api-platform/event-gateway/gateway-runtime/
COPY --from=sdk-core . /api-platform/sdk/core
COPY --from=common . /api-platform/common
COPY --from=analytics . /api-platform/gateway/system-policies/analytics
COPY --from=policy-engine . /api-platform/gateway/gateway-runtime/policy-engine
COPY --from=target build.yaml /api-platform/event-gateway/build.yaml

RUN mkdir -p /api-platform/output

WORKDIR /api-platform

# Run event-gateway-builder to generate plugin registry and compile event-gateway binary
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    /usr/local/bin/event-gateway-builder \
        -build-file /api-platform/event-gateway/build.yaml \
        -gateway-src /api-platform/event-gateway/gateway-runtime \
        -out-dir /api-platform/output

# Stage 3: Minimal runtime image
FROM alpine:3.22

ARG VERSION=unknown

RUN apk add --no-cache ca-certificates bash tini && \
    addgroup -S egw && adduser -S egw -G egw

WORKDIR /app

COPY --from=policy-compiler /api-platform/output/event-gateway/event-gateway /app/event-gateway
COPY --from=target LICENSE /app/LICENSE
COPY configs/config.toml /etc/event-gateway/config.toml
COPY configs/channels.yaml /etc/event-gateway/channels.yaml
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/event-gateway /app/docker-entrypoint.sh

USER egw

EXPOSE 8080 8081 9002 9003

# Metadata labels
LABEL org.opencontainers.image.title="API Platform Event Gateway Runtime"
LABEL org.opencontainers.image.description="API Platform Event Gateway Runtime with compiled-in policies"
LABEL org.opencontainers.image.vendor="WSO2"
LABEL org.opencontainers.image.version="${VERSION}"

ENTRYPOINT ["/sbin/tini", "--", "/app/docker-entrypoint.sh"]
