#!/usr/bin/env bash
# Test: Word Count Guard (word-count-guardrail, max 500 words)
#   Sends a normal-length prompt (~65 words; expect HTTP 200) and a verbose
#   prompt (2000+ words; expect HTTP 422 BLOCKED).
#
#   The verbose case uses short repeated tokens rather than ordinary prose:
#   average English prose runs ~5-6 bytes/word, so a genuinely prose-like
#   2000-word message would itself exceed the 5 KB content-length-guardrail
#   limit and get blocked there first (content-length-guardrail runs before
#   word-count-guardrail in the policy chain — see llm-proxy.yaml). Short
#   tokens keep this case isolated to word-count-guardrail specifically.
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

word_count() { echo -n "$1" | wc -w | tr -d ' '; }

echo ""
echo "══════════════════════════════════════════════════"
echo " Pre-flight checks"
echo "══════════════════════════════════════════════════"

if ! command -v jq >/dev/null 2>&1; then
  echo -e "${RED}[ERROR]${NC} jq is required: brew install jq" >&2; exit 1
fi

info "Checking gateway health at ${HEALTH_URL} ..."
if ! curl -sf "${HEALTH_URL}" >/dev/null 2>&1; then
  echo -e "${RED}[ERROR]${NC} Gateway is not running. Run ./setup.sh first." >&2; exit 1
fi
echo -e "${GREEN}[OK]${NC}    Gateway is healthy."

FAILURES=0

# ---------------------------------------------------------------------------
# Case 1 — Normal-length prompt (~65 words) — expect HTTP 200
# ---------------------------------------------------------------------------
echo ""
echo "══════════════════════════════════════════════════"
echo " Case 1: Normal-length prompt (~65 words) — expect HTTP 200 PASS"
echo "══════════════════════════════════════════════════"

NORMAL_PROMPT="Can you explain how the Pythagorean theorem works in basic geometry? I am studying trigonometry for an upcoming exam and would like a clear step by step walkthrough of the proof, including why the sum of the squares of the two shorter sides always equals the square of the hypotenuse, plus one simple worked numeric example so I can check my understanding before the test."
info "Word count: $(word_count "${NORMAL_PROMPT}") words (limit: 500 words)"

FULL=$(chat_req "${NORMAL_PROMPT}")
STATUS=$(echo "${FULL}" | tail -1)
BODY=$(echo "${FULL}" | sed '$d')

echo "Status : HTTP ${STATUS}"
echo "Body   : ${BODY}"
if [[ "${STATUS}" == "200" ]]; then
  pass "Normal-length prompt passed word-count-guardrail and reached the mock LLM."
else
  fail "Expected HTTP 200, got HTTP ${STATUS}."
  FAILURES=$((FAILURES + 1))
fi

# ---------------------------------------------------------------------------
# Case 2 — Verbose prompt (2000+ words) — expect HTTP 422 BLOCKED
# ---------------------------------------------------------------------------
echo ""
echo "══════════════════════════════════════════════════"
echo " Case 2: Verbose prompt (2000+ words) — expect HTTP 422 BLOCKED"
echo "══════════════════════════════════════════════════"

VERBOSE_PROMPT=$(printf 'a %.0s' $(seq 1 2200))
info "Word count  : $(word_count "${VERBOSE_PROMPT}") words (limit: 500 words)"
info "Content size: ${#VERBOSE_PROMPT} bytes (content-length-guardrail limit: 5120 bytes — stays under it on purpose)"

FULL=$(chat_req "${VERBOSE_PROMPT}")
STATUS=$(echo "${FULL}" | tail -1)
BODY=$(echo "${FULL}" | sed '$d')
GUARDRAIL_TYPE=$(echo "${BODY}" | jq -r '.type // "unknown"' 2>/dev/null)

echo "Status         : HTTP ${STATUS}"
echo "Body           : ${BODY}"
echo "Guardrail type : ${GUARDRAIL_TYPE}"
if [[ "${STATUS}" == "422" && "${GUARDRAIL_TYPE}" == "WORD_COUNT_GUARDRAIL" ]]; then
  pass "Verbose prompt BLOCKED by word-count-guardrail."
else
  fail "Expected HTTP 422 / WORD_COUNT_GUARDRAIL, got HTTP ${STATUS} / ${GUARDRAIL_TYPE}."
  FAILURES=$((FAILURES + 1))
fi

echo ""
echo "══════════════════════════════════════════════════"
if [[ "${FAILURES}" -eq 0 ]]; then
  pass "Word Count Guard: 2/2 cases behaved as expected."
else
  fail "Word Count Guard: ${FAILURES} case(s) did not behave as expected."
fi
echo "══════════════════════════════════════════════════"
echo ""
exit "${FAILURES}"
