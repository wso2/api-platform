###########################
# Node Application Build  #
###########################
FROM node:20-alpine AS build

# install python and other build tools for native deps (node-gyp)
RUN apk --no-cache add python3 build-base git

WORKDIR /app

# Use pinned, known-good cli versions (matches other apps in repo)
RUN npm install -g pnpm@10.6.5 && \
    npm install -g @microsoft/rush@5.149.0

# Copy the repository and install dependencies for the ai-workspace subspace
COPY . /app

RUN printf "packages:\n  - 'workspaces/apps/ai-workspace'\n" > /app/pnpm-workspace.yaml

# Install only the workspace packages required by the ai-workspace project using pnpm
RUN pnpm -w install --filter "@wso2-enterprise/ai-workspace..." --no-prefer-frozen-lockfile

# Build only the ai-workspace project using pnpm filtered run
WORKDIR /app
RUN pnpm -w --filter "@wso2-enterprise/ai-workspace..." run build

########################
# Final Static Server   #
########################
FROM nginx:alpine

ARG USERNAME=aiworkspace
ARG USER_UID=10001
ARG USER_GID=$USER_UID
ENV USERNAME=$USERNAME
ENV USER_UID=$USER_UID
ENV USER_GID=$USER_GID

WORKDIR /app
RUN mkdir -p /tmp && mkdir -p /home/$USERNAME && \
    addgroup -g $USER_GID $USERNAME && \
    adduser -D -u $USER_UID -G $USERNAME $USERNAME && \
    chown -R $USER_UID:$USER_GID /home/$USERNAME && \
    apk upgrade --no-cache && \
    apk add --no-cache xz-libs libcrypto3 libssl3 libxml2 \
    curl libcurl libexpat libxslt c-ares && \
    chown -R $USER_UID:$USER_GID /app && chmod -R 755 /app && \
    chown -R $USER_UID:$USER_GID /tmp && \
    mkdir -p /tmp/nginx/cache /tmp/nginx/logs /tmp/nginx/run && \
    ln -sf /tmp/nginx/cache /var/cache/nginx && \
    ln -sf /tmp/nginx/logs /var/log/nginx && \
    ln -sf /tmp/nginx/run /var/run/nginx && \
    chown -R $USER_UID:$USER_GID /tmp/nginx && \
    touch /tmp/nginx/nginx.pid && \
    chown $USER_UID:$USER_GID /tmp/nginx/nginx.pid && \
    chmod 666 /tmp/nginx/nginx.pid

USER 10001

# Copy configuration files and build artifacts
COPY --chown=aiworkspace:aiworkspace --from=build /app/workspaces/apps/ai-workspace/production/nginx.conf /etc/nginx/
COPY --chown=aiworkspace:aiworkspace --from=build /app/workspaces/apps/ai-workspace/dist /app/
COPY --chown=aiworkspace:aiworkspace --from=build /app/workspaces/apps/ai-workspace/production/entrypoint.sh /usr/local/bin/
# Ensure the entrypoint script is executable
RUN chmod +x /usr/local/bin/entrypoint.sh

EXPOSE 8080

STOPSIGNAL SIGQUIT
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
