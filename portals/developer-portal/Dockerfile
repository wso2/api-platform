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
# Use Node.js 23 Alpine for a smaller base image
FROM node:23-alpine

# Install only the runtime utilities needed by the entrypoint (bash, openssl)
RUN apk add --no-cache bash openssl

# Set the working directory inside the container
WORKDIR /app

# Copy package.json and package-lock.json first to leverage Docker caching
COPY package*.json ./

# Install dependencies using npm ci for faster, cleaner installations in production
RUN npm ci --only=production --ignore-scripts

# Copy the rest of the application files
COPY . .

# Expose the application port
EXPOSE 8080

# Create a non-root user with UID 10001 to satisfy Checkov CKV_CHOREO_1
RUN addgroup -g 10001 appgroup && adduser -D -u 10001 -G appgroup -s /bin/bash appuser

# Pre-create the certs and logs directories owned by the non-root user so the
# entrypoint script can write self-signed certs, and the app can write log files.
RUN mkdir -p /app/certs /app/logs && chown -R 10001:10001 /app/certs /app/logs

RUN chmod +x /app/docker-entrypoint.sh

# Explicitly switch to UID 10001 instead of using "appuser"
USER 10001

ENTRYPOINT ["/app/docker-entrypoint.sh"]
