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

# Developer Portal Dockerfile
# Uses node:23-slim (Debian/glibc) so better-sqlite3 can compile its native binding.
# Build tools (python3, make, g++) are installed only in the builder stage and are not
# carried into the runtime image.

# Build stage — installs production dependencies (including native build of better-sqlite3)
FROM node:23-slim AS builder
WORKDIR /app
# Install native build tools required by better-sqlite3's node-gyp compilation.
# python3 is needed by node-gyp; make and g++ compile the C++ binding.
RUN apt-get update && apt-get install -y --no-install-recommends python3 make g++ && rm -rf /var/lib/apt/lists/*
COPY package*.json ./
RUN npm ci --omit=dev

# Runtime stage
FROM node:23-slim

# Install runtime utilities needed by the entrypoint (openssl for cert generation)
RUN apt-get update && apt-get install -y --no-install-recommends openssl curl && rm -rf /var/lib/apt/lists/*

# Set the working directory inside the container
WORKDIR /app

# Copy compiled node_modules from the builder stage
COPY --from=builder /app/node_modules ./node_modules

# Copy the rest of the application files
COPY . .

# Expose the application port
EXPOSE 8080

# Create a non-root user with UID 10001 to satisfy Checkov CKV_CHOREO_1
RUN groupadd -g 10001 appgroup && useradd -u 10001 -g appgroup -s /bin/bash -M appuser

# Pre-create the certs and logs directories owned by the non-root user so the
# entrypoint script can write self-signed certs, and the app can write log files.
RUN mkdir -p /app/certs /app/logs /app/data && chown -R 10001:10001 /app/certs /app/logs /app/data

RUN chmod +x /app/docker-entrypoint.sh

# Explicitly switch to UID 10001 instead of using "appuser"
USER 10001

ENTRYPOINT ["/app/docker-entrypoint.sh"]
