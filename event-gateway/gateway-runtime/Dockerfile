# Stage 1: Build the Go application
FROM --platform=$BUILDPLATFORM golang:1.26.2-bookworm AS builder
ARG TARGETARCH
ARG VERSION=0.1.0-SNAPSHOT
ARG GIT_COMMIT=unknown

WORKDIR /build

# Copy go mod files for dependency caching
COPY --from=sdk-core go.mod /sdk/core/
COPY --from=common go.mod go.sum /common/
COPY --from=policy-engine go.mod go.sum /gateway/gateway-runtime/policy-engine/

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY --from=sdk-core . /sdk/core
COPY --from=common . /common
COPY --from=policy-engine . /gateway/gateway-runtime/policy-engine
COPY . ./

# Build — pure Go, no CGO needed (no SQLite)
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    export CGO_ENABLED=0 && \
    export GOOS=linux && \
    export GOARCH=${TARGETARCH} && \
    BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) && \
    go build \
        -ldflags "-X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildDate=${BUILD_DATE}" \
        -o event-gateway ./cmd/event-gateway

# Stage 2: Minimal runtime image
FROM alpine:3.22

RUN apk add --no-cache ca-certificates bash tini && \
    addgroup -S egw && adduser -S egw -G egw

WORKDIR /app

COPY --from=builder /build/event-gateway /app/event-gateway
COPY configs/config.toml /etc/event-gateway/config.toml
COPY configs/channels.yaml /etc/event-gateway/channels.yaml
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh

USER egw

EXPOSE 8080 8081 9002 9003

ENTRYPOINT ["/sbin/tini", "--", "/app/docker-entrypoint.sh"]
