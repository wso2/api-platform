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

# Gateway Runtime Health Watchdog
#
# Logs when the gateway stops answering its own liveness route and when it answers
# again. The policy engine cannot log its own hang, so this runs as a separate process
# beside it, started by the entrypoint. It calls /_gateway-health/healthy on the local
# router, the same route liveness probes use, and logs state changes in the format of
# the policy engine's health log:
#
#   health check failed     once when the route stops answering, then repeated on a
#                           doubling backoff while it stays down
#   health check recovered  once when the route answers again
#
# It stays silent until the route has answered once, so a normal startup logs nothing.
#
# Settings (environment):
#   ROUTER_HTTP_PORT                          router HTTP port (default 8080)
#   HEALTH_WATCHDOG_INTERVAL_SECONDS          seconds between checks (default 5)
#   HEALTH_WATCHDOG_TIMEOUT_SECONDS           seconds to wait for an answer (default 3)
#   HEALTH_WATCHDOG_BACKOFF_INITIAL_SECONDS   delay before the first repeated failure line (default 10)
#   HEALTH_WATCHDOG_BACKOFF_MAX_SECONDS       cap on the repeat interval (default 300)
#   GATEWAY_HEALTH_LOG_FILE                   absolute path of a file to append the lines to as
#                                             well (owner-only); empty logs to stdout only

set -u

warn() {
    echo "[hcw] $(date -u '+%Y-%m-%dT%H:%M:%SZ') level=WARN msg=\"$1\""
}

# setting prints the named variable when it is a positive integer, otherwise the default.
setting() {
    local name="$1" default="$2" value="${!1:-}"
    if [ -z "${value}" ]; then
        echo "${default}"
    elif [[ "${value}" =~ ^[1-9][0-9]*$ ]]; then
        echo "${value}"
    else
        warn "ignoring ${name}=${value}: not a positive integer, using ${default}" >&2
        echo "${default}"
    fi
}

INTERVAL="$(setting HEALTH_WATCHDOG_INTERVAL_SECONDS 5)"
TIMEOUT="$(setting HEALTH_WATCHDOG_TIMEOUT_SECONDS 3)"
BACKOFF_INITIAL="$(setting HEALTH_WATCHDOG_BACKOFF_INITIAL_SECONDS 10)"
BACKOFF_MAX="$(setting HEALTH_WATCHDOG_BACKOFF_MAX_SECONDS 300)"
if [ "${BACKOFF_MAX}" -lt "${BACKOFF_INITIAL}" ]; then
    warn "HEALTH_WATCHDOG_BACKOFF_MAX_SECONDS is below the initial backoff; using ${BACKOFF_INITIAL}" >&2
    BACKOFF_MAX="${BACKOFF_INITIAL}"
fi
URL="http://127.0.0.1:${ROUTER_HTTP_PORT:-8080}/_gateway-health/healthy"

LOG_FILE="${GATEWAY_HEALTH_LOG_FILE:-}"
if [ -n "${LOG_FILE}" ] && [ "${LOG_FILE#/}" = "${LOG_FILE}" ]; then
    warn "ignoring GATEWAY_HEALTH_LOG_FILE: not an absolute path; logging to stdout only" >&2
    LOG_FILE=""
fi

# emit writes one health line to stdout and, when configured, appends it to the health
# log file. The file is opened per line so writing continues across rotation, and is
# created owner-only.
emit() {
    local line
    line="time=$(date -u '+%Y-%m-%dT%H:%M:%SZ') $1"
    echo "[hcw] ${line}"
    if [ -n "${LOG_FILE}" ]; then
        (umask 077 && printf '%s\n' "${line}" >> "${LOG_FILE}") 2>/dev/null || true
    fi
}

sleep_pid=""
trap 'if [ -n "${sleep_pid}" ]; then kill "${sleep_pid}" 2>/dev/null; fi; exit 0' TERM INT

armed=false
down=false
down_since=0
failed_probes=0
backoff="${BACKOFF_INITIAL}"
next_repeat=0

while true; do
    code="$(curl -s -o /dev/null -m "${TIMEOUT}" -w '%{http_code}' "${URL}" 2>/dev/null)" || true
    now="$(date +%s)"

    if [ "${code}" = "200" ]; then
        if [ "${down}" = true ]; then
            emit "level=INFO msg=\"health check recovered\" check=policy_engine down_for=$((now - down_since))s"
        fi
        armed=true
        down=false
        failed_probes=0
    elif [ "${armed}" = true ]; then
        if [ -z "${code}" ] || [ "${code}" = "000" ]; then
            reason="no answer within ${TIMEOUT}s"
        else
            reason="health route answered ${code}"
        fi
        failed_probes=$((failed_probes + 1))
        if [ "${down}" = false ]; then
            down=true
            down_since="${now}"
            backoff="${BACKOFF_INITIAL}"
            next_repeat=$((now + backoff))
            emit "level=ERROR msg=\"health check failed\" check=policy_engine reason=\"${reason}\""
        elif [ "${now}" -ge "${next_repeat}" ]; then
            emit "level=ERROR msg=\"health check failed\" check=policy_engine reason=\"${reason}\" failing_for=$((now - down_since))s failed_probes=${failed_probes}"
            backoff=$((backoff * 2))
            if [ "${backoff}" -gt "${BACKOFF_MAX}" ]; then
                backoff="${BACKOFF_MAX}"
            fi
            next_repeat=$((now + backoff))
        fi
    fi

    # Sleep in the background so a termination signal is handled at once.
    sleep "${INTERVAL}" &
    sleep_pid=$!
    wait "${sleep_pid}"
    sleep_pid=""
done
