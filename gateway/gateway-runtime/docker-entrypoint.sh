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

# Gateway Runtime Entrypoint Script
# Manages both Policy Engine and Envoy processes
#
# Process-specific args can be passed using prefixed flags:
#   --rtr.<flag> <value>   → forwarded to Router (Envoy)
#   --pol.<flag> <value>   → forwarded to Policy Engine
#
# Examples:
#   docker run gateway-runtime --rtr.component-log-level upstream:debug --pol.log-format text
#   In Kubernetes:
#     args: ["--rtr.concurrency", "4", "--pol.log-format", "text"]

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

# Derive Router (Envoy) xDS config — used by envsubst on config-override.yaml
export XDS_SERVER_HOST="${GATEWAY_CONTROLLER_HOST}"
export XDS_SERVER_PORT="${ROUTER_XDS_PORT}"

# Policy Engine xDS address
PE_XDS_SERVER="${GATEWAY_CONTROLLER_HOST}:${POLICY_ENGINE_XDS_PORT}"

POLICY_ENGINE_SOCKET="/app/policy-engine.sock"

log "Starting Gateway Runtime"
log "  Gateway Controller: ${GATEWAY_CONTROLLER_HOST}"
log "  Router xDS: ${GATEWAY_CONTROLLER_HOST}:${ROUTER_XDS_PORT}"
log "  Policy Engine xDS: ${PE_XDS_SERVER}"
log "  Log Level: ${LOG_LEVEL}"
log "  Policy Engine Socket: ${POLICY_ENGINE_SOCKET}"
[[ ${#ROUTER_ARGS[@]} -gt 0 ]] && log "  Router extra args: ${ROUTER_ARGS[*]}"
[[ ${#PE_ARGS[@]} -gt 0 ]] && log "  Policy Engine extra args: ${PE_ARGS[*]}"

# Cleanup stale socket from previous runs
rm -f "${POLICY_ENGINE_SOCKET}"

# Generate Envoy config override by substituting environment variables
CONFIG_OVERRIDE=$(envsubst < /etc/envoy/config-override.yaml)

# Track child PIDs
PE_PID=""
ENVOY_PID=""

# Shutdown handler - gracefully terminate both processes
shutdown() {
    log "Received shutdown signal, terminating processes..."

    # Send SIGTERM to both processes
    if [ -n "$PE_PID" ] && kill -0 "$PE_PID" 2>/dev/null; then
        log "Stopping Policy Engine (PID $PE_PID)..."
        kill -TERM "$PE_PID" 2>/dev/null || true
    fi

    if [ -n "$ENVOY_PID" ] && kill -0 "$ENVOY_PID" 2>/dev/null; then
        log "Stopping Envoy (PID $ENVOY_PID)..."
        kill -TERM "$ENVOY_PID" 2>/dev/null || true
    fi

    # Wait for processes to exit
    wait

    # Cleanup socket
    rm -f "${POLICY_ENGINE_SOCKET}"

    log "Shutdown complete"
    exit 0
}

# Set up signal handlers
trap shutdown SIGTERM SIGINT SIGQUIT

# Start Policy Engine with [pol] log prefix
log "Starting Policy Engine..."
/app/policy-engine -xds-server "${PE_XDS_SERVER}" "${PE_ARGS[@]}" \
    > >(while IFS= read -r line; do echo "[pol] $line"; done) \
    2> >(while IFS= read -r line; do echo "[pol] $line" >&2; done) &
PE_PID=$!
log "Policy Engine started (PID $PE_PID)"

# Wait for Policy Engine to create the socket (with timeout)
SOCKET_WAIT_TIMEOUT=10
SOCKET_WAIT_COUNT=0
while [ ! -S "${POLICY_ENGINE_SOCKET}" ]; do
    if [ $SOCKET_WAIT_COUNT -ge $SOCKET_WAIT_TIMEOUT ]; then
        log "ERROR: Policy Engine socket not created within ${SOCKET_WAIT_TIMEOUT}s"
        log "Checking if Policy Engine is still running..."
        if ! kill -0 "$PE_PID" 2>/dev/null; then
            log "ERROR: Policy Engine process has exited"
        fi
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
    "${ROUTER_ARGS[@]}" \
    > >(while IFS= read -r line; do echo "[rtr] $line"; done) \
    2> >(while IFS= read -r line; do echo "[rtr] $line" >&2; done) &
ENVOY_PID=$!
log "Envoy started (PID $ENVOY_PID)"

log "Gateway Runtime running - Policy Engine (PID $PE_PID), Envoy (PID $ENVOY_PID)"

# Monitor both processes - exit if either dies
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
