#!/bin/sh
set -eu

# Initialise tput colors if available
if command -v tput >/dev/null 2>&1 && [ -n "${TERM:-}" ] && tput setaf 2 >/dev/null 2>&1; then
  GREEN="$(tput setaf 2)"; RESET="$(tput sgr0)"
else
  GREEN=""; RESET=""
fi

print_ok()   { echo "${GREEN}✔  $1${RESET}"; }
print_info() { echo "-->  $1"; }
print_title() { echo ""; echo "${GREEN}=== $1 ===${RESET}"; echo ""; }

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

print_title "Teardown"

print_info "Removing mock MCP backend container..."
docker rm -f mock-mcp-reading-list 2>/dev/null || true
print_ok "mock-mcp-reading-list removed"

BUNDLE_DIR="${SCRIPT_DIR}/wso2apip-ai-gateway-1.1.0"
if [ -d "$BUNDLE_DIR" ]; then
  print_info "Stopping WSO2 AI Gateway stack..."
  cd "$BUNDLE_DIR" && docker compose down -v
  print_ok "Teardown complete."
else
  print_info "Bundle directory not found at ${BUNDLE_DIR} — skipping docker compose down."
fi
