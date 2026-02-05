#!/bin/bash

# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

# Unified Gateway Entrypoint Script
# Manages both Policy Engine and Envoy processes

set -e

# Logging function
log() {
    echo "[unified-gw] $(date '+%Y-%m-%d %H:%M:%S') $1"
}

# Default configuration
export XDS_SERVER_HOST="${XDS_SERVER_HOST:-gateway-controller}"
export XDS_SERVER_PORT="${XDS_SERVER_PORT:-18000}"
export LOG_LEVEL="${LOG_LEVEL:-info}"
export POLICY_ENGINE_SOCKET="${POLICY_ENGINE_SOCKET:-/var/run/policy-engine.sock}"
export POLICY_ENGINE_CONFIG="${POLICY_ENGINE_CONFIG:-/etc/policy-engine/config.toml}"

log "Starting Unified Gateway"
log "  xDS Server: ${XDS_SERVER_HOST}:${XDS_SERVER_PORT}"
log "  Log Level: ${LOG_LEVEL}"
log "  Policy Engine Socket: ${POLICY_ENGINE_SOCKET}"

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

# Start Policy Engine
log "Starting Policy Engine..."
/app/policy-engine -config "${POLICY_ENGINE_CONFIG}" &
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

# Start Envoy
log "Starting Envoy..."
/usr/local/bin/envoy \
    -c /etc/envoy/envoy.yaml \
    --config-yaml "${CONFIG_OVERRIDE}" \
    --log-level "${LOG_LEVEL}" \
    "$@" &
ENVOY_PID=$!
log "Envoy started (PID $ENVOY_PID)"

log "Unified Gateway running - Policy Engine (PID $PE_PID), Envoy (PID $ENVOY_PID)"

# Monitor both processes - exit if either dies
while true; do
    # Check if Policy Engine is still running
    if ! kill -0 "$PE_PID" 2>/dev/null; then
        wait "$PE_PID" 2>/dev/null
        EXIT_CODE=$?
        log "Policy Engine exited with code $EXIT_CODE"

        # Terminate Envoy
        if kill -0 "$ENVOY_PID" 2>/dev/null; then
            log "Terminating Envoy due to Policy Engine exit..."
            kill -TERM "$ENVOY_PID" 2>/dev/null || true
            wait "$ENVOY_PID" 2>/dev/null || true
        fi

        rm -f "${POLICY_ENGINE_SOCKET}"
        exit $EXIT_CODE
    fi

    # Check if Envoy is still running
    if ! kill -0 "$ENVOY_PID" 2>/dev/null; then
        wait "$ENVOY_PID" 2>/dev/null
        EXIT_CODE=$?
        log "Envoy exited with code $EXIT_CODE"

        # Terminate Policy Engine
        if kill -0 "$PE_PID" 2>/dev/null; then
            log "Terminating Policy Engine due to Envoy exit..."
            kill -TERM "$PE_PID" 2>/dev/null || true
            wait "$PE_PID" 2>/dev/null || true
        fi

        rm -f "${POLICY_ENGINE_SOCKET}"
        exit $EXIT_CODE
    fi

    # Sleep before next check
    sleep 1
done
