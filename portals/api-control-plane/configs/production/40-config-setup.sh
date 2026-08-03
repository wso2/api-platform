#!/bin/sh
set -e

# Render the runtime config from environment variables at container startup.
# nginx's official entrypoint runs every /docker-entrypoint.d/*.sh before
# starting nginx. Only the listed variables are substituted so any other `$`
# in the template is left untouched.
TEMPLATE=/usr/share/nginx/html/api-platform.env.config.template.js
OUTPUT=/usr/share/nginx/html/api-platform.env.config.js

if [ -f "$TEMPLATE" ]; then
  echo "Rendering api-platform.env.config.js from environment variables..."
  envsubst '${APP_BASE_PATH} ${ENVIRONMENT_NAME} ${AUTH_BASE_URL} ${AUTH_CLIENT_ID} ${AUTH_SCOPES} ${THUNDER_ISSUER} ${PLATFORM_API_BASE_URL} ${BILLING_SERVICE_URL}' < "$TEMPLATE" > "$OUTPUT"
  echo "Runtime configuration generated: $OUTPUT"
else
  echo "Warning: $TEMPLATE not found, skipping runtime configuration"
fi
