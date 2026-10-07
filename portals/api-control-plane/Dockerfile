# syntax=docker/dockerfile:1
# -----------------------------------------------------------------------------
# Production image for the API Platform management console.
#
# Same three-stage shape as ../ai-workspace/Dockerfile (SPA build -> BFF build
# -> alpine runtime), but the BFF build has no relative-replace dependency on
# common/httpkit — it's a self-contained module by design (see bff/go.mod) so
# it vendors cleanly into apim-saas's git subtree — so, unlike ai-workspace,
# there is no need for named build contexts here.
#
# Runtime configuration (IdP client id/secret, control-plane URL, ...) is NOT
# baked in: mount a config.toml at /etc/api-control-plane/config.toml. The BFF
# owns authentication (file-based by default, or OIDC against a configured
# external IdP) and serves the SPA itself — there is no nginx in this image.
#
#   docker build -t api-control-plane .
# -----------------------------------------------------------------------------

###########################
# Stage 1 — SPA build     #
###########################
# Pinned to the build host: the output is static assets, identical for every
# target platform, so emulating node per-platform costs minutes and buys
# nothing.
FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /app

# Separate dependency install for better layer caching.
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts

COPY . .
RUN VITE_APP_BASE_PATH=/ npm run build

###########################
# Stage 2 — BFF build     #
###########################
# Cross-compilation is used instead of emulation. The toolchain runs natively on
# the build host, while GOARCH specifies the target architecture. Building an
# amd64 binary under QEMU on an arm64 host is substantially slower and may cause
# the Go garbage collector to fail with "marked free object in span". Because
# CGO is disabled, cross-compilation introduces no additional runtime
# dependencies. This approach is consistent with the platform-api and gateway
# images.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS bff
ARG TARGETOS
ARG TARGETARCH

# The BFF depends on the shared httpkit module (guarded HTTP client, netguard)
# via a relative replace directive, so the module tree is reproduced under /src
# and httpkit is supplied as a named build context:
#   docker buildx build --build-context httpkit=../../httpkit ...
# WORKDIR therefore mirrors the repository layout, so the ../../../httpkit in
# bff/go.mod resolves to /src/httpkit. The workspace is disabled so the image
# builds from these copies alone.
WORKDIR /src/portals/api-control-plane/bff

# go.mod/go.sum first for better layer caching.
COPY --from=httpkit go.mod go.sum /src/httpkit/
COPY bff/go.mod bff/go.sum ./
ENV CGO_ENABLED=0 GOWORK=off
RUN go mod download

COPY --from=httpkit . /src/httpkit
COPY bff/ ./
RUN GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w" -o /bff .

###########################
# Stage 3 — Runtime       #
###########################
FROM alpine:3.24

ARG USERNAME=apicontrolplane
ARG USER_UID=10001
ARG USER_GID=$USER_UID

RUN addgroup -g $USER_GID $USERNAME && \
    adduser -D -u $USER_UID -G $USERNAME $USERNAME && \
    apk upgrade --no-cache && \
    apk add --no-cache curl ca-certificates

WORKDIR /app

# Built SPA (served by the BFF as static files) and the BFF binary.
COPY --chown=$USER_UID:$USER_GID --from=web /app/build /app
COPY --chown=$USER_UID:$USER_GID --from=bff /bff /usr/local/bin/bff

USER $USER_UID

# No ENV config defaults are baked in on purpose: configure a deployment by
# mounting a config.toml at /etc/api-control-plane/config.toml; its
# '{{ env "APIP_ACP_..." }}' tokens are what pull values in from the
# environment. The -config flag is required (there is no default path), so the
# ENTRYPOINT passes the mounted path explicitly. This image ships no TLS
# material of its own, so it matches configs/config.toml's own defaults
# (plain HTTP on :8082, HTTPS disabled) rather than the Go-level
# defaultConfig()'s HTTPS-on-9743 — that default needs a mounted cert/key pair
# this image never provisions, so it cannot actually start unconfigured. A
# deployment that does mount its own TLS material and config.toml enabling
# [server.https] is fully supported by the binary; only this image's own
# EXPOSE/HEALTHCHECK below assume the common, out-of-the-box HTTP case.

EXPOSE 8082

# Reads the actually configured port at container start rather than a value
# fixed at build time, so a deployment that overrides
# APIP_ACP_SERVER_HTTP_PORT still gets a correct health probe.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f "http://localhost:${APIP_ACP_SERVER_HTTP_PORT:-8082}/healthz" || exit 1

STOPSIGNAL SIGTERM
ENTRYPOINT ["/usr/local/bin/bff", "-config", "/etc/api-control-plane/config.toml"]
