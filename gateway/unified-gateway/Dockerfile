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

# Stage 1: Builder Base
FROM --platform=$BUILDPLATFORM golang:1.25.6-alpine3.23 AS builder-base
ARG BUILDARCH
ARG VERSION=0.0.1-SNAPSHOT
ARG GIT_COMMIT=unknown
ARG BUILD_DATE

# Coverage build argument - set to "true" for coverage-instrumented builds
ARG ENABLE_COVERAGE=false

RUN apk add --no-cache \
    git \
    make \
    ca-certificates

WORKDIR /workspace

COPY --from=sdk go.mod go.sum /api-platform/sdk/
WORKDIR /api-platform/sdk
RUN go mod download

COPY --from=gateway-builder go.mod go.sum /api-platform/gateway/gateway-builder/
WORKDIR /api-platform/gateway/gateway-builder
RUN go mod download

COPY --from=sdk . /api-platform/sdk
COPY --from=gateway-builder . /api-platform/gateway/gateway-builder

WORKDIR /api-platform/gateway/gateway-builder
RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${BUILDARCH} \
    go build \
    -a -installsuffix cgo \
    -o /usr/local/bin/gateway-builder \
    ./cmd/builder

# Stage 2: Policy Compilation
FROM builder-base AS policy-compiler

ARG VERSION
ARG GIT_COMMIT
ARG BUILD_DATE
ARG ENABLE_COVERAGE
ENV VERSION=${VERSION}
ENV GIT_COMMIT=${GIT_COMMIT}
ENV BUILD_DATE=${BUILD_DATE}

COPY --from=sdk go.mod go.sum /api-platform/sdk/
RUN go mod download

WORKDIR /api-platform/gateway/policy-engine
COPY --from=policy-engine go.mod go.sum ./
RUN go mod download

COPY --from=policy-engine . /api-platform/gateway/policy-engine
COPY --from=sdk . /api-platform/sdk
COPY --from=system-policies . /workspace/system-policies/
COPY --from=dev-policies . /workspace/policies/dev-policies/
COPY --from=target build.yaml /workspace/policies/policy-manifest.yaml

RUN mkdir -p /workspace/output

WORKDIR /workspace

# Run gateway builder with manifest, set COVERAGE=true for coverage-instrumented builds
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

# Stage 3: Runtime
FROM envoyproxy/envoy:v1.35.3

ARG ENABLE_COVERAGE=false

USER root
RUN apt-get update && apt-get install -y --no-install-recommends \
    tini \
    gettext-base \
    ca-certificates \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /app /var/run /coverage

COPY --from=policy-compiler /workspace/output/policy-engine/policy-engine /app/policy-engine
COPY --from=router config/envoy-bootstrap.yaml /etc/envoy/envoy.yaml
COPY --from=router config/config-override.yaml /etc/envoy/config-override.yaml
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

RUN chmod +x /app/policy-engine /usr/local/bin/docker-entrypoint.sh

RUN groupadd -r -g 10001 wso2 && \
    useradd -r -u 10001 -g wso2 -s /sbin/nologin -c "WSO2 Application User" wso2 && \
    chown -R wso2:wso2 /app /coverage && \
    chmod -R 755 /app /coverage

# Set GOCOVERDIR for coverage data collection (only effective when binary is built with -cover)
ENV GOCOVERDIR=/coverage

ENV XDS_SERVER_HOST=gateway-controller \
    XDS_SERVER_PORT=18000 \
    LOG_LEVEL=info \
    POLICY_ENGINE_CONFIG=/etc/policy-engine/config.toml

USER wso2

EXPOSE 8080 8443 9901 9002 9003

ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/docker-entrypoint.sh"]
