# syntax=docker/dockerfile:1
# Builds the Platform API image from a downloaded API Portal binary ZIP or its
# extracted directory. See README.md.
#
# The binary uses cgo, so a different base image must be glibc-based (not Alpine).
ARG BASE_IMAGE=ubuntu:24.04
ARG PORT=9243

# BINARY_PATH is a ZIP file or extracted directory inside the build context.
# It may contain the whole API Portal binary or only its platform-api directory.
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
    if [ -d "$root/platform-api" ]; then \
        component="$root/platform-api"; \
    elif [ -d "$root/bin/linux-amd64" ] || [ -d "$root/bin/linux-arm64" ]; then \
        component="$root"; \
    else \
        set -- "$root"/*/platform-api; \
        if [ "$#" -ne 1 ] || [ ! -d "$1" ]; then \
            echo "Error: BINARY_PATH does not contain a platform-api binary package" >&2; \
            exit 1; \
        fi; \
        component="$1"; \
    fi; \
    mkdir /binary; \
    cp -a "$component"/. /binary/; \
    rm -rf /tmp/input /tmp/unpacked

FROM ${BASE_IMAGE} AS production
ARG TARGETARCH
ARG PORT
ARG PRODUCT_VERSION
ARG VERSION=${PRODUCT_VERSION:-unknown}
# true for the experimental (event-gateway plugin) build; recorded as a label.
ARG EXPERIMENTAL=false
ARG USERNAME=wso2
ARG USER_UID=10001
ARG USER_GID=$USER_UID

RUN case "$TARGETARCH" in amd64|arm64) ;; *) echo "Unsupported architecture: $TARGETARCH" >&2; exit 1 ;; esac && \
    apt-get update && apt-get upgrade -y && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN groupadd -g $USER_GID appgroup && \
    useradd -u $USER_UID -g appgroup -s /bin/bash -m $USERNAME

COPY --from=binary /binary/bin/linux-${TARGETARCH}/platform-api ./platform-api
COPY --from=binary /binary/LICENSE ./LICENSE
COPY --from=binary /binary/internal/database/schema.sql            ./internal/database/schema.sql
COPY --from=binary /binary/internal/database/schema.postgres.sql   ./internal/database/schema.postgres.sql
COPY --from=binary /binary/internal/database/schema.sqlserver.sql  ./internal/database/schema.sqlserver.sql
COPY --from=binary /binary/internal/database/schema.sqlite.sql     ./internal/database/schema.sqlite.sql
COPY --from=binary /binary/resources/default-llm-provider-templates ./resources/default-llm-provider-templates
COPY --from=binary /binary/resources/openapi.yaml                  ./resources/openapi.yaml

# Writable by the user and the root group (OpenShift runs with an arbitrary UID in GID 0).
RUN mkdir -p /app/data /app/data/certs && \
    chown -R $USERNAME:appgroup /app && \
    chgrp -R 0 /app && \
    chmod -R g=u /app

EXPOSE ${PORT}

ENV LOG_LEVEL=INFO
ENV PORT=${PORT}

USER $USERNAME
LABEL org.opencontainers.image.title="WSO2 API Platform Platform API" \
      org.opencontainers.image.vendor="WSO2" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.experimental="${EXPERIMENTAL}"
ENTRYPOINT ["./platform-api"]
