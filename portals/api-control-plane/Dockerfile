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
FROM node:24-alpine AS web
WORKDIR /app

# Separate dependency install for better layer caching.
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts

COPY . .
RUN VITE_APP_BASE_PATH=/ npm run build

###########################
# Stage 2 — BFF build     #
###########################
FROM golang:1.26-alpine AS bff
WORKDIR /src

# go.mod/go.sum first for better layer caching.
COPY bff/go.mod bff/go.sum ./
ENV CGO_ENABLED=0 GOWORK=off
RUN go mod download

COPY bff/ ./
RUN go build -trimpath -ldflags "-s -w" -o /bff .

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

# No ENV config defaults are baked in on purpose: the BFF's built-in defaults
# already match what this image needs (static dir /app, listener :9743,
# in-memory sessions, Secure/Lax cookies). Configure a deployment by mounting
# a config.toml at /etc/api-control-plane/config.toml; its
# '{{ env "APIP_ACP_..." }}' tokens are what pull values in from the
# environment. The -config flag is required (there is no default path), so the
# ENTRYPOINT passes the mounted path explicitly.

EXPOSE 9743 9780

# Each protocol is probed on its own configured port (9743 https, 9780 http) —
# unlike a fixed single port, this stays correct whichever listener(s) a given
# deployment actually enables.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -fk https://localhost:9743/healthz || curl -f http://localhost:9780/healthz || exit 1

STOPSIGNAL SIGTERM
ENTRYPOINT ["/usr/local/bin/bff", "-config", "/etc/api-control-plane/config.toml"]
