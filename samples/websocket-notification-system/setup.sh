#!/usr/bin/env bash
# Installs dependencies and starts the local stock notification server in
# the background. See README.md for full context.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib.sh
source "$SCRIPT_DIR/scripts/lib.sh"

log_header "WebSocket Notification System -- Setup"

require_cmd node
require_cmd npm

if [[ ! -d "$SCRIPT_DIR/node_modules" ]]; then
  log_info "Installing dependencies..."
  (cd "$SCRIPT_DIR" && npm install --no-fund --no-audit --silent)
  log_ok "Dependencies installed"
else
  log_info "Dependencies already installed -- skipping npm install"
fi

if [[ -f "$SERVER_PID_FILE" ]] && kill -0 "$(cat "$SERVER_PID_FILE")" 2>/dev/null; then
  log_warn "Notification server already running (PID $(cat "$SERVER_PID_FILE")) -- leaving it as-is"
else
  log_info "Starting notification server on ws://localhost:${NOTIFICATION_PORT}..."
  : > "$SERVER_LOG_FILE"
  PORT="$NOTIFICATION_PORT" nohup node "$SCRIPT_DIR/server.js" > "$SERVER_LOG_FILE" 2>&1 &
  echo $! > "$SERVER_PID_FILE"
  disown

  log_info "Waiting for the server to become healthy..."
  healthy=false
  for _ in $(seq 1 20); do
    if grep -q "listening on port" "$SERVER_LOG_FILE" 2>/dev/null; then
      healthy=true
      break
    fi
    sleep 0.5
  done

  if [[ "$healthy" != "true" ]]; then
    log_err "Notification server did not become healthy in time. Check $SERVER_LOG_FILE."
    exit 1
  fi
  log_ok "Notification server is up (ws://localhost:${NOTIFICATION_PORT}), publishing a tick every 3 seconds"
fi

log_header "Next steps"
cat <<EOF
Run ${BOLD}./demo.sh${NC} to connect a few subscriber clients and watch them
receive identical broadcast ticks in real time.

When you're done, run ${BOLD}./teardown.sh${NC} to stop the server and clean up.
EOF
