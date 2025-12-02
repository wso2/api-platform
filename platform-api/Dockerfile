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

# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /api-platform

COPY src/go.mod src/go.sum ./
RUN go mod download

COPY src/ ./

RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags "-s -w" \
    -o platform-api \
    ./cmd/main.go

# Runtime stage
FROM alpine:3.22.2

RUN apk add --no-cache ca-certificates sqlite
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S wso2 -G appgroup

WORKDIR /api-platform

COPY --from=builder /api-platform/platform-api .
RUN mkdir -p /api-platform/data && \
    chown -R wso2:appgroup /api-platform
COPY src/internal/database/schema.sql ./schema.sql

# Create volume for persistent data storage
VOLUME ["/api-platform/data"]

USER wso2

EXPOSE 8443

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 --spider --no-check-certificate https://localhost:8443/health || exit 1

# Set Gin to release mode
ENV GIN_MODE=release
ENV LOG_LEVEL=INFO
ENV DATABASE_DRIVER=sqlite3
ENV DB_SCHEMA_PATH=./schema.sql
ENV DATABASE_PATH=./data/api_platform.db

CMD ["./platform-api"]
