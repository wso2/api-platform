#!/usr/bin/env bash
# Stops the local notification server and removes local log/PID files
# created by setup.sh and demo.sh.
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib.sh
source "$SCRIPT_DIR/scripts/lib.sh"

log_header "WebSocket Notification System -- Teardown"

if [[ -f "$SERVER_PID_FILE" ]]; then
  pid="$(cat "$SERVER_PID_FILE")"
  if kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null
    log_ok "Stopped notification server (PID $pid)"
  else
    log_info "No running notification server found for PID $pid"
  fi
  rm -f "$SERVER_PID_FILE"
else
  log_info "No notification server PID file found -- nothing to stop"
fi

# Kill any leftover subscriber client processes started by demo.sh, in case
# demo.sh was interrupted before its own cleanup ran.
pkill -f "client/subscriber.js" 2>/dev/null || true

rm -f "$SCRIPT_DIR"/.client-*.log "$SERVER_LOG_FILE"
log_ok "Cleaned up log and PID files."

log_info "Note: this does NOT remove any WebSocket API proxy you created in WSO2 API Platform Cloud."
log_info "Remove that manually from the console if you no longer need it."
