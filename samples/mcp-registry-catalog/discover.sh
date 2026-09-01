#!/usr/bin/env bash
# Queries the API Portal's MCP Registry API (a public, unauthenticated read
# API modeled on the official MCP registry schema) and prints the catalog.
#
# Endpoint: GET /registry/{orgHandle}/v0.1/servers
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib.sh
source "$SCRIPT_DIR/scripts/lib.sh"

require_cmd curl
require_cmd jq

REGISTRY_URL="${API_PORTAL_BASE_URL}/registry/${ORG_HANDLE}/v0.1/servers"

log_header "MCP Registry -- ${REGISTRY_URL}"

RESPONSE="$(curl -sSk "$REGISTRY_URL")"
COUNT="$(printf '%s' "$RESPONSE" | jq -r '.metadata.count // 0')"

printf '%s' "$RESPONSE" | jq .

echo
if [[ "$COUNT" -gt 0 ]]; then
  log_ok "${COUNT} MCP server(s) in the catalog:"
  printf '%s' "$RESPONSE" | jq -r '.servers[] | "  - \(.server.name) v\(.server.version) -> \(.server.remotes[0].url // "n/a")"'
else
  log_warn "No MCP servers found. Did you run ./publish.sh?"
  exit 1
fi
