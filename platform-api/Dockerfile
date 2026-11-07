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

# Platform API image.
#
# The only Dockerfile that defines the Platform API image. It compiles nothing: it
# copies the prebuilt binary for the target CPU from the `bin` stage, plus the
# schema files and resources the binary reads at runtime.
#
# Where the binary comes from:
#   - In the binary zip (wso2apip-platform-api-binary-<version>.zip, built by
#     `make binary`), the `bin` stage copies ./bin/, which holds
#     linux-amd64/platform-api and linux-arm64/platform-api, so a plain
#     `docker build .` there works. Our released images are built this way, from
#     the zip, so they hold exactly the zipped binaries.
#   - For builds from source (docker-bake.hcl, e.g. the coverage image), bake
#     replaces the `bin` stage with the binary Dockerfile.build compiles
#     (contexts bin = "target:compile").
#
# Changing the base image: edit the second FROM line. The binary is built with cgo,
# so the base must be glibc-based (not Alpine/musl). The steps below use apt-get,
# groupadd and useradd; replace them with your base's equivalents if it isn't
# Debian/Ubuntu, and keep the user's UID 10001.

ARG PORT=9243

# The prebuilt binaries: bin/linux-<arch>/platform-api from the build folder.
FROM scratch AS bin
COPY bin/ /

FROM ubuntu:24.04

# Set by `docker buildx build --platform ...`: amd64 or arm64.
ARG TARGETARCH
ARG PORT
# true for the experimental (event-gateway plugin) build; recorded as a label.
ARG EXPERIMENTAL=false
ARG USERNAME=wso2
ARG USER_UID=10001
ARG USER_GID=$USER_UID

LABEL org.opencontainers.image.experimental="${EXPERIMENTAL}"

# Install runtime dependencies
RUN apt-get update && apt-get upgrade -y \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Create non-root user
RUN groupadd -g $USER_UID appgroup && \
    useradd -u $USER_UID -g appgroup -s /bin/bash -m $USERNAME

# The binary for this CPU, and the files it reads at runtime
COPY --from=bin /linux-${TARGETARCH}/platform-api ./platform-api
COPY internal/database/schema.sql            ./internal/database/schema.sql
COPY internal/database/schema.postgres.sql   ./internal/database/schema.postgres.sql
COPY internal/database/schema.sqlserver.sql  ./internal/database/schema.sqlserver.sql
COPY internal/database/schema.sqlite.sql     ./internal/database/schema.sqlite.sql
COPY resources/default-llm-provider-templates ./resources/default-llm-provider-templates
COPY resources/openapi.yaml                  ./resources/openapi.yaml

# Used only by coverage-instrumented binaries (see Dockerfile.coverage).
ENV GOCOVERDIR=/coverage

# Create data directory, set ownership and OpenShift-compatible group permissions
RUN mkdir -p /app/data /app/data/certs && \
    chown -R wso2:appgroup /app && \
    chgrp -R 0 /app && \
    chmod -R g=u /app

EXPOSE ${PORT}

ENV LOG_LEVEL=INFO
ENV PORT=${PORT}

USER $USERNAME

ENTRYPOINT ["./platform-api"]
