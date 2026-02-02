# Stage 1: Build the Go application
# Using Debian-based image for better cross-compilation support
FROM --platform=$BUILDPLATFORM golang:1.25.5-bookworm AS builder
ARG TARGETARCH

# Build arguments for version information
ARG VERSION=0.0.1-SNAPSHOT
ARG GIT_COMMIT=unknown

# Coverage build argument - set to "true" for coverage-instrumented builds
ARG ENABLE_COVERAGE=false

WORKDIR /build

# Install build dependencies for SQLite (CGO requirement)
# Install cross-compilation toolchains based on target architecture
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

# Copy main component go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy full source code for SDK and Common
COPY --from=sdk . /sdk
COPY --from=common . /common

# Copy component source code
COPY . ./

# Build the binary with CGO enabled (required for mattn/go-sqlite3)
# Set CC for cross-compilation based on target architecture
# Add -cover flag when ENABLE_COVERAGE=true for coverage instrumentation
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

# Stage 2: Create minimal runtime image
# Using Debian slim for glibc compatibility with builder stage
FROM debian:bookworm-slim

# Re-declare ARG after FROM to use it in this stage
ARG ENABLE_COVERAGE=false

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/controller .

# Copy default configuration file


# Copy policy definitions
COPY default-policies /app/default-policies

# Copy default llm provider templates
COPY default-llm-provider-templates /app/default-llm-provider-templates

# Create data directory for SQLite database
RUN mkdir -p /app/data

# Create coverage output directory if coverage is enabled
RUN if [ "$ENABLE_COVERAGE" = "true" ]; then mkdir -p /coverage; fi

# Set GOCOVERDIR for coverage data collection (only effective when binary is built with -cover)
ENV GOCOVERDIR=/coverage

# Expose ports
EXPOSE 9090 18000

# Run the application
ENTRYPOINT ["/app/controller"]
