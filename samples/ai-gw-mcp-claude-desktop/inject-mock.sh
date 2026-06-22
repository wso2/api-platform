#!/bin/sh
set -eu

# Initialise tput colors if available
if command -v tput >/dev/null 2>&1 && [ -n "${TERM:-}" ] && tput setaf 2 >/dev/null 2>&1; then
  GREEN="$(tput setaf 2)"; RESET="$(tput sgr0)"
else
  GREEN=""; RESET=""
fi

print_ok() {
  echo "${GREEN}✔  $1${RESET}"
}

print_info() {
  echo "-->  $1"
}

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ -f "${SCRIPT_DIR}/.env" ]; then
  . "${SCRIPT_DIR}/.env"
fi

MGMT_PORT="${MGMT_PORT:-9090}"
MGMT_BASE="http://localhost:${MGMT_PORT}/api/management/v0.9"
AUTH="Authorization: Basic YWRtaW46YWRtaW4="

print_info "Registering MCP Proxy (context: /reading-list, policy: mcp-auth)..."
curl -sf -X POST "${MGMT_BASE}/mcp-proxies" \
  -H "Content-Type: application/yaml" \
  -H "${AUTH}" \
  --data-binary @"${SCRIPT_DIR}/mcp.yaml"
echo ""
print_ok "MCP Proxy registered"
