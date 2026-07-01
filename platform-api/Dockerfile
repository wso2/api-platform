# --------------------------------------------------------------------
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# -----------------------------------------------------------------------

# Global build arguments
ARG PORT=9243

# Set EXPERIMENTAL=true to include the event-gateway plugin.
# The resulting image should be tagged with a "-experimental" suffix.
ARG EXPERIMENTAL=false

# Build stage
FROM --platform=$BUILDPLATFORM golang:1.26.2-bookworm AS builder
ARG TARGETARCH

# Build arguments for version information
ARG VERSION=0.0.1-SNAPSHOT
ARG GIT_COMMIT=unknown
ARG EXPERIMENTAL=false

WORKDIR /build

# CGO is required for mattn/go-sqlite3; install cross-compile toolchains.
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

# Copy go mod files for dependencies (needed for go.mod replace directives)
COPY --from=common go.mod go.sum /common/
COPY --from=httpkit go.mod go.sum /httpkit/

# Fetch dependencies before copying full source for better layer caching.
COPY src/go.mod src/go.sum ./
RUN go mod download

COPY --from=common . /common
COPY --from=httpkit . /httpkit
COPY src/ ./

RUN if [ "$TARGETARCH" = "amd64" ]; then \
        export CC=x86_64-linux-gnu-gcc; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
        export CC=aarch64-linux-gnu-gcc; \
    else \
        export CC=gcc; \
    fi && \
    BUILD_TAGS="" && \
    if [ "$EXPERIMENTAL" = "true" ]; then BUILD_TAGS="-tags experimental"; fi && \
    CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=${TARGETARCH} \
    go build -a \
        ${BUILD_TAGS} \
        -ldflags "-X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT}" \
        -o platform-api ./cmd/main.go

###########################
# Runtime Stage           #
###########################
FROM ubuntu:24.04

ARG PORT
ARG EXPERIMENTAL=false
ARG USERNAME=wso2
ARG USER_UID=10001
ARG USER_GID=$USER_UID

LABEL org.opencontainers.image.experimental="${EXPERIMENTAL}"

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN groupadd -g $USER_UID appgroup && \
    useradd -u $USER_UID -g appgroup -s /bin/bash -m $USERNAME

COPY --from=builder /build/platform-api .

# Schema files and LLM provider templates
COPY src/internal/database/schema.sql            ./schema.sql
COPY src/internal/database/schema.postgres.sql   ./schema.postgres.sql
COPY src/internal/database/schema.sqlserver.sql ./schema.sqlserver.sql
COPY src/internal/database/schema.sqlite.sql     ./schema.sqlite.sql
COPY src/resources/default-llm-provider-templates ./default-llm-provider-templates
COPY src/resources/openapi.yaml                  ./resources/openapi.yaml

RUN mkdir -p /app/data /app/data/certs && \
    chown -R wso2:appgroup /app

EXPOSE ${PORT}

ENV LOG_LEVEL=INFO
ENV PORT=${PORT}
# SQLite defaults — override DATABASE_DRIVER / DATABASE_* for PostgreSQL
ENV DATABASE_DRIVER=sqlite3
ENV DATABASE_DB_PATH=/app/data/api_platform.db
ENV DB_SCHEMA_PATH=./schema.sql
ENV LLM_TEMPLATE_DEFINITIONS_PATH=./default-llm-provider-templates
ENV TLS_CERT_DIR=/app/data/certs

USER $USERNAME

ENTRYPOINT ["./platform-api"]
