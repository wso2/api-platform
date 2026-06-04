#!/usr/bin/env bash
# Test: PII Masking
#   Sends a message containing an email and phone number, then asks the model
#   to repeat it back verbatim. With redactPII=true the gateway replaces PII
#   before forwarding to OpenAI, so the original values should never appear
#   in the response.
#
#   Run interactively to provide your own email and phone, or pass them as
#   environment variables:
#     TEST_EMAIL="you@example.com" TEST_PHONE="+15551234567" ./test-pii-masking.sh
set -uo pipefail

CHAT_URL="http://localhost:8080/openai-proxy/chat/completions"
HEALTH_URL="http://localhost:9094/health"

GREEN="\033[0;32m"; RED="\033[0;31m"; BLUE="\033[0;34m"; NC="\033[0m"
pass() { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; }
info() { echo -e "${BLUE}[INFO]${NC} $*"; }

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

echo ""
echo "══════════════════════════════════════════════════"
echo " Test: PII Masking"
echo "══════════════════════════════════════════════════"

# Accept values from environment, prompt interactively, or fall back to defaults.
DEFAULT_EMAIL="user.piitest.99887@never-real-domain.io"
DEFAULT_PHONE="+15550198765"

if [[ -z "${TEST_EMAIL:-}" ]]; then
  read -rp "  Enter email to test [${DEFAULT_EMAIL}]: " TEST_EMAIL
  TEST_EMAIL="${TEST_EMAIL:-${DEFAULT_EMAIL}}"
fi

if [[ -z "${TEST_PHONE:-}" ]]; then
  read -rp "  Enter phone to test [${DEFAULT_PHONE}]: " TEST_PHONE
  TEST_PHONE="${TEST_PHONE:-${DEFAULT_PHONE}}"
fi

echo ""
info "Using email : ${TEST_EMAIL}"
info "Using phone : ${TEST_PHONE}"

PII_MSG="Repeat this back to me word for word: my email is ${TEST_EMAIL} and my phone is ${TEST_PHONE}."
info "Sending message containing email and phone ..."

HDR=$(mktemp)
RESP=$(chat_req "${PII_MSG}" "${HDR}")
rm -f "${HDR}"

echo ""
echo "══════════════════════════════════════════════════"
if echo "${RESP}" | jq -e '.error' >/dev/null 2>&1; then
  fail "Request errored: $(echo "${RESP}" | jq -r '.error.message // "unknown error"')"
  echo "══════════════════════════════════════════════════"
  echo ""
  exit 1
fi

CONTENT=$(echo "${RESP}" | jq -r '.choices[0].message.content // ""')
info "AI response: ${CONTENT}"
echo ""

EMAIL_LEAKED=false; PHONE_LEAKED=false
echo "${CONTENT}" | grep -qF "${TEST_EMAIL}" && EMAIL_LEAKED=true
echo "${CONTENT}" | grep -qF "${TEST_PHONE}"  && PHONE_LEAKED=true

FAILURES=0
if [[ "${EMAIL_LEAKED}" == true ]]; then
  fail "Email NOT masked — '${TEST_EMAIL}' leaked into AI response."
  FAILURES=$((FAILURES + 1))
fi
if [[ "${PHONE_LEAKED}" == true ]]; then
  fail "Phone NOT masked — '${TEST_PHONE}' leaked into AI response."
  FAILURES=$((FAILURES + 1))
fi
if [[ "${FAILURES}" -eq 0 ]]; then
  pass "PII masked — original email and phone absent from AI response."
fi

echo "══════════════════════════════════════════════════"
echo ""
exit "${FAILURES}"
