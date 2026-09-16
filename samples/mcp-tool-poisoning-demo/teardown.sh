#!/usr/bin/env bash
# Stops the evil-server and removes local temp files created by demo.sh.
# Does NOT touch anything you configured in WSO2 API Platform -- remove the
# MCP Proxy there manually if you no longer need it.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib.sh
source "$SCRIPT_DIR/scripts/lib.sh"

log_header "MCP Tool Poisoning Demo -- Teardown"

if docker container inspect "${CONTAINER_NAME}" >/dev/null 2>&1; then
  docker rm -f "${CONTAINER_NAME}" >/dev/null
  log_ok "Removed ${CONTAINER_NAME}"
else
  log_info "No evil-server container found -- nothing to remove."
fi

rm -f "$SCRIPT_DIR/.pass1-response.json" "$SCRIPT_DIR/.pass2-response.json"
log_ok "Cleaned up temporary files."

log_info "Note: this does NOT remove the MCP Proxy you created in WSO2 API Platform."
log_info "Remove that manually from the Publisher portal if you no longer need it."
