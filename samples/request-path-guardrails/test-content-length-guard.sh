#!/usr/bin/env bash
# Test: Content Length Guard (content-length-guardrail, max 5120 bytes)
#   Sends a small message (well under the 5 KB limit; expect HTTP 200) and an
#   oversized message (10 KB+; expect HTTP 422 BLOCKED). content-length-guardrail
#   runs first in the policy chain, so the oversized case never reaches the
#   word-count, regex, or semantic-prompt-guard checks.
set -uo pipefail

CHAT_URL="https://localhost:8443/api/llm/chat/completions"
# The gateway's admin/health API is plain HTTP even when the traffic listener
# is HTTPS — two different Envoy/controller listeners, not the same scheme.
HEALTH_URL="http://localhost:9094/api/admin/v1/health"

GREEN="\033[0;32m"; RED="\033[0;31m"; BLUE="\033[0;34m"; NC="\033[0m"
pass() { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; }
info() { echo -e "${BLUE}[INFO]${NC} $*"; }

chat_req() {
  local msg="$1"
  local body
  body=$(jq -n --arg m "${msg}" '{"model":"gpt-4o-mini","messages":[{"role":"user","content":$m}]}')
  curl -sk --max-time 15 -w "\n%{http_code}" -X POST "${CHAT_URL}" \
    -H "Content-Type: application/json" \
    -d "${body}"
}

echo ""
echo "══════════════════════════════════════════════════"
echo " Pre-flight checks"
echo "══════════════════════════════════════════════════"

if ! command -v jq >/dev/null 2>&1; then
  echo -e "${RED}[ERROR]${NC} jq is required: brew install jq" >&2; exit 1
fi

info "Checking gateway health at ${HEALTH_URL} ..."
if ! curl -sf --connect-timeout 5 --max-time 10 "${HEALTH_URL}" >/dev/null 2>&1; then
  echo -e "${RED}[ERROR]${NC} Gateway is not running. Run ./setup.sh first." >&2; exit 1
fi
echo -e "${GREEN}[OK]${NC}    Gateway is healthy."

FAILURES=0

# ---------------------------------------------------------------------------
# Case 1 — Small payload, well under the 5 KB limit — expect HTTP 200
# ---------------------------------------------------------------------------
echo ""
echo "══════════════════════════════════════════════════"
echo " Case 1: Small payload — expect HTTP 200 PASS"
echo "══════════════════════════════════════════════════"

SMALL_PROMPT="Can you explain how the Pythagorean theorem works in basic geometry and give one simple worked example?"
info "Prompt      : ${SMALL_PROMPT}"
info "Content size: ${#SMALL_PROMPT} bytes (limit: 5120 bytes)"

FULL=$(chat_req "${SMALL_PROMPT}")
STATUS=$(echo "${FULL}" | tail -1)
BODY=$(echo "${FULL}" | sed '$d')

echo "Status : HTTP ${STATUS}"
echo "Body   : ${BODY}"
if [[ "${STATUS}" == "200" ]]; then
  pass "Small payload passed content-length-guardrail and reached the mock LLM."
else
  fail "Expected HTTP 200, got HTTP ${STATUS}."
  FAILURES=$((FAILURES + 1))
fi

# ---------------------------------------------------------------------------
# Case 2 — Oversized payload, 10 KB+ — expect HTTP 422 BLOCKED
# ---------------------------------------------------------------------------
echo ""
echo "══════════════════════════════════════════════════"
echo " Case 2: Oversized payload (10 KB+) — expect HTTP 422 BLOCKED"
echo "══════════════════════════════════════════════════"

LARGE_PROMPT=$(printf 'A%.0s' $(seq 1 10240))
info "Content size: ${#LARGE_PROMPT} bytes (limit: 5120 bytes)"

FULL=$(chat_req "${LARGE_PROMPT}")
STATUS=$(echo "${FULL}" | tail -1)
BODY=$(echo "${FULL}" | sed '$d')
GUARDRAIL_TYPE=$(echo "${BODY}" | jq -r '.type // "unknown"' 2>/dev/null)

echo "Status         : HTTP ${STATUS}"
echo "Body           : ${BODY}"
echo "Guardrail type : ${GUARDRAIL_TYPE}"
if [[ "${STATUS}" == "422" && "${GUARDRAIL_TYPE}" == "CONTENT_LENGTH_GUARDRAIL" ]]; then
  pass "Oversized payload BLOCKED by content-length-guardrail."
else
  fail "Expected HTTP 422 / CONTENT_LENGTH_GUARDRAIL, got HTTP ${STATUS} / ${GUARDRAIL_TYPE}."
  FAILURES=$((FAILURES + 1))
fi

echo ""
echo "══════════════════════════════════════════════════"
if [[ "${FAILURES}" -eq 0 ]]; then
  pass "Content Length Guard: 2/2 cases behaved as expected."
else
  fail "Content Length Guard: ${FAILURES} case(s) did not behave as expected."
fi
echo "══════════════════════════════════════════════════"
echo ""
exit "${FAILURES}"
