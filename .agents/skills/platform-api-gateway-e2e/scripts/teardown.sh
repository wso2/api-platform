#!/usr/bin/env bash
# teardown.sh [--purge]
# Stops every gateway stack started by this skill (containers + volumes), request-info and
# platform-api. Keeps the temp work dir (logs, DB, gateway copies) unless --purge is given.
set -uo pipefail
. "$(dirname "$0")/lib.sh"

[ -n "$WORK" ] || log "no work dir recorded — only stopping listeners"
for d in ${WORK:+"$WORK"/gw-*}; do
  [ -f "$d/docker-compose.yaml" ] || continue
  ver=${d##*/gw-}
  log "stopping gateway $ver"
  ( cd "$d" && docker compose -p "$(gw_project "$ver")" down -v --remove-orphans >/dev/null 2>&1 ) \
    || log "  compose down failed for $ver (docker not running?)"
done

# stop_port PORT NAME RECORDED-PID: stop the listener on PORT only when it is the process this
# skill started (its pid was recorded in state at start). Anything else on the port is left alone.
stop_port() {
  local pid; pid=$(port_pid "$1")
  [ -n "$pid" ] || return 0
  if [ -n "$3" ] && [ "$pid" = "$3" ]; then
    kill "$pid" 2>/dev/null && log "stopped $2 (pid $pid)"
  else
    log "port $1 is held by pid $pid, which this skill did not start — leaving it alone"
  fi
}
state_load
stop_port "$BACKEND_PORT" request-info "${BACKEND_PID:-}"
stop_port "$PAPI_HTTP_PORT" platform-api "${PAPI_PID:-}"

if [ -n "$WORK" ]; then
  if [ "${1:-}" = "--purge" ]; then rm -rf "$WORK" "$WORK_POINTER"; log "removed $WORK"
  else log "kept $WORK (logs in $LOGS); remove with: teardown.sh --purge"; fi
fi
