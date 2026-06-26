FROM --platform=$BUILDPLATFORM golang:1.26.2-bookworm AS builder
ARG TARGETARCH
ARG VERSION=0.0.1-SNAPSHOT
ARG GIT_COMMIT=unknown
ARG ENABLE_COVERAGE=false
ARG ENABLE_DEBUG=false

WORKDIR /build

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

# Copy dependency mod files for layer caching
COPY --from=sdk-core go.mod /sdk/core/
COPY --from=common go.mod go.sum /common/
COPY --from=gateway-controller go.mod go.sum /gateway/gateway-controller/

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY --from=sdk-core . /sdk/core
COPY --from=common . /common
COPY --from=gateway-controller . /gateway/gateway-controller
COPY . ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    if [ "$TARGETARCH" = "amd64" ]; then \
        export CC=x86_64-linux-gnu-gcc; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
        export CC=aarch64-linux-gnu-gcc; \
    else \
        export CC=gcc; \
    fi && \
    if [ "$ENABLE_COVERAGE" = "true" ]; then COVER_FLAG="-cover"; else COVER_FLAG=""; fi && \
    export CGO_ENABLED=1 GOOS=linux GOARCH=${TARGETARCH} && \
    BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) && \
    VERSION_PKG=github.com/wso2/api-platform/gateway/gateway-controller/pkg/version && \
    LDFLAGS="-X ${VERSION_PKG}.Version=${VERSION} -X ${VERSION_PKG}.GitCommit=${GIT_COMMIT} -X ${VERSION_PKG}.BuildDate=${BUILD_DATE}" && \
    if [ "$ENABLE_DEBUG" = "true" ]; then \
        go build $COVER_FLAG -gcflags "all=-N -l" -ldflags "$LDFLAGS" -o controller ./cmd/controller; \
    else \
        go build $COVER_FLAG -ldflags "$LDFLAGS" -o controller ./cmd/controller; \
    fi

FROM ubuntu:24.04 AS production
ARG VERSION=0.0.1-SNAPSHOT
ARG ENABLE_COVERAGE=false

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends ca-certificates wget && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=build-manifest build.yaml /app/build-manifest.yaml
COPY --from=builder /build/controller .
COPY --from=policies . /app/default-policies
COPY --from=gateway-controller lua /app/lua
COPY --from=gateway-controller default-llm-provider-templates /app/default-llm-provider-templates

RUN mkdir -p /app/data && \
    if [ "$ENABLE_COVERAGE" = "true" ]; then mkdir -p /coverage; fi

RUN groupadd -r -g 10001 wso2 && \
    useradd -r -u 10001 -g wso2 -s /usr/sbin/nologin -c "WSO2 Application User" wso2 && \
    chown -R wso2:wso2 /app/data && \
    chmod -R 755 /app/data && \
    if [ "$ENABLE_COVERAGE" = "true" ]; then \
        chown -R wso2:wso2 /coverage && chmod -R 755 /coverage; \
    fi

ENV GOCOVERDIR=/coverage
USER wso2

EXPOSE 9090 18000

LABEL org.opencontainers.image.title="API Platform Event Gateway Controller"
LABEL org.opencontainers.image.description="API Platform Event Gateway Controller with WebSub/WebBroker support"
LABEL org.opencontainers.image.vendor="WSO2"
LABEL org.opencontainers.image.version="${VERSION}"

ENTRYPOINT ["/app/controller"]
