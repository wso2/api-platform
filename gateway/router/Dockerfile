FROM envoyproxy/envoy:v1.35.3

# Install envsubst (from gettext package)
USER root
RUN apt-get update && \
    apt-get install -y gettext-base && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Copy bootstrap configuration
COPY config/envoy-bootstrap.yaml /etc/envoy/envoy.yaml

# Copy config override template
COPY config/config-override.yaml /etc/envoy/config-override.yaml

# Copy entrypoint script
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Expose HTTP port and admin port
EXPOSE 8080 9901

# Set environment variables with default values
ENV XDS_SERVER_HOST=gateway-controller
ENV XDS_SERVER_PORT=18000

# Use entrypoint script
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
