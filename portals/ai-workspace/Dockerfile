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

WORKDIR /src

# The BFF is a standalone, dependency-free module (stdlib only); build it without
# the workspace so the image is self-contained.
COPY bff/go.mod ./
COPY bff/ ./
ENV CGO_ENABLED=0 GOFLAGS=-mod=mod GOWORK=off
RUN go build -trimpath -ldflags "-s -w" -o /bff .

###########################
# Stage 3 — Runtime        #
###########################
FROM alpine:3.20

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

# BFF defaults: serve /app, self-signed TLS, listen on 5380.
ENV STATIC_DIR=/app \
    BFF_ADDR=:5380 \
    BFF_TLS_SELF_SIGNED=true \
    SESSION_STORE=memory \
    COOKIE_SECURE=true \
    COOKIE_SAMESITE=lax

EXPOSE 5380

# Probe HTTPS first (the default), then fall back to HTTP so the check also works
# when TLS is disabled (BFF_TLS_SELF_SIGNED=false and no cert mounted).
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -fk https://localhost:5380/healthz || curl -f http://localhost:5380/healthz || exit 1

STOPSIGNAL SIGTERM
ENTRYPOINT ["/usr/local/bin/bff"]
