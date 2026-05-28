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

# Runtime environment variable injection
echo "Generating runtime configuration from environment variables..."

# Start the runtime config file
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

# Close the config object
cat >> /tmp/runtime-config.js << 'EOF_FOOTER'
};

console.log('Runtime configuration loaded from environment variables');
console.log('Loaded', Object.keys(window.__RUNTIME_CONFIG__).length, 'configuration values');
EOF_FOOTER

chmod 644 /tmp/runtime-config.js

var_count=$(env | grep -c '^VITE_' || echo "0")
echo "Runtime configuration generated with $var_count VITE_* variables at /tmp/runtime-config.js"

# Start nginx in foreground
echo "Starting nginx..."
exec nginx -g "daemon off;"
