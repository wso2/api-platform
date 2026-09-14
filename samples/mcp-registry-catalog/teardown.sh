#!/usr/bin/env bash
# Stops the stack. By default, data volumes and generated secrets are kept
# so a plain `docker compose up` picks up where you left off.
#
# Usage: ./teardown.sh [--volumes]
#   --volumes   also remove data volumes, generated certs/keys, and .env /
#               api-platform.env -- a full reset back to a fresh checkout.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib.sh
source "$SCRIPT_DIR/scripts/lib.sh"

log_header "MCP Registry Catalog -- Teardown"

if [[ "${1:-}" == "--volumes" ]]; then
  log_info "Stopping stack and removing volumes..."
  (cd "$SCRIPT_DIR" && docker compose down -v)
  rm -rf "$SCRIPT_DIR/resources/certificates" "$SCRIPT_DIR/resources/keys" \
         "$SCRIPT_DIR/resources/gateway-certificates" "$SCRIPT_DIR/resources/gateway-listener-certs" \
         "$SCRIPT_DIR/resources/gateway-aesgcm-keys"
  rm -f "$SCRIPT_DIR/.env" "$SCRIPT_DIR/api-platform.env"
  log_ok "Removed containers, volumes, generated secrets, and .env files."
else
  log_info "Stopping stack (data volumes kept -- pass --volumes for a full reset)..."
  (cd "$SCRIPT_DIR" && docker compose down)
  log_ok "Stack stopped."
fi
