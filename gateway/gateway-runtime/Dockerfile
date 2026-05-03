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

# Gateway Runtime Dockerfile
# Combines Envoy Router and Policy Engine into a single container

# Stage 1: Builder Base
FROM --platform=$BUILDPLATFORM golang:1.26.2-bookworm AS builder-base
ARG BUILDARCH
ARG VERSION=unknown
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

# Coverage build argument - set to "true" for coverage-instrumented builds
ARG ENABLE_COVERAGE=false

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends \
    git \
    make \
    ca-certificates \
    python3 \
    python3-pip \
    python3-venv

WORKDIR /workspace

# Copy SDK core for go.mod replace directive
COPY --from=sdk-core go.mod /api-platform/sdk/core/

# Download gateway-builder dependencies (cached by go.mod/go.sum changes)
COPY --from=gateway-builder go.mod go.sum /api-platform/gateway/gateway-builder/
WORKDIR /api-platform/gateway/gateway-builder
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy full source for build
COPY --from=sdk . /api-platform/sdk
COPY --from=sdk-core . /api-platform/sdk/core
COPY --from=gateway-builder . /api-platform/gateway/gateway-builder

# Build gateway-builder binary (cache mounts persist across rebuilds)
WORKDIR /api-platform/gateway/gateway-builder
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${BUILDARCH} \
    go build \
    -o /usr/local/bin/gateway-builder \
    ./cmd/builder

# Stage 2: Policy Compilation
FROM builder-base AS policy-compiler

ARG VERSION
ARG GIT_COMMIT
ARG BUILD_DATE
ARG ENABLE_COVERAGE
ARG ENABLE_DEBUG=false
ARG TARGETARCH
ENV VERSION=${VERSION}
ENV GIT_COMMIT=${GIT_COMMIT}
ENV BUILD_DATE=${BUILD_DATE}
ENV TARGETARCH=${TARGETARCH}
ENV ENABLE_DEBUG=${ENABLE_DEBUG}

# Copy SDK core for go.mod replace directive
COPY --from=sdk-core go.mod /api-platform/sdk/core/

# Download common dependencies (cached by go.mod/go.sum changes)
COPY --from=common go.mod go.sum /api-platform/common/
WORKDIR /api-platform/common
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Download policy-engine dependencies (cached by go.mod/go.sum changes)
WORKDIR /api-platform/gateway/gateway-runtime/policy-engine
COPY policy-engine/go.mod policy-engine/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy all source and policy files
COPY policy-engine/ /api-platform/gateway/gateway-runtime/policy-engine/
COPY python-executor/ /api-platform/gateway/gateway-runtime/python-executor/
COPY --from=sdk-python . /api-platform/sdk-python
COPY --from=sdk-core . /api-platform/sdk/core
COPY --from=common . /api-platform/common
COPY --from=system-policies . /api-platform/gateway/system-policies/
COPY --from=dev-policies . /api-platform/gateway/dev-policies/
COPY --from=target build.yaml /api-platform/gateway/build.yaml

RUN mkdir -p /api-platform/output

WORKDIR /api-platform

# Run gateway builder with build file, conditionally enable coverage or debug
# Cache mounts let the builder's internal go build reuse compiled packages
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    [ "$ENABLE_COVERAGE" = "true" ] && export COVERAGE=true || true; \
    [ "$ENABLE_DEBUG" = "true" ] && export DEBUG=true || true; \
    /usr/local/bin/gateway-builder \
        -build-file /api-platform/gateway/build.yaml \
        -system-build-lock /api-platform/gateway/system-policies/system-build-lock.yaml \
        -policy-engine-src /api-platform/gateway/gateway-runtime/policy-engine \
        -out-dir /api-platform/output

# Stage: policy-export — extracts controller policy definitions for host consumption.
# Built with `--target policy-export --output type=local,dest=...`. Reuses the
# policy-compiler stage cache, so no second gateway-builder run is required.
FROM scratch AS policy-export
COPY --from=policy-compiler /api-platform/output/gateway-controller/policies /

# Stage 3a: Python dependency installer
# Uses the same base image as the production stage so that C extensions
# (e.g. grpcio) are compiled against the identical Python version and glibc.
# pip, venv, and git are installed here only — they are NOT copied to production.
FROM envoyproxy/envoy:v1.37.1 AS python-deps

USER root
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends \
    git \
    python3 \
    python3-pip \
    python3-venv

COPY --from=policy-compiler /api-platform/output/python-executor/requirements.txt /tmp/requirements.txt

# Upgrade pip and setuptools so PEP 621 (pyproject.toml) and recent git dependencies are supported
RUN pip3 install --no-cache-dir --upgrade pip setuptools wheel

