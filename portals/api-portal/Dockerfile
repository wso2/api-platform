# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

# API Portal Dockerfile
# Uses node:24-alpine (musl libc) — better-sqlite3 compiles its native binding against musl
# just as well as glibc (verified). Build tools (python3, make, g++) are installed only in
# the builder stage and are not carried into the runtime image.

# Build stage — installs production dependencies (including native build of better-sqlite3)
FROM node:24-alpine AS builder
WORKDIR /app
# Install native build tools required by better-sqlite3's node-gyp compilation.
# python3 is needed by node-gyp; make and g++ compile the C++ binding.
RUN apk add --no-cache python3 make g++
COPY package*.json ./
RUN npm ci --omit=dev

# Runtime stage
FROM node:24-alpine

ARG ENABLE_COVERAGE=false

# Set the working directory inside the container
WORKDIR /app

# Copy compiled node_modules from the builder stage
COPY --from=builder /app/node_modules ./node_modules

# Copy the rest of the application files
COPY . .

# Expose the application port
EXPOSE 9543

# Create a non-root user with UID 10001 to satisfy Checkov CKV_CHOREO_1
RUN addgroup -g 10001 appgroup && adduser -u 10001 -G appgroup -s /bin/sh -D -H appuser

# Pre-create the logs and data directories owned by the non-root user so the
# app can write log files and its database. The TLS certificate is bind-mounted
# read-only at /etc/api-portal/tls (generated host-side by setup.sh) — never
# written to by the container.
RUN mkdir -p /app/logs /app/data /coverage && chown -R 10001:10001 /app/logs /app/data /coverage

RUN chmod +x /app/docker-entrypoint.sh

# Explicitly switch to UID 10001 instead of using "appuser"
USER 10001

ENTRYPOINT ["/app/docker-entrypoint.sh"]
