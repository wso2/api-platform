###########################
# Build Stage             #
###########################
FROM node:20-alpine AS build

RUN apk --no-cache add python3 build-base git

WORKDIR /app

# Separate dependency install for better layer caching
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts

COPY . .
RUN npm run build

###########################
# Runtime Stage           #
###########################
FROM nginx:1.27-alpine

ARG USERNAME=aiworkspace
ARG USER_UID=10001
ARG USER_GID=$USER_UID

RUN addgroup -g $USER_GID $USERNAME && \
    adduser -D -u $USER_UID -G $USERNAME $USERNAME && \
    apk upgrade --no-cache && \
    apk add --no-cache curl openssl && \
    mkdir -p /tmp/nginx/cache /tmp/nginx/logs /tmp/nginx/run && \
    ln -sf /tmp/nginx/cache /var/cache/nginx && \
    ln -sf /tmp/nginx/logs /var/log/nginx && \
    ln -sf /tmp/nginx/run /var/run/nginx && \
    touch /tmp/nginx/nginx.pid && \
    chown -R $USER_UID:$USER_GID /tmp/nginx && \
    chmod 666 /tmp/nginx/nginx.pid

WORKDIR /app

COPY --chown=$USERNAME:$USERNAME --from=build /app/dist /app/
COPY --chown=$USERNAME:$USERNAME nginx.docker.conf /etc/nginx/nginx.conf
COPY --chown=$USERNAME:$USERNAME entrypoint.sh /usr/local/bin/entrypoint.sh

RUN chown -R $USER_UID:$USER_GID /app && \
    chmod +x /usr/local/bin/entrypoint.sh

USER $USER_UID

EXPOSE 5380

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -fk https://localhost:5380/ || exit 1

STOPSIGNAL SIGQUIT
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
