#!/usr/bin/env bash
# No set -e — tests must not abort on failure; we track them manually.
set -uo pipefail

CHAT_URL="http://localhost:8080/openai-proxy/chat/completions"
HEALTH_URL="http://localhost:9094/health"
FAILURES=0

# ---------------------------------------------------------------------------
# Colours
# ---------------------------------------------------------------------------
GREEN="\033[0;32m"; RED="\033[0;31m"; BLUE="\033[0;34m"; YELLOW="\033[0;33m"; NC="\033[0m"
pass() { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; FAILURES=$((FAILURES + 1)); }
info() { echo -e "${BLUE}[INFO]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }

ms() { python3 -c "import time; print(int(time.time()*1000))"; }

# ---------------------------------------------------------------------------
# Pre-flight checks
# ---------------------------------------------------------------------------
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

# ---------------------------------------------------------------------------
# Helper: POST a chat completion, capture response headers to a temp file.
# Usage: response=$(chat_req "<message>" <headers_tmp_file>)
# ---------------------------------------------------------------------------
chat_req() {
  local msg="$1" hdr_file="$2"
  local body
  body=$(jq -n --arg m "${msg}" \
    '{"model":"gpt-4","messages":[{"role":"user","content":$m}],"max_tokens":150}')
  curl -s -X POST "${CHAT_URL}" \
    -H "Content-Type: application/json" \
    -D "${hdr_file}" \
    -d "${body}"
}

# ═══════════════════════════════════════════════════════════════════════════
# Test 1 — Model Round Robin
#   Sends 3 requests and verifies the gateway cycles across different models.
#   The round-robin policy overrides the model field; the actual model used
#   is visible in OpenAI's response body.
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "══════════════════════════════════════════════════"
echo " Test 1: Model Round Robin"
echo "══════════════════════════════════════════════════"

MODELS=()
for i in 1 2 3; do
  info "Request ${i}/3 ..."
  HDR=$(mktemp)
  RESP=$(chat_req "Reply with one word: ok" "${HDR}")
  rm -f "${HDR}"

  if echo "${RESP}" | jq -e '.error' >/dev/null 2>&1; then
    ERR=$(echo "${RESP}" | jq -r '.error.message // "unknown error"')
    fail "Request ${i} errored: ${ERR}"
    MODELS+=("error")
  else
    MODEL=$(echo "${RESP}" | jq -r '.model // "unknown"')
    MODELS+=("${MODEL}")
    info "  → model used: ${MODEL}"
  fi
done

UNIQUE=$(printf '%s\n' "${MODELS[@]}" | grep -v '^error$' | sort -u | wc -l | tr -d ' ')
if [[ "${UNIQUE}" -gt 1 ]]; then
  pass "Round robin confirmed — saw ${UNIQUE} different models across 3 requests."
  info "  Sequence: ${MODELS[0]} → ${MODELS[1]} → ${MODELS[2]}"
else
  fail "Round robin not detected — all 3 requests used '${MODELS[0]:-unknown}'."
fi

# ═══════════════════════════════════════════════════════════════════════════
# Test 2 — Semantic Cache
#   Sends the same question twice. The second request should be served from
#   the semantic cache (Redis + Mistral embeddings), confirmed via a cache
#   response header or a significantly faster response time.
#
#   NOTE: requires embedding_provider_api_key in additional-config.toml.
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "══════════════════════════════════════════════════"
echo " Test 2: Semantic Cache"
echo "══════════════════════════════════════════════════"

CACHE_MSG="What is the boiling point of water in Celsius?"

info "Request 1 — cache miss expected ..."
HDR1=$(mktemp)
T1_START=$(ms); R1=$(chat_req "${CACHE_MSG}" "${HDR1}"); T1_END=$(ms)
T1=$((T1_END - T1_START))
CACHE_HDR1=$(grep -i "x-cache\|cache-status\|x-semantic" "${HDR1}" | tr -d '\r' || true)
rm -f "${HDR1}"
info "  time: ${T1}ms"
[[ -n "${CACHE_HDR1}" ]] && info "  cache header: ${CACHE_HDR1}"

info "Request 2 — cache hit expected ..."
HDR2=$(mktemp)
T2_START=$(ms); R2=$(chat_req "${CACHE_MSG}" "${HDR2}"); T2_END=$(ms)
T2=$((T2_END - T2_START))
CACHE_HDR2=$(grep -i "x-cache\|cache-status\|x-semantic" "${HDR2}" | tr -d '\r' || true)
ALL_X_HDRS=$(grep -i "^x-" "${HDR2}" | tr -d '\r' || true)
rm -f "${HDR2}"
info "  time: ${T2}ms"
[[ -n "${CACHE_HDR2}" ]] && info "  cache header: ${CACHE_HDR2}"
[[ -n "${ALL_X_HDRS}" ]]  && info "  all x-headers: ${ALL_X_HDRS}"

# Detect hit via explicit header or ≥3× speedup (LLM baseline > 500ms)
HIT_BY_HEADER=$(echo "${CACHE_HDR2}" | grep -i "hit" || true)
if [[ -n "${HIT_BY_HEADER}" ]]; then
  pass "Semantic cache HIT confirmed via response header."
elif [[ "${T1}" -gt 500 && "${T2}" -gt 0 && "${T2}" -lt $((T1 / 3)) ]]; then
  pass "Semantic cache HIT inferred: ${T1}ms → ${T2}ms ($(( T1 / T2 ))× speedup)."
else
  warn "Cache hit not detected. Times: ${T1}ms → ${T2}ms."
  warn "Check that embedding_provider_api_key is set in additional-config.toml."
fi

# ═══════════════════════════════════════════════════════════════════════════
# Test 3 — PII Masking
#   Sends a message containing a unique email and phone number, then asks
#   the model to repeat it back verbatim. With redactPII=true the gateway
#   replaces PII before forwarding to OpenAI, so the original values should
#   never appear in the response.
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "══════════════════════════════════════════════════"
echo " Test 3: PII Masking"
echo "══════════════════════════════════════════════════"

TEST_EMAIL="user.piitest.99887@never-real-domain.io"
TEST_PHONE="+15550198765"

PII_MSG="Repeat this back to me word for word: my email is ${TEST_EMAIL} and my phone is ${TEST_PHONE}."
info "Sending message containing email and phone ..."
info "  email : ${TEST_EMAIL}"
info "  phone : ${TEST_PHONE}"

HDR=$(mktemp)
RESP=$(chat_req "${PII_MSG}" "${HDR}")
rm -f "${HDR}"

if echo "${RESP}" | jq -e '.error' >/dev/null 2>&1; then
  fail "Request errored: $(echo "${RESP}" | jq -r '.error.message // "unknown error"')"
else
  CONTENT=$(echo "${RESP}" | jq -r '.choices[0].message.content // ""')
  info "  AI response: ${CONTENT}"

  EMAIL_LEAKED=false; PHONE_LEAKED=false
  echo "${CONTENT}" | grep -qF "${TEST_EMAIL}" && EMAIL_LEAKED=true
  echo "${CONTENT}" | grep -qF "${TEST_PHONE}"  && PHONE_LEAKED=true

  if [[ "${EMAIL_LEAKED}" == false && "${PHONE_LEAKED}" == false ]]; then
    pass "PII masked — original email and phone absent from AI response."
  else
    [[ "${EMAIL_LEAKED}" == true ]] && fail "Email NOT masked — '${TEST_EMAIL}' leaked into AI response."
    [[ "${PHONE_LEAKED}" == true ]] && fail "Phone NOT masked — '${TEST_PHONE}' leaked into AI response."
  fi
fi

# ═══════════════════════════════════════════════════════════════════════════
# Summary
# ═══════════════════════════════════════════════════════════════════════════
echo ""
echo "══════════════════════════════════════════════════"
if [[ "${FAILURES}" -eq 0 ]]; then
  echo -e "${GREEN}[PASS]${NC} All tests passed."
else
  echo -e "${RED}[FAIL]${NC} ${FAILURES} test(s) failed."
fi
echo "══════════════════════════════════════════════════"
echo ""
exit "${FAILURES}"
