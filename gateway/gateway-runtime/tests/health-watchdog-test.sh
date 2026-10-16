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

# Tests for health-watchdog.sh. Runs the watchdog against a local HTTP responder whose
# answer can be switched between 200, 503 and no answer, and checks what it logs.
#
#   bash tests/health-watchdog-test.sh
#
# Needs bash, curl and python3. Takes about half a minute.

set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WATCHDOG="${SCRIPT_DIR}/../health-watchdog.sh"

for tool in curl python3; do
    if ! command -v "${tool}" >/dev/null 2>&1; then
        echo "SKIP: ${tool} is not installed"
        exit 0
    fi
done

WORK_DIR="$(mktemp -d)"
STATUS_FILE="${WORK_DIR}/status"
OUT_FILE="${WORK_DIR}/stdout"
LOG_FILE="${WORK_DIR}/logs/health.log"
mkdir -p "${WORK_DIR}/logs"
failures=0
stub_pid=""
watchdog_pid=""

cleanup() {
    for pid in "${watchdog_pid}" "${stub_pid}"; do
        if [ -n "${pid}" ]; then
            kill "${pid}" 2>/dev/null || true
            wait "${pid}" 2>/dev/null || true
        fi
    done
    rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

check() {
    local description="$1" actual="$2" expected="$3"
    if [ "${actual}" = "${expected}" ]; then
        echo "ok   - ${description}"
    else
        echo "FAIL - ${description}: got '${actual}', want '${expected}'"
        failures=$((failures + 1))
    fi
}

count() {
    local pattern="$1" file="$2"
    grep -c -- "${pattern}" "${file}" 2>/dev/null || true
}

set_status() {
    echo "$1" > "${STATUS_FILE}"
}

# A responder that answers with the status in STATUS_FILE, or not at all for "hang".
start_stub() {
    python3 - "${STATUS_FILE}" "${WORK_DIR}/port" <<'PY' &
import http.server, sys, time

status_file, port_file = sys.argv[1], sys.argv[2]

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        with open(status_file) as f:
            status = f.read().strip()
        if status == "hang":
            time.sleep(5)
            return
        self.send_response(int(status))
        self.end_headers()

    def log_message(self, *args):
        pass

server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
with open(port_file, "w") as f:
    f.write(str(server.server_address[1]))
server.serve_forever()
PY
    stub_pid=$!
    for _ in $(seq 1 50); do
        [ -s "${WORK_DIR}/port" ] && return 0
        sleep 0.1
    done
    echo "FAIL - the test responder did not start"
    exit 1
}

set_status 503
start_stub

ROUTER_HTTP_PORT="$(cat "${WORK_DIR}/port")" \
HEALTH_WATCHDOG_INTERVAL_SECONDS=1 \
HEALTH_WATCHDOG_TIMEOUT_SECONDS=1 \
HEALTH_WATCHDOG_BACKOFF_INITIAL_SECONDS=2 \
HEALTH_WATCHDOG_BACKOFF_MAX_SECONDS=4 \
GATEWAY_HEALTH_LOG_FILE="${LOG_FILE}" \
    bash "${WATCHDOG}" > "${OUT_FILE}" 2>&1 &
watchdog_pid=$!

sleep 3
check "silent before the route has answered once" "$(count 'health check' "${OUT_FILE}")" "0"

set_status 200
sleep 2
check "silent while healthy" "$(count 'health check' "${OUT_FILE}")" "0"

set_status 503
sleep 2
check "one failure line when the route stops answering 200" \
    "$(count 'msg="health check failed" check=policy_engine reason="health route answered 503"$' "${OUT_FILE}")" "1"

set_status hang
sleep 7
check "repeats carry how long it has been failing" \
    "$([ "$(count 'failing_for=.*failed_probes=' "${OUT_FILE}")" -ge 1 ] && echo yes || echo no)" "yes"
check "a hung route is reported as no answer" \
    "$([ "$(count 'reason="no answer within 1s"' "${OUT_FILE}")" -ge 1 ] && echo yes || echo no)" "yes"
check "repeats back off instead of one line per check" \
    "$([ "$(count 'health check failed' "${OUT_FILE}")" -le 4 ] && echo yes || echo no)" "yes"

set_status 200
sleep 4
check "one recovery line when the route answers again" \
    "$(count 'msg="health check recovered" check=policy_engine down_for=' "${OUT_FILE}")" "1"

check "every line also reaches the health log file" \
    "$(count 'health check' "${LOG_FILE}")" "$(count 'health check' "${OUT_FILE}")"
check "the health log file is owner-only" "$(stat -c '%a' "${LOG_FILE}" 2>/dev/null || stat -f '%Lp' "${LOG_FILE}")" "600"
check "stdout lines carry the watchdog tag" \
    "$(grep 'health check' "${OUT_FILE}" | grep -vc '^\[hcw\] ' || true)" "0"

kill -TERM "${watchdog_pid}"
for _ in $(seq 1 20); do
    kill -0 "${watchdog_pid}" 2>/dev/null || break
    sleep 0.1
done
wait "${watchdog_pid}"
check "exits cleanly on SIGTERM" "$?" "0"
watchdog_pid=""

# Invalid settings fall back to the defaults with a warning, and a relative log path is
# not used.
HEALTH_WATCHDOG_INTERVAL_SECONDS=abc GATEWAY_HEALTH_LOG_FILE=relative.log ROUTER_HTTP_PORT=1 \
    bash "${WATCHDOG}" > "${WORK_DIR}/invalid" 2>&1 &
watchdog_pid=$!
sleep 1
check "warns about an invalid number" "$(count 'ignoring HEALTH_WATCHDOG_INTERVAL_SECONDS=abc' "${WORK_DIR}/invalid")" "1"
check "warns about a relative log path" "$(count 'ignoring GATEWAY_HEALTH_LOG_FILE' "${WORK_DIR}/invalid")" "1"

if [ "${failures}" -gt 0 ]; then
    echo "${failures} check(s) failed"
    exit 1
fi
echo "all checks passed"
