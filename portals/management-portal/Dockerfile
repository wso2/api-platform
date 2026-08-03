# syntax=docker/dockerfile:1
# -----------------------------------------------------------------------------
# Production image for the API Platform management console.
#
# Self-contained multi-stage build (no monorepo tooling, no private registry):
# the SPA is built with a plain `npm ci && npm run build` and served by
# unprivileged nginx.
#
# Runtime configuration (IdP/BML URLs, client id, ...) is NOT baked in:
# api-platform.env.config.js is generated at container startup from environment
# variables via /docker-entrypoint.d/40-config-setup.sh (envsubst over
# api-platform.env.config.template.js). index.html loads it before React boots.
#
#   docker build -t management-portal .
# -----------------------------------------------------------------------------

# Stage 1: build the SPA.
FROM public.ecr.aws/docker/library/node:20.19-slim AS builder
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
# vite build → /app/build (build.outDir: 'build'); base path "/" for the console host.
RUN VITE_APP_BASE_PATH=/ npm run build

# Stage 2: serve with unprivileged nginx (uid 101, listens on 5173) — satisfies
# the OpenChoreo web-application ComponentType's runAsNonRoot security context,
# and matches the port the all-in-one docker-compose service expects.
FROM nginxinc/nginx-unprivileged:1.27-alpine
COPY configs/production/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder --chown=101:101 /app/build /usr/share/nginx/html
# Runtime config: template (envsubst'd at startup) + empty common config so
# index.html's <script> tags resolve.
COPY --chown=101:101 configs/production/api-platform.env.config.template.js /usr/share/nginx/html/api-platform.env.config.template.js
COPY --chown=101:101 configs/production/api-platform.common.config.js /usr/share/nginx/html/api-platform.common.config.js
COPY configs/production/40-config-setup.sh /docker-entrypoint.d/40-config-setup.sh
USER root
# The startup entrypoint (uid 101) renders api-platform.env.config.js into the
# web root via envsubst, so the html dir must be writable by 101.
RUN chmod +x /docker-entrypoint.d/40-config-setup.sh && \
    chown -R 101:101 /usr/share/nginx/html
USER 101
EXPOSE 5173
