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
#
# Builds the API Portal image from a folder that holds an `api-portal/` folder, which
# the COPY lines below read from. That folder comes from the binary zip
# (wso2apip-api-portal-binary-<version>.zip, built by `make binary`), which holds no
# Dockerfile: unzip it next to this Dockerfile and run a plain `docker build .`:
#
#   ./Dockerfile
#   ./api-portal/      <- unzipped binary zip
#
# This is the Dockerfile for our released images, which are built from that zip the
# same way, and the one customers use to build their own image, for example on a
# different base image. Builds from source use the repository's portals/ folder,
# which holds api-portal/ too.
#
# Changing the base image: edit both FROM lines. The base must provide Node.js 24,
# and npm in the build stage. The steps below work on any Linux base with a POSIX
# shell, including bases whose default user isn't root: each stage switches to root
# for its setup steps, and the image still runs as UID 10001.
#
# The image's runtime contract, which the pack's docker-compose.yaml and the Helm
# chart rely on: working directory /app, UID 10001, writable /app/data and /app/logs,
# port 9543, base config at $APIP_AP_CONFIG_PATH, started with `node src/server.js`.
# Keep it unchanged when editing this file.
#
# The integration tests' coverage image is built on top of this one by
# Dockerfile.coverage; nothing here is specific to it.

# Build stage: installs the production dependencies. .npmrc sets ignore-scripts=true,
# so no package's install scripts run.
FROM node:24-alpine AS build
# Build steps run as root; some base images (e.g. Red Hat's Node.js images) default
# to a non-root user, which can't create /app or change ownership below.
USER 0
WORKDIR /app
COPY api-portal/package.json api-portal/package-lock.json api-portal/.npmrc ./
RUN npm ci --omit=dev

# Runtime image
FROM node:24-alpine
# Root for the setup steps below; the image runs as UID 10001 (USER at the end).
USER 0
WORKDIR /app

COPY --from=build /app/node_modules ./node_modules
COPY api-portal/package.json api-portal/package-lock.json ./
COPY api-portal/src ./src
COPY api-portal/database ./database
COPY api-portal/resources ./resources

# npm and npx are never used at runtime. This removes the copy the official node
# image ships; on a base that keeps npm elsewhere it removes nothing.
RUN rm -rf /usr/local/lib/node_modules/npm /usr/local/bin/npm /usr/local/bin/npx

# The app writes its database to /app/data and optional log files to /app/logs.
# Owned by UID 10001 and the root group, with group permissions matching the owner,
# so OpenShift's arbitrary UID (always in GID 0) can write to them too. The TLS
# certificate is bind-mounted read-only at /etc/api-portal/tls, never written here.
RUN mkdir -p /app/data /app/logs \
 && chown -R 10001:0 /app/data /app/logs \
 && chmod -R g=u /app/data /app/logs

# The base config file. Extra `--config <path>` arguments passed to the container
# are layered on top of it, last wins.
ENV APIP_AP_CONFIG_PATH=/app/configs/config.toml

EXPOSE 9543

# Non-root UID/GID 10001 (satisfies Checkov CKV_CHOREO_1). Numeric, so no account
# has to exist and the line works on any base image.
USER 10001:10001

ENTRYPOINT ["node", "src/server.js"]
