# syntax=docker/dockerfile:1
# Builds the API Portal image from a downloaded API Portal binary ZIP or its
# extracted directory. See README.md.
#
# When changing the base image, keep the image's runtime contract: working
# directory /app, UID 10001, writable /app/data and /app/logs, port 9543, base
# config at $APIP_AP_CONFIG_PATH, started with `node src/server.js`. The base
# image must provide Node.js 24 and npm.
ARG BASE_IMAGE=node:24-alpine

# BINARY_PATH is a ZIP file or extracted directory inside the build context.
# It may contain the whole API Portal binary or only its api-portal directory.
FROM --platform=$BUILDPLATFORM ubuntu:24.04 AS binary
RUN apt-get update && \
    apt-get install -y --no-install-recommends unzip && \
    rm -rf /var/lib/apt/lists/*
ARG BINARY_PATH=.
COPY ${BINARY_PATH} /tmp/input/
RUN set -eu; \
    root=/tmp/input; \
    set -- "$root"/*.zip; \
    if [ -f "$1" ]; then \
        if [ "$#" -ne 1 ]; then \
            echo "Error: BINARY_PATH must identify one ZIP or an extracted binary directory" >&2; \
            exit 1; \
        fi; \
        mkdir /tmp/unpacked; \
        unzip -q "$1" -d /tmp/unpacked; \
        root=/tmp/unpacked; \
    fi; \
    if [ -d "$root/api-portal" ]; then \
        component="$root/api-portal"; \
    elif [ -f "$root/package.json" ] && [ -d "$root/src" ]; then \
        component="$root"; \
    else \
        set -- "$root"/*/api-portal; \
        if [ "$#" -ne 1 ] || [ ! -d "$1" ]; then \
            echo "Error: BINARY_PATH does not contain an api-portal binary package" >&2; \
            exit 1; \
        fi; \
        component="$1"; \
    fi; \
    mkdir /binary; \
    cp -a "$component"/. /binary/; \
    rm -rf /tmp/input /tmp/unpacked

FROM ${BASE_IMAGE} AS build
# Root for the setup steps; some base images default to a non-root user.
USER 0
WORKDIR /app
COPY --from=binary /binary/package.json /binary/package-lock.json /binary/.npmrc ./
RUN npm ci --omit=dev

FROM ${BASE_IMAGE} AS production
ARG PRODUCT_VERSION
ARG VERSION=${PRODUCT_VERSION:-unknown}
USER 0
WORKDIR /app
COPY --from=build /app/node_modules ./node_modules
COPY --from=binary /binary/package.json /binary/package-lock.json /binary/LICENSE ./
COPY --from=binary /binary/src ./src
COPY --from=binary /binary/database ./database
COPY --from=binary /binary/resources ./resources

# npm and npx are not used at runtime.
RUN rm -rf /usr/local/lib/node_modules/npm /usr/local/bin/npm /usr/local/bin/npx

# Writable by UID 10001 and the root group (OpenShift runs with an arbitrary UID in GID 0).
RUN mkdir -p /app/data /app/logs \
 && chown -R 10001:0 /app/data /app/logs \
 && chmod -R g=u /app/data /app/logs

# Base config file; `--config <path>` arguments are layered on top of it.
ENV APIP_AP_CONFIG_PATH=/app/configs/config.toml

EXPOSE 9543
USER 10001:10001
LABEL org.opencontainers.image.title="WSO2 API Platform API Portal" \
      org.opencontainers.image.vendor="WSO2" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.licenses="Apache-2.0"
ENTRYPOINT ["node", "src/server.js"]
