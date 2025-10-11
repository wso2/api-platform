# STS Dockerfile - Phase 2: Gate App Build Integration
# Multi-stage build: Builder stage for Gate App + Thunder runtime

# ============================================
# Stage 1: Gate App Builder
# ============================================
FROM node:20-alpine AS gate-builder

# Install PNPM and OpenSSL
RUN npm install -g pnpm@10 && apk add --no-cache openssl

# Set working directory
WORKDIR /build

# Copy packages directory (oxygen-ui dependency - symlinked from Thunder)
COPY packages ./packages

# Set up pnpm workspace
RUN printf 'packages:\n  - "packages/*"\n' > pnpm-workspace.yaml

# Copy Gate App source
COPY gate-app/package.json gate-app/tsconfig.json gate-app/next.config.ts gate-app/server.js ./
COPY gate-app/src ./src
COPY gate-app/public ./public
COPY gate-app/scripts ./scripts

# Install dependencies
RUN pnpm install

# Generate SSL certificates
RUN pnpm ensure-certificates

# Build Gate App
RUN pnpm next_build

# Copy public and static assets into standalone
RUN cp -r public .next/standalone/ && \
    cp -r .next/static .next/standalone/.next/

# Copy server files
RUN cp server.js server.key server.cert .next/standalone/

# ============================================
# Stage 2: Thunder Runtime with Gate App
# ============================================
FROM ghcr.io/asgardeo/thunder:latest

# Metadata
LABEL maintainer="WSO2"
LABEL description="STS (Security Token Service) - OAuth 2.0 / OIDC server with authentication UI"
LABEL version="0.2.0"

# Switch to root to install Node.js
USER root

# Install Node.js 20 in Thunder image for running Gate App
RUN apk add --no-cache nodejs npm

# Switch back to thunder user
USER thunder

# Expose ports
EXPOSE 8090 9090

# Copy Gate App build from builder stage with proper ownership
COPY --from=gate-builder --chown=thunder:thunder /build/.next/standalone /opt/gate-app

# Copy startup script with execute permissions
COPY --chmod=755 scripts/startup.sh /opt/sts/startup.sh

# Set working directory
WORKDIR /opt/sts

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD curl -k -f https://localhost:8090/health/liveness || exit 1

# Start Thunder
CMD ["/opt/sts/startup.sh"]
