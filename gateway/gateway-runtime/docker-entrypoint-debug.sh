#!/bin/bash

# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

# Gateway Runtime Debug Entrypoint Script
# Variant of docker-entrypoint.sh that wraps the policy engine in dlv for
# remote debugging via VS Code (port 2346).

# NOTE: Mostly duplicates docker-entrypoint.sh — keep in sync.

# Process-specific args can be passed using prefixed flags:
#   --rtr.<flag> <value>   → forwarded to Router (Envoy)
#   --pol.<flag> <value>   → forwarded to Policy Engine
#
# Examples:
#   docker run gateway-runtime-debug --rtr.component-log-level upstream:debug --pol.log-format text

set -e

# Logging function for entrypoint messages
log() {
    echo "[ent] $(date '+%Y-%m-%d %H:%M:%S') $1"
}

# Parse process-specific args from command line.
# Uses dot (.) as the prefix separator (e.g. --rtr.flag, --pol.flag) because no
# standard CLI flag contains a dot, making prefix detection unambiguous.
# --rtr.X → ROUTER_ARGS, --pol.X → PE_ARGS, unrecognized → warning
ROUTER_ARGS=()
PE_ARGS=()

while [[ $# -gt 0 ]]; do
    case "$1" in
        --rtr.*)
            ROUTER_ARGS+=("--${1#--rtr.}")
            shift
            # Consume the value if next arg is not a flag
            if [[ $# -gt 0 && "$1" != --* ]]; then
                ROUTER_ARGS+=("$1")
                shift
            fi
            ;;
        --pol.*)
            PE_ARGS+=("--${1#--pol.}")
            shift
            if [[ $# -gt 0 && "$1" != --* ]]; then
                PE_ARGS+=("$1")
                shift
            fi
            ;;
        *)
            log "ERROR: Unrecognized arg '$1' (use --rtr. or --pol. prefix)"
            exit 1
            ;;
    esac
done

# Default configuration
# GATEWAY_CONTROLLER_HOST is the primary user-facing env var to configure connectivity
# to the gateway controller. The xDS ports default to well-known values:
#   - ROUTER_XDS_PORT (18000): Router (Envoy) route/cluster/listener configs
#   - POLICY_ENGINE_XDS_PORT (18001): Policy Engine policy chain configs
export GATEWAY_CONTROLLER_HOST="${GATEWAY_CONTROLLER_HOST:-gateway-controller}"
export ROUTER_XDS_PORT="${ROUTER_XDS_PORT:-18000}"
export POLICY_ENGINE_XDS_PORT="${POLICY_ENGINE_XDS_PORT:-18001}"
export LOG_LEVEL="${LOG_LEVEL:-info}"

# Performance tuning configuration
# GOMAXPROCS limits Go's CPU usage - set to leave cores for Envoy (default: 2)
# ROUTER_CONCURRENCY sets Envoy's worker thread count (default: auto-detect, 0 means use all cores)
# APIP_GW_POLICY_ENGINE_METRICS_ENABLED controls metrics collection (default: true, set false for high-load)
# APIP_GW_ROUTER_RE2_MAX_PROGRAM_SIZE sets Envoy's RE2 regex program-size error cap (default: 400).
#   Envoy's built-in default is 100, which is too low for paths with many path parameters
#   (each [^/]+ segment costs ~10 units; 7 params → program size 115 → RouteConfiguration rejected).
#   400 covers paths with up to ~30 path parameters. Override if your API has deeper paths.
export GOMAXPROCS="${GOMAXPROCS:-2}"
export ROUTER_CONCURRENCY="${ROUTER_CONCURRENCY:-0}"
export APIP_GW_POLICY_ENGINE_METRICS_ENABLED="${APIP_GW_POLICY_ENGINE_METRICS_ENABLED:-true}"
export APIP_GW_ROUTER_RE2_MAX_PROGRAM_SIZE="${APIP_GW_ROUTER_RE2_MAX_PROGRAM_SIZE:-400}"

# Graceful shutdown configuration (see docker-entrypoint.sh for details).
# On SIGTERM the Router (Envoy) is drained before processes are terminated so in-flight
# requests finish and keep-alive connections close cleanly instead of being reset.
# Keep ROUTER_DRAIN_TIME_SECONDS < the pod terminationGracePeriodSeconds; 0 disables it.
export ROUTER_ADMIN_HOST="${ROUTER_ADMIN_HOST:-127.0.0.1}"
export ROUTER_ADMIN_PORT="${ROUTER_ADMIN_PORT:-9901}"
export ROUTER_DRAIN_TIME_SECONDS="${ROUTER_DRAIN_TIME_SECONDS:-15}"

# Derive Router (Envoy) xDS config — used by envsubst on config-override.yaml
export XDS_SERVER_HOST="${GATEWAY_CONTROLLER_HOST}"
export XDS_SERVER_PORT="${ROUTER_XDS_PORT}"

# Policy Engine xDS address
PE_XDS_SERVER="${GATEWAY_CONTROLLER_HOST}:${POLICY_ENGINE_XDS_PORT}"

POLICY_ENGINE_SOCKET="/var/run/api-platform/policy-engine.sock"

log "Starting Gateway Runtime (DEBUG mode — dlv on port 2346)"
log "  Gateway Controller: ${GATEWAY_CONTROLLER_HOST}"
log "  Router xDS: ${GATEWAY_CONTROLLER_HOST}:${ROUTER_XDS_PORT}"
log "  Policy Engine xDS: ${PE_XDS_SERVER}"
log "  Log Level: ${LOG_LEVEL}"
log "  Policy Engine Socket: ${POLICY_ENGINE_SOCKET}"
log "  GOMAXPROCS: ${GOMAXPROCS}"
log "  Router Concurrency: ${ROUTER_CONCURRENCY}"
log "  Router RE2 Max Program Size: ${APIP_GW_ROUTER_RE2_MAX_PROGRAM_SIZE}"
log "  Policy Engine Metrics: ${APIP_GW_POLICY_ENGINE_METRICS_ENABLED}"
[[ ${#ROUTER_ARGS[@]} -gt 0 ]] && log "  Router extra args: ${ROUTER_ARGS[*]}"
[[ ${#PE_ARGS[@]} -gt 0 ]] && log "  Policy Engine extra args: ${PE_ARGS[*]}"

# Cleanup stale socket from previous runs
rm -f "${POLICY_ENGINE_SOCKET}"

# Generate Envoy config override by substituting environment variables
CONFIG_OVERRIDE=$(envsubst < /etc/envoy/config-override.yaml)

# Track child PIDs
PE_PID=""
ENVOY_PID=""

# Gracefully drain the Router (Envoy) listeners via its admin API so in-flight requests
# complete and keep-alive connections close cleanly (Connection: close) rather than being
# reset. No curl/wget in the image, so use a bash /dev/tcp socket. Best-effort.
drain_router() {
    local host="${ROUTER_ADMIN_HOST}" port="${ROUTER_ADMIN_PORT}"
    if ! { exec 3<>"/dev/tcp/${host}/${port}"; } 2>/dev/null; then
        log "  WARN: Router admin ${host}:${port} unreachable; skipping graceful drain"
        return 1
    fi
    printf 'POST /drain_listeners?graceful HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n' "$host" >&3 2>/dev/null || true
    while IFS= read -r -t 3 -u 3 _line 2>/dev/null; do :; done
    exec 3<&- 3>&- 2>/dev/null || true
    return 0
}

# SIGTERM a tracked process and wait for it to fully exit before returning.
stop_proc() {
    local pid_var="$1" label="$2" pid
    pid="${!pid_var}"
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
        log "Stopping ${label} (PID $pid)..."
        kill -TERM "$pid" 2>/dev/null || true
        wait "$pid" 2>/dev/null || true
        log "${label} exited"
    fi
}

# Shutdown handler - gracefully drain the Router, then terminate both processes
shutdown() {
    log "Received shutdown signal..."

    # Drain the Router first so in-flight requests finish and keep-alive connections are
    # closed cleanly — prevents client-visible connection resets during rolling restarts.
    if [ -n "$ENVOY_PID" ] && kill -0 "$ENVOY_PID" 2>/dev/null \
       && [ "${ROUTER_DRAIN_TIME_SECONDS}" -gt 0 ] 2>/dev/null; then
        log "Draining Router (Envoy); waiting up to ${ROUTER_DRAIN_TIME_SECONDS}s for in-flight requests..."
        if drain_router; then
            sleep "${ROUTER_DRAIN_TIME_SECONDS}"
        fi
    fi

    # Terminate in dependency order: the Router (Envoy) exits first; once it is gone the
    # Policy Engine exits. Each step waits for the process to fully exit before the next.
    set +e
    stop_proc ENVOY_PID "Router (Envoy)"
    stop_proc PE_PID    "Policy Engine / dlv"
    set -e

    # Cleanup socket
    rm -f "${POLICY_ENGINE_SOCKET}"

    log "Shutdown complete"
    exit 0
}

# Set up signal handlers
trap shutdown SIGTERM SIGINT SIGQUIT

# Start Policy Engine under dlv for remote debugging (port 2346)
log "Starting Policy Engine under dlv (listening on :2346, headless)..."
/usr/local/bin/dlv exec /app/policy-engine \
    --listen=:2346 --headless=true \
    --api-version=2 --accept-multiclient -- \
    -xds-server "${PE_XDS_SERVER}" "${PE_ARGS[@]}" \
    > >(while IFS= read -r line; do echo "[pol] $line"; done) \
    2> >(while IFS= read -r line; do echo "[pol] $line" >&2; done) &
PE_PID=$!
log "Policy Engine (dlv) started (PID $PE_PID)"

# Wait for Policy Engine to create the socket (with timeout)
# Increased to 60s to account for dlv startup overhead before policy-engine initialises
SOCKET_WAIT_TIMEOUT=60
SOCKET_WAIT_COUNT=0
while [ ! -S "${POLICY_ENGINE_SOCKET}" ]; do
    if [ $SOCKET_WAIT_COUNT -ge $SOCKET_WAIT_TIMEOUT ]; then
        log "ERROR: Policy Engine socket not created within ${SOCKET_WAIT_TIMEOUT}s"
        if kill -0 "$PE_PID" 2>/dev/null; then
            log "Stopping Policy Engine / dlv (PID $PE_PID)..."
            kill -TERM "$PE_PID" 2>/dev/null || true
            wait "$PE_PID" 2>/dev/null || true
        else
            log "ERROR: Policy Engine process has already exited"
        fi
        rm -f "${POLICY_ENGINE_SOCKET}"
        exit 1
    fi

    # Check if Policy Engine is still running
    if ! kill -0 "$PE_PID" 2>/dev/null; then
        log "ERROR: Policy Engine exited before creating socket"
        exit 1
    fi

    sleep 1
    SOCKET_WAIT_COUNT=$((SOCKET_WAIT_COUNT + 1))
done
log "Policy Engine socket ready: ${POLICY_ENGINE_SOCKET}"

# Start Envoy (Router) with [rtr] log prefix
log "Starting Envoy..."
/usr/local/bin/envoy \
    -c /etc/envoy/envoy.yaml \
    --config-yaml "${CONFIG_OVERRIDE}" \
    --log-level "${LOG_LEVEL}" \
    --concurrency "${ROUTER_CONCURRENCY}" \
    "${ROUTER_ARGS[@]}" \
    > >(while IFS= read -r line; do echo "[rtr] $line"; done) \
    2> >(while IFS= read -r line; do echo "[rtr] $line" >&2; done) &
ENVOY_PID=$!
log "Envoy started (PID $ENVOY_PID)"

log "Gateway Runtime running (DEBUG) - Policy Engine/dlv (PID $PE_PID), Envoy (PID $ENVOY_PID)"

# Monitor both processes - exit if either dies
# Disable errexit so a non-zero child exit code doesn't abort before cleanup
set +e
wait -n "$PE_PID" "$ENVOY_PID"
EXIT_CODE=$?

# Determine which process exited and clean up the other
if ! kill -0 "$PE_PID" 2>/dev/null; then
    log "Policy Engine exited with code $EXIT_CODE"
    if kill -0 "$ENVOY_PID" 2>/dev/null; then
        log "Terminating Envoy due to Policy Engine exit..."
        kill -TERM "$ENVOY_PID" 2>/dev/null || true
        wait "$ENVOY_PID" 2>/dev/null || true
    fi
else
    log "Envoy exited with code $EXIT_CODE"
    if kill -0 "$PE_PID" 2>/dev/null; then
        log "Terminating Policy Engine due to Envoy exit..."
        kill -TERM "$PE_PID" 2>/dev/null || true
        wait "$PE_PID" 2>/dev/null || true
    fi
fi

rm -f "${POLICY_ENGINE_SOCKET}"
exit $EXIT_CODE
