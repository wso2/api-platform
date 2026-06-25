#!/bin/sh
set -eu

# Initialise tput colors if available
if command -v tput >/dev/null 2>&1 && [ -n "${TERM:-}" ] && tput setaf 2 >/dev/null 2>&1; then
  GREEN="$(tput setaf 2)"
  RED="$(tput setaf 1)"
  BOLD="$(tput bold)"
  RESET="$(tput sgr0)"
else
  GREEN=""; RED=""; BOLD=""; RESET=""
fi

print_ok() {
  echo "${GREEN}✔  $1${RESET}"
}

print_error() {
  echo "${RED}✖  $1${RESET}"
}

print_title() {
  echo ""
  echo "${BOLD}${GREEN}=== $1 ===${RESET}"
  echo ""
}

print_result() {
  echo "${BOLD}$1${RESET}"
}

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ ! -f "${SCRIPT_DIR}/.env" ] && [ -f "${SCRIPT_DIR}/.env.example" ]; then
  cp "${SCRIPT_DIR}/.env.example" "${SCRIPT_DIR}/.env"
fi
if [ -f "${SCRIPT_DIR}/.env" ]; then
  . "${SCRIPT_DIR}/.env"
fi

TRAFFIC_PORT="${TRAFFIC_PORT:-8443}"
BEARER_TOKEN="${BEARER_TOKEN:-}"
TARGET="https://localhost:${TRAFFIC_PORT}/reading-list/mcp"

if [ -z "$BEARER_TOKEN" ]; then
  print_error "BEARER_TOKEN is not set. Copy .env.example to .env and try again."
  exit 1
fi

# Helper: send a JSON-RPC request, return "<body>\n<status>"
mcp_call() {
  curl -sk -w "\n%{http_code}" -X POST "$TARGET" \
    -H "Content-Type: application/json" \
    "$@"
}

PASSED=0
FAILED=0

check() {
  label="$1"
  got="$2"
  want="$3"
  if [ "$got" = "$want" ]; then
    print_ok "$label (HTTP $got)"
    PASSED=$((PASSED + 1))
  else
    print_error "$label — expected HTTP $want, got $got"
    FAILED=$((FAILED + 1))
  fi
}

INIT_PAYLOAD='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0"}}}'
LIST_PAYLOAD='{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
CALL_LIST='{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"listBooks","arguments":{}}}'
CALL_ADD='{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"addBook","arguments":{"title":"The Phoenix Project","author":"Gene Kim","status":"to_read"}}}'
CALL_DELETE='{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"deleteBook","arguments":{"id":"3"}}}'

print_title "WSO2 AI Gateway — MCP Proxy Test"
echo "Target : ${TARGET}"

# ── Test 1: No token → 401 ────────────────────────────────────────────────────
print_title "Test 1: No token — expect HTTP 401"
FULL=$(mcp_call -d "$INIT_PAYLOAD")
STATUS=$(echo "$FULL" | tail -1)
BODY=$(echo "$FULL" | sed '$d')
echo "Response : ${BODY}"
check "initialize (no token)" "$STATUS" "401"

# ── Test 2: initialize with valid token → 200 ─────────────────────────────────
print_title "Test 2: initialize with valid token — expect HTTP 200"
FULL=$(mcp_call -H "Authorization: Bearer ${BEARER_TOKEN}" -d "$INIT_PAYLOAD")
STATUS=$(echo "$FULL" | tail -1)
BODY=$(echo "$FULL" | sed '$d')
echo "Response : ${BODY}"
check "initialize (valid token)" "$STATUS" "200"

# ── Test 3: tools/list → 200 ──────────────────────────────────────────────────
print_title "Test 3: tools/list — expect HTTP 200 with 3 tools"
FULL=$(mcp_call -H "Authorization: Bearer ${BEARER_TOKEN}" -d "$LIST_PAYLOAD")
STATUS=$(echo "$FULL" | tail -1)
BODY=$(echo "$FULL" | sed '$d')
echo "Response : ${BODY}"
check "tools/list" "$STATUS" "200"

# ── Test 4: tools/call listBooks → 200 ───────────────────────────────────────
print_title "Test 4: tools/call listBooks — expect HTTP 200"
FULL=$(mcp_call -H "Authorization: Bearer ${BEARER_TOKEN}" -d "$CALL_LIST")
STATUS=$(echo "$FULL" | tail -1)
BODY=$(echo "$FULL" | sed '$d')
echo "Response : ${BODY}"
check "tools/call listBooks" "$STATUS" "200"

# ── Test 5: tools/call addBook → 200 ─────────────────────────────────────────
print_title "Test 5: tools/call addBook — expect HTTP 200"
FULL=$(mcp_call -H "Authorization: Bearer ${BEARER_TOKEN}" -d "$CALL_ADD")
STATUS=$(echo "$FULL" | tail -1)
BODY=$(echo "$FULL" | sed '$d')
echo "Response : ${BODY}"
check "tools/call addBook" "$STATUS" "200"

# ── Test 6: tools/call deleteBook → 200 ──────────────────────────────────────
print_title "Test 6: tools/call deleteBook — expect HTTP 200"
FULL=$(mcp_call -H "Authorization: Bearer ${BEARER_TOKEN}" -d "$CALL_DELETE")
STATUS=$(echo "$FULL" | tail -1)
BODY=$(echo "$FULL" | sed '$d')
echo "Response : ${BODY}"
check "tools/call deleteBook" "$STATUS" "200"

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
if [ "$FAILED" -eq 0 ]; then
  print_result "✔  PASSED — All ${PASSED} tests passed."
  exit 0
else
  print_result "✖  FAILED — ${PASSED} passed, ${FAILED} failed."
  echo ""
  echo "Troubleshooting:"
  echo "  - Verify setup completed: sh setup.sh"
  echo "  - Check containers: docker ps"
  echo "  - Check gateway logs: cd wso2apip-ai-gateway-1.1.0 && docker compose logs"
  exit 1
fi
