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

# Tests for the gateway-route mode of health-check.sh. Runs the script against a local
# HTTP responder whose answer can be switched between a status code and no answer.
#
#   bash tests/health-check-test.sh
#
# Needs bash, curl and python3. Takes about ten seconds.

set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HEALTH_CHECK="${SCRIPT_DIR}/../health-check.sh"

for tool in curl python3; do
    if ! command -v "${tool}" >/dev/null 2>&1; then
        echo "SKIP: ${tool} is not installed"
        exit 0
    fi
done

WORK_DIR="$(mktemp -d)"
STATUS_FILE="${WORK_DIR}/status"
failures=0
stub_pid=""

cleanup() {
    if [ -n "${stub_pid}" ]; then
        kill "${stub_pid}" 2>/dev/null || true
        wait "${stub_pid}" 2>/dev/null || true
    fi
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

set_status() {
    echo "$1" > "${STATUS_FILE}"
}

# A responder that answers with the status in STATUS_FILE, or not at all for "hang".
# It records the path of the last request, so the route the script calls can be checked.
start_stub() {
    python3 - "${STATUS_FILE}" "${WORK_DIR}/port" "${WORK_DIR}/path" <<'PY' &
import http.server, sys, time

status_file, port_file, path_file = sys.argv[1], sys.argv[2], sys.argv[3]

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        with open(path_file, "w") as f:
            f.write(self.path)
        with open(status_file) as f:
            status = f.read().strip()
        if status == "hang":
            time.sleep(10)
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

# run_check <route> <port> runs the script and sets exit_code and output.
run_check() {
    output="$(GATEWAY_HEALTH_ROUTE="$1" ROUTER_HTTP_PORT="$2" bash "${HEALTH_CHECK}" 2>&1)"
    exit_code=$?
}

start_stub
port="$(cat "${WORK_DIR}/port")"

set_status 200
run_check ready "${port}"
check "200 is healthy" "${exit_code}" "0"
check "ready calls the readiness route" "$(cat "${WORK_DIR}/path")" "/_gateway-health/ready"

run_check healthy "${port}"
check "healthy calls the liveness route" "$(cat "${WORK_DIR}/path")" "/_gateway-health/healthy"

set_status 503
run_check ready "${port}"
check "503 is unhealthy" "${exit_code}" "1"
check "503 is reported" "${output}" "Gateway not ready (HTTP 503)"

set_status hang
started=${SECONDS}
run_check ready "${port}"
elapsed=$((SECONDS - started))
check "no answer is unhealthy" "${exit_code}" "1"
check "no answer is reported as no HTTP status" "${output}" "Gateway not ready (HTTP 000)"
check "no answer ends within the time limit" "$([ "${elapsed}" -le 5 ] && echo yes || echo "no (${elapsed}s)")" "yes"

kill "${stub_pid}" 2>/dev/null || true
wait "${stub_pid}" 2>/dev/null || true
stub_pid=""
run_check ready "${port}"
check "nothing listening is unhealthy" "${exit_code}" "1"

run_check live "${port}"
check "an unsupported route is rejected" "${exit_code}" "1"
check "an unsupported route is reported" "${output}" "unsupported GATEWAY_HEALTH_ROUTE 'live' (use ready or healthy)"

if [ "${failures}" -gt 0 ]; then
    echo "${failures} check(s) failed"
    exit 1
fi
echo "all checks passed"
