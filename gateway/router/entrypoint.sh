#!/bin/sh
set -e

# Default XDS server configuration if not provided
export XDS_SERVER_HOST="${XDS_SERVER_HOST:-gateway-controller}"
export XDS_SERVER_PORT="${XDS_SERVER_PORT:-18000}"

echo "Starting Envoy with xDS server at ${XDS_SERVER_HOST}:${XDS_SERVER_PORT}"

# Generate config override by substituting environment variables
CONFIG_OVERRIDE=$(envsubst < /etc/envoy/config-override.yaml)

# Use --config-yaml to override the xDS cluster address
exec /usr/local/bin/envoy \
  -c /etc/envoy/envoy.yaml \
  --config-yaml "${CONFIG_OVERRIDE}" \
  --log-level info
