# Stage 1: Build the Go application
# Using Debian-based image for better cross-compilation support
FROM --platform=$BUILDPLATFORM golang:1.25.5-bookworm AS builder
ARG TARGETARCH
ARG VERSION=0.0.1-SNAPSHOT
ARG GIT_COMMIT=unknown

# Coverage build argument - set to "true" for coverage-instrumented builds
ARG ENABLE_COVERAGE=false

WORKDIR /build

# Install build dependencies for SQLite (CGO) and cross-compilation toolchains
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libc6-dev \
    libsqlite3-dev \
    && if [ "$TARGETARCH" = "amd64" ]; then \
        apt-get install -y --no-install-recommends gcc-x86-64-linux-gnu libc6-dev-amd64-cross; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
        apt-get install -y --no-install-recommends gcc-aarch64-linux-gnu libc6-dev-arm64-cross; \
    fi \
    && rm -rf /var/lib/apt/lists/*

# Copy go mod files for dependencies (needed for go.mod replace directive)
# Copying only the mod files first ensures that the go mod download cache 
# is only invalidated when dependencies change, not when source code changes.
COPY --from=sdk go.mod go.sum /sdk/
COPY --from=common go.mod go.sum /common/

COPY go.mod go.sum ./
RUN go mod download

COPY --from=sdk . /sdk
COPY --from=common . /common
COPY . ./

# Build with CGO (required for SQLite), set CC for cross-compilation, add -cover flag if enabled
RUN if [ "$TARGETARCH" = "amd64" ]; then \
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
    CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=${TARGETARCH} \
    go build $COVER_FLAG -a \
    -ldflags "-X main.Version=${VERSION} \
              -X main.GitCommit=${GIT_COMMIT} \
              -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o controller cmd/controller/main.go

# Stage 2: Runtime image (Debian slim for glibc compatibility)
FROM debian:bookworm-slim

# Re-declare ARG after FROM to use it in this stage
ARG ENABLE_COVERAGE=false

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /build/controller .
COPY default-policies /app/default-policies
COPY lua /app/lua
COPY default-llm-provider-templates /app/default-llm-provider-templates

RUN mkdir -p /app/data && \
    if [ "$ENABLE_COVERAGE" = "true" ]; then mkdir -p /coverage; fi

RUN groupadd -r -g 10001 wso2 && \
    useradd -r -u 10001 -g wso2 -s /sbin/nologin -c "WSO2 Application User" wso2 && \
    chown -R wso2:wso2 /app/data && \
    chmod -R 755 /app/data && \
    if [ "$ENABLE_COVERAGE" = "true" ]; then \
        chown -R wso2:wso2 /coverage && \
        chmod -R 755 /coverage; \
    fi

# Set GOCOVERDIR for coverage data collection (only effective when binary is built with -cover)
ENV GOCOVERDIR=/coverage

USER wso2

EXPOSE 9090 18000

ENTRYPOINT ["/app/controller"]
