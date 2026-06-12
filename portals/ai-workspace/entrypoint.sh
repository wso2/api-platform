#!/bin/sh
set -e

echo "Starting AI Workspace with runtime configuration..."

# Ensure runtime directories exist and have correct permissions
mkdir -p /tmp
# Create nginx runtime dirs under /tmp so non-root user can write
mkdir -p /tmp/nginx/cache /tmp/nginx/logs /tmp/nginx/run || true

# Ensure nginx pid exists and is writable
touch /tmp/nginx/nginx.pid
chmod 666 /tmp/nginx/nginx.pid || true

# Try to symlink /var paths to /tmp ones when possible (won't overwrite read-only mounts)
if [ ! -e /var/cache/nginx ] || [ -L /var/cache/nginx ]; then
  ln -sf /tmp/nginx/cache /var/cache/nginx || true
fi
if [ ! -e /var/log/nginx ] || [ -L /var/log/nginx ]; then
  ln -sf /tmp/nginx/logs /var/log/nginx || true
fi
if [ ! -e /var/run/nginx ] || [ -L /var/run/nginx ]; then
  ln -sf /tmp/nginx/run /var/run/nginx || true
fi

# ---------------------------------------------------------------------------
# config.toml injection
# Read [ai_workspace] section from a mounted config.toml and set VITE_* env
# vars. Environment variables already set in the container take priority —
# they are never overwritten by the config file.
# ---------------------------------------------------------------------------
CONFIG_FILE="/etc/ai-workspace/config.toml"
if [ -f "$CONFIG_FILE" ]; then
  echo "Loading configuration from $CONFIG_FILE ..."

  in_section=0
  while IFS= read -r line; do
    # Detect section header
    case "$line" in
      '[ai_workspace]') in_section=1; continue ;;
      '['*']')          in_section=0; continue ;;
    esac
    [ "$in_section" -eq 0 ] && continue

    # Skip blank lines and comments
    case "$line" in
      ''|'#'*|' #'*) continue ;;
    esac

    # Split on first '='
    key=$(echo "$line" | sed 's/[[:space:]]*=.*//' | tr -d '[:space:]')
    value=$(echo "$line" | sed 's/^[^=]*=[[:space:]]*//' | sed 's/^"//' | sed 's/"[[:space:]]*$//')

    # Map config.toml key → VITE_* env var name
    case "$key" in
      auth_mode)                  vite_key="VITE_AUTH_MODE" ;;
      domain)                     vite_key="VITE_DOMAIN" ;;
oidc_authority)             vite_key="VITE_OIDC_AUTHORITY" ;;
      oidc_client_id)             vite_key="VITE_OIDC_CLIENT_ID" ;;
      default_org_region)         vite_key="VITE_DEFAULT_ORG_REGION" ;;
      platform_api_base_url)      vite_key="VITE_PLATFORM_API_BASE_URL" ;;
      controlplane_host)          vite_key="VITE_CONTROLPLANE_HOST" ;;
      *)                          continue ;;
    esac

    # Only set if the env var is not already set (env vars take priority)
    if ! env | grep -q "^${vite_key}="; then
      export "${vite_key}=${value}"
      echo "  ${vite_key} set from config.toml"
    fi
  done < "$CONFIG_FILE"
fi

# ---------------------------------------------------------------------------
# Runtime environment variable injection into the SPA
# ---------------------------------------------------------------------------
echo "Generating runtime configuration from environment variables..."

cat > /tmp/runtime-config.js << 'EOF_HEADER'
// Runtime Configuration - Generated from environment variables
// This file is dynamically created at container startup
// Auto-generated from all VITE_* environment variables
window.__RUNTIME_CONFIG__ = {
EOF_HEADER

# Get all environment variables that start with VITE_ and add them to the config
env | grep '^VITE_' | while IFS='=' read -r key value; do
  # Escape single quotes in the value
  escaped_value=$(echo "$value" | sed "s/'/\\\\'/g")
  # Write the key-value pair to the config file
  echo "  $key: '$escaped_value'," >> /tmp/runtime-config.js
done

cat >> /tmp/runtime-config.js << 'EOF_FOOTER'
};

console.log('Runtime configuration loaded from environment variables');
console.log('Loaded', Object.keys(window.__RUNTIME_CONFIG__).length, 'configuration values');
EOF_FOOTER

chmod 644 /tmp/runtime-config.js

var_count=$(env | grep -c '^VITE_' || echo "0")
echo "Runtime configuration generated with $var_count VITE_* variables at /tmp/runtime-config.js"

# ---------------------------------------------------------------------------
# TLS — use a user-provided certificate if mounted, otherwise self-signed.
# Mount your cert/key at /etc/ai-workspace/tls/tls.crt and tls.key to avoid
# the browser trust warning (see docker-compose.yaml for the volume example).
# ---------------------------------------------------------------------------
USER_CERT="/etc/ai-workspace/tls/tls.crt"
USER_KEY="/etc/ai-workspace/tls/tls.key"

if [ -f "$USER_CERT" ] && [ -f "$USER_KEY" ]; then
  echo "Using user-provided TLS certificate from $USER_CERT"
  cp "$USER_CERT" /tmp/nginx/tls.crt
  cp "$USER_KEY"  /tmp/nginx/tls.key
  chmod 600 /tmp/nginx/tls.key
else
  echo "No certificate found at $USER_CERT — generating self-signed certificate..."
  openssl req -x509 -nodes -newkey rsa:2048 -days 3650 \
    -keyout /tmp/nginx/tls.key \
    -out    /tmp/nginx/tls.crt \
    -subj   "/CN=localhost" 2>/dev/null
  chmod 600 /tmp/nginx/tls.key
  echo "Self-signed certificate generated (browsers will show a trust warning)"
fi

# Start nginx in foreground
echo "Starting nginx..."
exec nginx -g "daemon off;"
