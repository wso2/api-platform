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

# Build stage
FROM --platform=$BUILDPLATFORM golang:1.25.5-bookworm AS builder
ARG TARGETARCH

# Build arguments for version information
ARG VERSION=0.0.1-SNAPSHOT
ARG GIT_COMMIT=unknown

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

COPY src/go.mod src/go.sum ./
RUN go mod download

COPY src/ ./

# Build the binary with CGO enabled (required for mattn/go-sqlite3)
# Set CC for cross-compilation based on target architecture
RUN if [ "$TARGETARCH" = "amd64" ]; then \
        export CC=x86_64-linux-gnu-gcc; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
        export CC=aarch64-linux-gnu-gcc; \
    else \
        export CC=gcc; \
    fi && \
    CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=${TARGETARCH} \
    go build -a \
    -ldflags "-X main.Version=${VERSION} \
              -X main.GitCommit=${GIT_COMMIT} \
              -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o platform-api ./cmd/main.go

# Stage 2: Create minimal runtime image
# Using Ubuntu 24.04 for glibc compatibility with builder stage
FROM ubuntu:24.04

ARG USERNAME=wso2
ARG USER_UID=10001
ARG USER_GID=$USER_UID

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN groupadd -g $USER_UID appgroup && \
    useradd -u $USER_UID -g appgroup -s /bin/bash -m $USERNAME

COPY --from=builder /build/platform-api .
RUN mkdir -p /app/data && \
    chown -R wso2:appgroup /app
COPY src/internal/database/schema.sql ./schema.sql
COPY src/internal/database/schema.postgres.sql ./schema.postgres.sql
COPY src/internal/database/schema.sqlite.sql ./schema.sqlite.sql
COPY src/resources/default-llm-provider-templates ./default-llm-provider-templates

ARG PORT
EXPOSE ${PORT}

ENV DRIVER=sqlite3
ENV LOG_LEVEL=INFO
ENV DB_SCHEMA_PATH=./schema.sql
ENV LLM_TEMPLATE_DEFINITIONS_PATH=./default-llm-provider-templates
# For SQLite: configured via DATABASE_DB_PATH to avoid clashing with OS PATH
ENV DATABASE_DB_PATH=/app/data/api_platform.db
ENV PORT=${PORT}

USER $USERNAME

CMD ["./platform-api"]
