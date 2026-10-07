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

stop_port() { # port name probe-url
  local pid; pid=$(port_pid "$1")
  [ -n "$pid" ] || return 0
  if curl -sf "$3" >/dev/null 2>&1; then
    kill "$pid" 2>/dev/null && log "stopped $2 (pid $pid)"
  else
    log "port $1 is held by pid $pid, which does not answer like $2 — leaving it alone"
  fi
}
stop_port "$BACKEND_PORT" request-info "http://127.0.0.1:${BACKEND_PORT}/healthz"
stop_port "$PAPI_HTTP_PORT" platform-api "${PAPI_URL}/health"

if [ -n "$WORK" ]; then
  if [ "${1:-}" = "--purge" ]; then rm -rf "$WORK" "$WORK_POINTER"; log "removed $WORK"
  else log "kept $WORK (logs in $LOGS); remove with: teardown.sh --purge"; fi
fi
