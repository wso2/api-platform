#!/usr/bin/env bash
# Install request-info (echo backend, https://github.com/renuka-fernando/request-info) with
# go install — straight from GitHub, no local checkout — into the work dir and run it on
# 127.0.0.1:$BACKEND_PORT; check a container can reach it through host.docker.internal.
# -read-envs=false: its default echoes the whole process environment in every response.
set -euo pipefail
. "$(dirname "$0")/lib.sh"
need_work

if [ -n "$(port_pid "$BACKEND_PORT")" ]; then
  curl -sf "http://127.0.0.1:${BACKEND_PORT}/healthz" >/dev/null || die "port $BACKEND_PORT busy with something that is not request-info (set BACKEND_PORT)"
  log "request-info already running on $BACKEND_PORT — reusing"
else
  BDIR="$WORK/backend"; mkdir -p "$BDIR"
  GOWORK=off GOBIN="$BDIR" GOFLAGS= go install "$REQUEST_INFO_MODULE" > "$LOGS/backend-build.log" 2>&1 \
    || { cat "$LOGS/backend-build.log"; die "go install $REQUEST_INFO_MODULE failed"; }
  # Run from $BDIR: request-info writes ./last_response on every request.
  ( cd "$BDIR" && exec ./request-info -addr "127.0.0.1:${BACKEND_PORT}" -read-envs=false -logH ) \
    > "$LOGS/backend.log" 2>&1 < /dev/null &
  state_set BACKEND_PID "$!"   # teardown stops only this pid
  wait_for 30 "request-info" curl -sf "http://127.0.0.1:${BACKEND_PORT}/healthz" || die "request-info did not start"
fi

if docker info >/dev/null 2>&1; then
  docker run --rm --add-host host.docker.internal:host-gateway curlimages/curl:8.10.1 \
    -sf -m 5 "${BACKEND_URL_FROM_GW}/healthz" >/dev/null 2>&1 \
    || die "containers cannot reach $BACKEND_URL_FROM_GW (see SKILL.md troubleshooting)"
  log "backend reachable from containers at $BACKEND_URL_FROM_GW"
fi
