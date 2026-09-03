#!/bin/sh
set -u

# Checks the whole chain, one layer at a time, so a failure tells you which
# piece is broken rather than just "it didn't work".

if command -v tput >/dev/null 2>&1 && [ -n "${TERM:-}" ] && tput setaf 2 >/dev/null 2>&1; then
  GREEN="$(tput setaf 2)"; RED="$(tput setaf 1)"; BOLD="$(tput bold)"; RESET="$(tput sgr0)"
else
  GREEN=""; RED=""; BOLD=""; RESET=""
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
[ -f "${SCRIPT_DIR}/.env" ] && . "${SCRIPT_DIR}/.env"
TRAFFIC_PORT="${TRAFFIC_PORT:-8443}"
MCP_PORT="${MCP_PORT:-5050}"

PASS=0
FAIL=0
check() {
  if [ "$1" = "0" ]; then
    echo "${GREEN}PASS${RESET}  $2"; PASS=$((PASS + 1))
  else
    echo "${RED}FAIL${RESET}  $2"; FAIL=$((FAIL + 1))
  fi
}
title() { echo ""; echo "${BOLD}$1${RESET}"; }

echo "${BOLD}=== rest-to-mcp tests ===${RESET}"

# ---------------------------------------------------------------------------
title "1. The REST backend answers, and does NOT speak MCP"

curl -sf -o /dev/null http://localhost:8090/health
check $? "sample-service /health responds"

RAW="$(curl -s -X POST http://localhost:8090/mcp \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')"
echo "$RAW" | grep -q '"tools"'
if [ $? -eq 0 ]; then
  check 1 "sample-service does not answer tools/list (it returned a tool list!)"
else
  check 0 "sample-service does not answer tools/list - it is plain REST"
fi

# ---------------------------------------------------------------------------
title "2. The generated MCP server exposes the workflows as tools"

DIRECT="$(curl -s -X POST "http://localhost:${MCP_PORT}/mcp" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')"

echo "$DIRECT" | grep -q 'echo_message'
check $? "generated server lists echo_message"

echo "$DIRECT" | grep -q 'echo_and_verify'
check $? "generated server lists echo_and_verify"

# ---------------------------------------------------------------------------
title "3. The same tools are reachable through the gateway"

VIA_GW="$(curl -sk -X POST "https://localhost:${TRAFFIC_PORT}/sample-service/mcp" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')"

echo "$VIA_GW" | grep -q 'echo_message'
check $? "gateway lists echo_message"

echo "$VIA_GW" | grep -q 'echo_and_verify'
check $? "gateway lists echo_and_verify"

# ---------------------------------------------------------------------------
title "4. Calling a tool through the gateway reaches the REST backend"

CALL="$(curl -sk -X POST "https://localhost:${TRAFFIC_PORT}/sample-service/mcp" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo_message","arguments":{"message":"test-sh-probe"}}}')"

echo "$CALL" | grep -q 'test-sh-probe'
check $? "echo_message round-trips the message through to sample-service"

# The backend remembers the last request it saw, which proves the call
# actually arrived rather than being answered somewhere upstream.
sleep 1
CAPTURED="$(curl -s http://localhost:8090/captured-request)"
echo "$CAPTURED" | grep -q 'test-sh-probe'
check $? "sample-service captured the request from that tool call"

# ---------------------------------------------------------------------------
echo ""
echo "${BOLD}=== ${PASS} passed, ${FAIL} failed ===${RESET}"
[ "$FAIL" -eq 0 ] || exit 1
