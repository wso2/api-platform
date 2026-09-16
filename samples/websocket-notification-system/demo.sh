#!/usr/bin/env bash
# Connects subscriber clients to the local stock notification server and
# watches them receive identical broadcast ticks in real time. See
# README.md for the full walkthrough.
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib.sh
source "$SCRIPT_DIR/scripts/lib.sh"

log_header "WebSocket Notification System -- Demo"

if [[ ! -f "$SERVER_PID_FILE" ]] || ! kill -0 "$(cat "$SERVER_PID_FILE")" 2>/dev/null; then
  log_err "Notification server isn't running. Run ./setup.sh first."
  exit 1
fi

CLIENT_PIDS=()
CLIENT_NAMES=(dashboard-1 dashboard-2 dashboard-3)

start_client() {
  local name="$1"
  local log_file="$SCRIPT_DIR/.client-${name}.log"
  : > "$log_file"
  SERVER_PORT="$NOTIFICATION_PORT" node "$SCRIPT_DIR/client/subscriber.js" "$name" >> "$log_file" 2>&1 &
  CLIENT_PIDS+=("$!")
}

stop_client_by_index() {
  local idx="$1"
  local pid="${CLIENT_PIDS[$idx]}"
  if kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null
    wait "$pid" 2>/dev/null
  fi
}

cleanup_clients() {
  for pid in "${CLIENT_PIDS[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
  wait 2>/dev/null || true
}
trap cleanup_clients EXIT

log_info "Connecting 3 subscriber clients: ${CLIENT_NAMES[*]}..."
for name in "${CLIENT_NAMES[@]}"; do
  start_client "$name"
done
sleep 1
log_ok "All clients connected. The server publishes a tick every 3 seconds."

log_header "Step 1: Every connected client receives the same tick"
sleep 4
for name in "${CLIENT_NAMES[@]}"; do
  log_info "$(tail -n 1 "$SCRIPT_DIR/.client-${name}.log")"
done

log_header "Step 2: Disconnect dashboard-2, then wait for the next tick"
stop_client_by_index 1
sleep 3.5
log_info "$(tail -n 1 "$SCRIPT_DIR/.client-dashboard-1.log")"
log_warn "dashboard-2 is disconnected -- it won't receive this tick"
log_info "$(tail -n 1 "$SCRIPT_DIR/.client-dashboard-3.log")"

log_header "Step 3: Connect a late-joining client, then wait for the next tick"
start_client "dashboard-4"
sleep 3.5
log_info "$(tail -n 1 "$SCRIPT_DIR/.client-dashboard-4.log")"
log_warn "dashboard-4 only has one line in its log -- it missed every tick published before it connected"

log_header "Demo complete"
cat <<EOF
Every client that was connected received the identical tick at the same
moment, and only while it was connected. Run ${BOLD}./teardown.sh${NC} to stop the
server and clean up log files.
EOF
