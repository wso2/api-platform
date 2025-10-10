# STS Dockerfile - Phase 1: Basic Thunder Integration
# This Dockerfile packages Thunder OAuth 2.0 / OIDC server

FROM ghcr.io/asgardeo/thunder:latest

# Metadata
LABEL maintainer="WSO2"
LABEL description="STS (Security Token Service) - OAuth 2.0 / OIDC server with authentication UI"
LABEL version="0.1.0"

# Expose Thunder port
EXPOSE 8090

# Copy startup script with execute permissions
COPY --chmod=755 scripts/startup.sh /opt/sts/startup.sh

# Set working directory
WORKDIR /opt/sts

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD curl -k -f https://localhost:8090/health/liveness || exit 1

# Start Thunder
CMD ["/opt/sts/startup.sh"]
