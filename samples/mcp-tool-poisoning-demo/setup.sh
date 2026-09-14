#!/usr/bin/env bash
# Starts the evil-server (a WireMock instance serving a poisoned MCP tool
# listing) and prepares a local .env file. See README.md for full context.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib.sh
source "$SCRIPT_DIR/scripts/lib.sh"

log_header "MCP Tool Poisoning Demo -- Setup"

require_cmd docker
require_cmd curl
require_cmd jq

if [[ ! -f "$SCRIPT_DIR/.env" ]]; then
  cp "$SCRIPT_DIR/.env.example" "$SCRIPT_DIR/.env"
  log_ok "Created .env from .env.example"
else
  log_info ".env already exists -- leaving it as-is"
fi

if docker container inspect "${CONTAINER_NAME}" >/dev/null 2>&1; then
  log_warn "Container ${CONTAINER_NAME} already exists -- removing it first"
  docker rm -f "${CONTAINER_NAME}" >/dev/null
fi

log_info "Starting evil-server (WireMock) on port ${EVIL_SERVER_PORT}..."
docker run -d \
  --name "${CONTAINER_NAME}" \
  -p "${EVIL_SERVER_PORT}:8080" \
  -v "$SCRIPT_DIR/evil-server/mappings:/home/wiremock/mappings:ro" \
  wiremock/wiremock:3.9.2 \
  --port 8080 >/dev/null

log_info "Waiting for evil-server to become healthy..."
healthy=false
for _ in $(seq 1 30); do
  if curl -sS -o /dev/null "http://localhost:${EVIL_SERVER_PORT}/__admin/mappings" 2>/dev/null; then
    healthy=true
    break
  fi
  sleep 1
done

if [[ "$healthy" != "true" ]]; then
  log_err "evil-server did not become healthy in time. Check 'docker logs ${CONTAINER_NAME}'."
  exit 1
fi

log_ok "evil-server is up at ${EVIL_SERVER_URL}"

log_header "Next steps"
cat <<EOF
1. Run ${BOLD}./demo.sh pass1${NC} now to see the poisoned tool with no gateway policy
   in front of it.

2. Then follow "Configure the gateway" in README.md to create an MCP Proxy
   in AI Workspace pointing at:

     ${BOLD}${EVIL_SERVER_URL}${NC}

   and attach the MCP Access Control policy (tools.mode=deny,
   tools.exceptions=[list_open_invoices]).

3. Put the deployed proxy's invoke URL into .env as GATEWAY_MCP_URL (and
   GATEWAY_API_KEY if required), then run ${BOLD}./demo.sh pass2${NC} to see the
   poisoned tool disappear.
EOF
