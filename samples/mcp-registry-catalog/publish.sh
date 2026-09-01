#!/usr/bin/env bash
# Pushes both sample MCP proxies (mcp-servers/weather-mcp,
# mcp-servers/inventory-mcp) to the API Portal via the Platform API's
# Management API: POST /api/v0.9/mcp-servers (falls back to PUT if the
# server was already published by an earlier run -- safe to re-run).
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib.sh
source "$SCRIPT_DIR/scripts/lib.sh"

require_cmd curl
require_cmd jq

if [[ ! -f "$SCRIPT_DIR/.env" ]]; then
  log_err ".env not found. Run ./setup.sh first."
  exit 1
fi
# shellcheck source=.env
source "$SCRIPT_DIR/.env"

log_header "Publishing sample MCP servers to the portal"

log_info "Logging in to the Management API at ${PLATFORM_API_URL}..."
TOKEN="$(platform_api_login "$ADMIN_USERNAME" "$ADMIN_PASSWORD")" || exit 1
log_ok "Authenticated as ${ADMIN_USERNAME}"

publish_one() {
  local dir="$1"
  local mcp_id
  mcp_id="$(grep -A1 '^metadata:' "$dir/mcp.yaml" | sed -n 's/^ *name: *//p' | tr -d '\r')"

  log_info "Publishing ${mcp_id}..."
  local response status
  response=$(curl -sSk -w '\n%{http_code}' -X POST "${API_PORTAL_BASE_URL}/api/v0.9/mcp-servers" \
    -H "Authorization: Bearer ${TOKEN}" \
    -F "metadata=@${dir}/mcp.yaml" \
    -F "definition=@${dir}/schemaDefinition.yaml;type=application/yaml")
  status="${response##*$'\n'}"
  response="${response%$'\n'*}"

  if [[ "$status" == "201" ]]; then
    log_ok "  created (${mcp_id})"
  elif [[ "$status" == "409" ]]; then
    log_info "  already published -- updating"
    response=$(curl -sSk -w '\n%{http_code}' -X PUT "${API_PORTAL_BASE_URL}/api/v0.9/mcp-servers/${mcp_id}" \
      -H "Authorization: Bearer ${TOKEN}" \
      -F "metadata=@${dir}/mcp.yaml" \
      -F "definition=@${dir}/schemaDefinition.yaml;type=application/yaml")
    status="${response##*$'\n'}"
    response="${response%$'\n'*}"
    if [[ "$status" == "200" ]]; then
      log_ok "  updated (${mcp_id})"
    else
      log_err "  update failed (HTTP ${status}): ${response}"
      return 1
    fi
  else
    log_err "  publish failed (HTTP ${status}): ${response}"
    return 1
  fi
}

publish_one "$SCRIPT_DIR/mcp-servers/weather-mcp"
publish_one "$SCRIPT_DIR/mcp-servers/inventory-mcp"

log_header "Done"
cat <<EOF
Both MCP servers are now published. See them:

  Portal UI:      ${BOLD}${API_PORTAL_BASE_URL}/default/views/default${NC}
  Registry API:   ${BOLD}./discover.sh${NC}
EOF
