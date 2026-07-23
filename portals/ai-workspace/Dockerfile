###########################
# Stage 1 — SPA build      #
###########################
FROM node:20-alpine AS web

RUN apk --no-cache add python3 build-base git

WORKDIR /web

# Separate dependency install for better layer caching
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts

COPY . .
RUN npm run build

###########################
# Stage 2 — BFF build      #
###########################
FROM golang:1.26-alpine AS bff

# The BFF depends on the shared common module (configinterpolate) via a relative
# replace directive, so the module tree is reproduced under /src and common /
# httpkit are supplied as named build contexts:
#   docker buildx build --build-context common=../../common \
#                       --build-context httpkit=../../httpkit ...
# The workspace is disabled so the image builds from these copies alone.
WORKDIR /src/portals/ai-workspace/bff

# go.mod/go.sum first for better layer caching.
COPY --from=common go.mod go.sum /src/common/
COPY --from=httpkit go.mod go.sum /src/httpkit/
COPY bff/go.mod bff/go.sum ./
ENV CGO_ENABLED=0 GOWORK=off
RUN go mod download

COPY --from=common . /src/common
COPY --from=httpkit . /src/httpkit
COPY bff/ ./
RUN go build -trimpath -ldflags "-s -w" -o /bff .

###########################
# Stage 3 — Runtime        #
###########################
FROM alpine:3.24

ARG USERNAME=aiworkspace
ARG USER_UID=10001
ARG USER_GID=$USER_UID

RUN addgroup -g $USER_GID $USERNAME && \
    adduser -D -u $USER_UID -G $USERNAME $USERNAME && \
    apk upgrade --no-cache && \
    apk add --no-cache curl ca-certificates

WORKDIR /app

# Built SPA (served by the BFF as static files) and the BFF binary.
COPY --chown=$USER_UID:$USER_GID --from=web /web/dist /app
COPY --chown=$USER_UID:$USER_GID --from=bff /bff /usr/local/bin/bff

USER $USER_UID

# No ENV config defaults are baked in on purpose: the BFF's built-in defaults already
# match what this image needs (static dir /app, listener :5380, self-signed TLS,
# in-memory sessions, Secure/Lax cookies). Configure a deployment by mounting a
# config.toml at /etc/ai-workspace/config.toml; its '{{ env "APIP_AIW_..." }}' tokens
# are what pull values in from the environment.

EXPOSE 5380

# Probe HTTPS first (the default), then fall back to HTTP so the check also works
# when TLS is disabled (tls_self_signed = false and no cert mounted).
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -fk https://localhost:5380/healthz || curl -f http://localhost:5380/healthz || exit 1

STOPSIGNAL SIGTERM
ENTRYPOINT ["/usr/local/bin/bff"]
