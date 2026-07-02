#!/usr/bin/env bash
# test.sh — verifies the agent works end-to-end.
# Run AFTER setup.sh has completed successfully.
#
# Usage:
#   ./test.sh
#   ./test.sh "What is the return policy for gold members?"
#
# If a question is provided it is passed to the agent instead of the default.
# Exit code 0 = all checks passed.

set -uo pipefail

PASS=0
FAIL=0

ok()   { echo "  [PASS] $*"; PASS=$((PASS+1)); }
fail() { echo "  [FAIL] $*"; FAIL=$((FAIL+1)); }

check_mcp() {
  local label="$1" url="$2"
  local body='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0.1"}}}'
  local http_code
  http_code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$url" \
    -H "Content-Type: application/json" -d "$body" 2>/dev/null)
  if [[ "$http_code" == "200" ]]; then
    ok "$label (HTTP $http_code)"
  else
    fail "$label (HTTP $http_code — expected 200)"
  fi
}

echo ""
echo "========================================================"
echo " Scenario 4 — Multi-Server MCP Agent: Test Suite       "
echo "========================================================"

# Prerequisites
# These checks verify the infrastructure is up before running the agent.
# If any fail, fix the setup before proceeding.
echo ""
echo "━━━ Prerequisites ━━━"
echo " Checking MCP backends and gateway routes are reachable..."
echo " (These must all pass for the agent to work.)"
echo ""
check_mcp "CRM backend    (direct  :8001)" "http://localhost:8001/mcp"
check_mcp "Orders backend (direct  :8002)" "http://localhost:8002/mcp"
check_mcp "KB backend     (direct  :8003)" "http://localhost:8003/mcp"
check_mcp "CRM proxy      (gateway :8080/crm)"    "http://localhost:8080/crm/mcp"
check_mcp "Orders proxy   (gateway :8080/orders)" "http://localhost:8080/orders/mcp"
check_mcp "KB proxy       (gateway :8080/kb)"     "http://localhost:8080/kb/mcp"

if (( FAIL > 0 )); then
  echo ""
  echo "  Some prerequisites failed. Fix the setup before running the agent."
  echo "  Tip: re-run setup.sh, or check 'docker compose ps' for unhealthy containers."
  echo ""
  echo "========================================================"
  echo " Results: ${PASS} passed, ${FAIL} failed               "
  echo "========================================================"
  exit 1
fi

# Main: agent run
# The agent must connect to all three MCP servers
# through the gateway, call tools, and return a coherent answer.
echo ""
echo "━━━ Agent (main test) ━━━"

CUSTOM_QUESTION="${1:-}"

if [[ -n "$CUSTOM_QUESTION" ]]; then
  echo " Question: $CUSTOM_QUESTION"
  echo " Expected: A coherent response — review it manually below."
  echo ""
  AGENT_OUT=$(QUESTION="$CUSTOM_QUESTION" python agent.py 2>&1) || true
else
  SMOKE_Q="Who is customer C-4821? What is their latest order status? Reply in two sentences."
  echo " Question: $SMOKE_Q"
  echo " Expected: Response mentions John Smith (C-4821) and order O-9901 (in transit)."
  echo ""
  AGENT_OUT=$(QUESTION="$SMOKE_Q" python agent.py 2>&1) || true
fi

echo "──── Agent response ────────────────────────────────────"
echo "$AGENT_OUT"
echo "────────────────────────────────────────────────────────"
echo ""

if [[ -n "$CUSTOM_QUESTION" ]]; then
  # Custom question
  if [[ -n "$AGENT_OUT" ]]; then
    ok "Agent completed and returned a response"
  else
    fail "Agent produced no output"
  fi
else
  # Smoke-test — verify key facts appear in the response
  if echo "$AGENT_OUT" | grep -qi "john\|smith\|C-4821"; then
    ok "Response mentions John Smith / C-4821"
  else
    fail "Response did not mention John Smith or C-4821"
  fi
  if echo "$AGENT_OUT" | grep -qi "O-9901\|in.transit\|in_transit"; then
    ok "Response mentions order O-9901 / in transit"
  else
    fail "Response did not mention order O-9901 or its status"
  fi
fi

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "========================================================"
echo " Results: ${PASS} passed, ${FAIL} failed               "
echo "========================================================"
echo ""

[[ $FAIL -eq 0 ]]
