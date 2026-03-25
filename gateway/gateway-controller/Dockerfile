# Stage 1: Build the Go application
# Using Debian-based image for better cross-compilation support
FROM --platform=$BUILDPLATFORM golang:1.25.7-bookworm AS builder
ARG TARGETARCH
ARG VERSION=0.0.1-SNAPSHOT
ARG GIT_COMMIT=unknown

# Coverage build argument - set to "true" for coverage-instrumented builds
ARG ENABLE_COVERAGE=false

# Debug build argument - set to "true" for debug builds with dlv support (disables optimizations)
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

# Copy go mod files for dependencies (needed for go.mod replace directive)
# Copying only the mod files first ensures that the go mod download cache 
# is only invalidated when dependencies change, not when source code changes.
COPY --from=sdk go.mod go.sum /sdk/
COPY --from=sdk-core go.mod /sdk/core/
COPY --from=common go.mod go.sum /common/

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY --from=sdk . /sdk
COPY --from=sdk-core . /sdk/core
COPY --from=common . /common
COPY . ./

# Build with CGO (required for SQLite), set CC for cross-compilation, add -cover/-gcflags if enabled
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
    if [ "$ENABLE_DEBUG" = "true" ]; then \
        go build $COVER_FLAG \
        -gcflags "all=-N -l" \
        -ldflags "-X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildDate=${BUILD_DATE}" \
        -o controller cmd/controller/main.go; \
    else \
        go build $COVER_FLAG \
        -ldflags "-X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildDate=${BUILD_DATE}" \
        -o controller cmd/controller/main.go; \
    fi

# Stage: debug (select with --target debug; NOT part of the default build)
# Full Go image needed to install dlv via go install; acceptable for dev-only use.
# Runs as root — dlv requires ptrace; container security settings handle isolation.
FROM golang:1.25.7-bookworm AS debug
WORKDIR /app

# Install dlv (Delve debugger)
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/go-delve/delve/cmd/dlv@v1.26.0

# Copy debug-compiled binary and required runtime assets
COPY --from=builder /build/controller .
COPY default-policies /app/default-policies
COPY lua /app/lua
COPY default-llm-provider-templates /app/default-llm-provider-templates
RUN mkdir -p /app/data

EXPOSE 2345 9090 18000

# Run controller under dlv in headless mode, waiting for debugger attach on :2345
ENTRYPOINT ["/go/bin/dlv", "exec", "/app/controller", \
            "--listen=:2345", "--headless=true", \
            "--api-version=2", "--accept-multiclient", "--"]

# Stage 2: Prepare runtime filesystem contents in a shell-capable stage
FROM builder AS runtime-prep

RUN mkdir -p /runtimefs/app/data /runtimefs/coverage

# Stage 3: Runtime image (distroless glibc runtime for CGO-enabled SQLite support)
FROM gcr.io/distroless/cc-debian12:nonroot AS production

# Re-declare ARG after FROM to use it in metadata labels
ARG VERSION=0.0.1-SNAPSHOT

WORKDIR /app

COPY --from=builder --chown=65532:65532 /build/controller /app/controller
COPY --chown=65532:65532 default-policies /app/default-policies
COPY --chown=65532:65532 lua /app/lua
COPY --chown=65532:65532 default-llm-provider-templates /app/default-llm-provider-templates
COPY --from=runtime-prep --chown=65532:65532 /runtimefs/app/data /app/data
COPY --from=runtime-prep --chown=65532:65532 /runtimefs/coverage /coverage

# Set GOCOVERDIR for coverage data collection (only effective when binary is built with -cover)
ENV GOCOVERDIR=/coverage

USER 65532:65532

EXPOSE 9090 18000

# Metadata labels
LABEL org.opencontainers.image.title="API Platform Gateway Controller"
LABEL org.opencontainers.image.description="API Platform Gateway Controller with xDS control plane"
LABEL org.opencontainers.image.vendor="WSO2"
LABEL org.opencontainers.image.version="${VERSION}"

ENTRYPOINT ["/app/controller"]
