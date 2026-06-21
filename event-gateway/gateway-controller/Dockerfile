# Stage 1: Build the Go application
FROM --platform=$BUILDPLATFORM golang:1.26.2-bookworm AS builder
ARG TARGETARCH
ARG VERSION=0.0.1-SNAPSHOT
ARG GIT_COMMIT=unknown

# Coverage build argument - set to "true" for coverage-instrumented builds
ARG ENABLE_COVERAGE=false

# Debug build argument - set to "true" for debug builds with dlv support
ARG ENABLE_DEBUG=false

WORKDIR /build

# Install build dependencies for SQLite (CGO) and cross-compilation toolchains
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libc6-dev \
    libsqlite3-dev \
    && if [ "$TARGETARCH" = "amd64" ]; then \
        apt-get install -y --no-install-recommends gcc-x86-64-linux-gnu libc6-dev-amd64-cross; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
        apt-get install -y --no-install-recommends gcc-aarch64-linux-gnu libc6-dev-arm64-cross; \
    fi

# Copy go mod files for dependencies (needed for go.mod replace directives)
COPY --from=sdk-core go.mod /sdk/core/
COPY --from=common go.mod go.sum /common/
COPY --from=gateway-controller go.mod go.sum /gateway/gateway-controller/

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy full source trees needed by replace directives
COPY --from=sdk-core . /sdk/core
COPY --from=common . /common
COPY --from=gateway-controller . /gateway/gateway-controller
COPY . ./

# Build with CGO (required for SQLite), set CC for cross-compilation
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    if [ "$TARGETARCH" = "amd64" ]; then \
        export CC=x86_64-linux-gnu-gcc; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
        export CC=aarch64-linux-gnu-gcc; \
    else \
        export CC=gcc; \
    fi && \
    if [ "$ENABLE_COVERAGE" = "true" ]; then \
        COVER_FLAG="-cover"; \
    else \
        COVER_FLAG=""; \
    fi && \
    export CGO_ENABLED=1 && \
    export GOOS=linux && \
    export GOARCH=${TARGETARCH} && \
    BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) && \
    VERSION_PKG=github.com/wso2/api-platform/gateway/gateway-controller/pkg/version && \
    LDFLAGS="-X ${VERSION_PKG}.Version=${VERSION} -X ${VERSION_PKG}.GitCommit=${GIT_COMMIT} -X ${VERSION_PKG}.BuildDate=${BUILD_DATE}" && \
    if [ "$ENABLE_DEBUG" = "true" ]; then \
        go build $COVER_FLAG \
        -gcflags "all=-N -l" \
        -ldflags "$LDFLAGS" \
        -o event-gateway-controller ./cmd/main; \
    else \
        go build $COVER_FLAG \
        -ldflags "$LDFLAGS" \
        -o event-gateway-controller ./cmd/main; \
    fi

# Stage: debug (select with --target debug; NOT part of the default build)
FROM golang:1.26.2-bookworm AS debug
WORKDIR /app

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/go-delve/delve/cmd/dlv@v1.26.0

COPY --from=builder /build/event-gateway-controller .
COPY --from=target LICENSE /app/LICENSE

RUN mkdir -p /app/data

EXPOSE 2345 9090 18000

ENTRYPOINT ["/go/bin/dlv", "exec", "/app/event-gateway-controller", \
            "--listen=:2345", "--headless=true", \
            "--api-version=2", "--accept-multiclient", "--"]

# Stage 2: Runtime image
FROM ubuntu:24.04 AS production

ARG VERSION=0.0.1-SNAPSHOT
ARG ENABLE_COVERAGE=false

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    wget && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /build/event-gateway-controller .
COPY --from=target LICENSE /app/LICENSE

RUN mkdir -p /app/data && \
    if [ "$ENABLE_COVERAGE" = "true" ]; then mkdir -p /coverage; fi

RUN groupadd -r -g 10001 wso2 && \
    useradd -r -u 10001 -g wso2 -s /usr/sbin/nologin -c "WSO2 Application User" wso2 && \
    chown -R wso2:wso2 /app/data && \
    chmod -R 755 /app/data && \
    if [ "$ENABLE_COVERAGE" = "true" ]; then \
        chown -R wso2:wso2 /coverage && \
        chmod -R 755 /coverage; \
    fi

ENV GOCOVERDIR=/coverage

USER wso2

EXPOSE 9090 18000

LABEL org.opencontainers.image.title="API Platform Event Gateway Controller"
LABEL org.opencontainers.image.description="Event Gateway Controller with WebSub/WebBroker support and xDS control plane"
LABEL org.opencontainers.image.vendor="WSO2"
LABEL org.opencontainers.image.version="${VERSION}"

ENTRYPOINT ["/app/event-gateway-controller"]
