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
# Fixed by the gateway distribution's docker-compose.yaml, not configurable.
TRAFFIC_PORT=8443
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
# MCP requires a handshake: a client sends initialize first, and only then asks
# anything else. These helpers do that, so the tests exercise the protocol the
# way a real client would rather than skipping straight to the tool calls.

MCP_INIT='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test-sh","version":"1.0"}}}'

# _post <url> <session-id-or-empty> <body>
_post() {
  if [ -n "$2" ]; then
    curl -sk -X POST "$1" \
      -H 'Content-Type: application/json' \
      -H 'Accept: application/json, text/event-stream' \
      -H "Mcp-Session-Id: $2" -d "$3"
  else
    curl -sk -X POST "$1" \
      -H 'Content-Type: application/json' \
      -H 'Accept: application/json, text/event-stream' -d "$3"
  fi
}

# mcp_call <url> <body> - opens a session, then sends the body.
# HTTP header names are case-insensitive, so the session header is matched
# case-insensitively while the id itself keeps its original casing.
# A stateless server returns no session id; the calls then simply carry none.
mcp_call() {
  _sid="$(curl -sk -D - -o /dev/null -X POST "$1" \
    -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' \
    -d "$MCP_INIT" | tr -d '\r' \
    | awk 'tolower($0) ~ /^mcp-session-id:/ { sub(/^[^:]*:[[:space:]]*/, ""); print; exit }')"
  _post "$1" "$_sid" '{"jsonrpc":"2.0","method":"notifications/initialized"}' >/dev/null
  _post "$1" "$_sid" "$2"
}

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

DIRECT="$(mcp_call "http://localhost:${MCP_PORT}/mcp" \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list"}')"

echo "$DIRECT" | grep -q 'echo_message'
check $? "generated server lists echo_message"

echo "$DIRECT" | grep -q 'echo_and_verify'
check $? "generated server lists echo_and_verify"

# ---------------------------------------------------------------------------
title "3. The same tools are reachable through the gateway"

VIA_GW="$(mcp_call "https://localhost:${TRAFFIC_PORT}/sample-service/mcp" \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list"}')"

echo "$VIA_GW" | grep -q 'echo_message'
check $? "gateway lists echo_message"

echo "$VIA_GW" | grep -q 'echo_and_verify'
check $? "gateway lists echo_and_verify"

# ---------------------------------------------------------------------------
title "4. Calling a tool through the gateway reaches the REST backend"

CALL="$(mcp_call "https://localhost:${TRAFFIC_PORT}/sample-service/mcp" \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo_message","arguments":{"message":"test-sh-probe"}}}')"

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