RUN pip3 install --no-cache-dir \
    --target /app/python-libs \
    -r /tmp/requirements.txt

# Stage 3b: dlv installer (builds dlv for the debug stage)
FROM golang:1.26.2-bookworm AS debug-dlv-builder
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/go-delve/delve/cmd/dlv@v1.26.0

# Stage 3c: Debug Runtime (policy-engine wrapped in dlv, port 2346)
FROM envoyproxy/envoy:v1.37.1 AS debug

USER root
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends \
    tini \
    gettext-base \
    ca-certificates \
    curl \
    netcat-openbsd \
    dnsutils \
    iproute2 \
    iputils-ping \
    net-tools

RUN mkdir -p /app /var/run/api-platform /coverage

COPY --from=debug-dlv-builder /go/bin/dlv /usr/local/bin/dlv
COPY --from=policy-compiler /api-platform/output/gateway-runtime/policy-engine /app/policy-engine
COPY --from=policy-compiler /api-platform/gateway/build-manifest.yaml /app/build-manifest.yaml
COPY router/config/envoy-bootstrap.yaml /etc/envoy/envoy.yaml
COPY router/config/config-override.yaml /etc/envoy/config-override.yaml
COPY docker-entrypoint-debug.sh /usr/local/bin/docker-entrypoint.sh
COPY health-check.sh /usr/local/bin/health-check.sh
COPY --from=target configs/llm-pricing/ /home/wso2/conf/llm-pricing/

RUN chmod +x /app/policy-engine /usr/local/bin/dlv /usr/local/bin/docker-entrypoint.sh /usr/local/bin/health-check.sh

ENV GATEWAY_CONTROLLER_HOST=gateway-controller \
    LOG_LEVEL=info

# dlv requires elevated privileges (no USER wso2)
EXPOSE 2346 8080 8443 9901 9002 9003

ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/docker-entrypoint.sh"]

# Stage 4: Runtime (production — must remain last so default builds are unaffected)
FROM envoyproxy/envoy:v1.37.1 AS production

ARG VERSION=unknown
ARG ENABLE_COVERAGE=false

USER root
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends \
    tini \
    gettext-base \
    ca-certificates \
    curl \
    netcat-openbsd \
    dnsutils \
    iproute2 \
    iputils-ping \
    net-tools \
    python3

RUN mkdir -p /app /var/run/api-platform /coverage

# Copy Policy Engine binary
COPY --from=policy-compiler /api-platform/output/gateway-runtime/policy-engine /app/policy-engine
COPY --from=policy-compiler /api-platform/gateway/build-manifest.yaml /app/build-manifest.yaml

# Copy Python executor + policies + SDK from builder output
COPY --from=policy-compiler /api-platform/output/python-executor/ /app/python-executor/

# Copy pre-installed Python dependencies from the python-deps stage.
# pip and venv are NOT present in this image — only the installed packages.
COPY --from=python-deps /app/python-libs/ /app/python-libs/

# Set PYTHONPATH so Python can find the libs and executor modules
ENV PYTHONPATH="/app/python-libs:/app/python-executor"
COPY router/config/envoy-bootstrap.yaml /etc/envoy/envoy.yaml
COPY router/config/config-override.yaml /etc/envoy/config-override.yaml
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
COPY health-check.sh /usr/local/bin/health-check.sh
COPY --from=target configs/llm-pricing/ /home/wso2/conf/llm-pricing/

RUN chmod +x /app/policy-engine /usr/local/bin/docker-entrypoint.sh /usr/local/bin/health-check.sh

RUN groupadd -r -g 10001 wso2 && \
    useradd -r -u 10001 -g wso2 -s /sbin/nologin -c "WSO2 Application User" wso2 && \
    mkdir -p /home/wso2/conf/llm-pricing && \
    chown -R wso2:wso2 /app /var/run/api-platform /coverage /home/wso2 && \
    chmod -R 755 /app /var/run/api-platform /coverage /home/wso2

# Set GOCOVERDIR for coverage data collection (only effective when binary is built with -cover)
ENV GOCOVERDIR=/coverage

ENV GATEWAY_CONTROLLER_HOST=gateway-controller \
    LOG_LEVEL=info

USER wso2

EXPOSE 8080 8443 9901 9002 9003

# Metadata labels
LABEL org.opencontainers.image.title="API Platform Gateway Runtime"
LABEL org.opencontainers.image.description="API Platform Gateway Runtime with Envoy Router and Policy Engine"
LABEL org.opencontainers.image.vendor="WSO2"
LABEL org.opencontainers.image.version="${VERSION}"

ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/docker-entrypoint.sh"]
