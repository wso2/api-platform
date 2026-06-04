#!/usr/bin/env bash
# Test: Model Round Robin
#   Sends 3 requests and verifies the gateway cycles across different models.
#   The round-robin policy overrides the model field; the actual model used
#   is visible in OpenAI's response body.
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
echo " Test: Model Round Robin"
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

echo ""
echo "══════════════════════════════════════════════════"
UNIQUE=$(printf '%s\n' "${MODELS[@]}" | grep -v '^error$' | sort -u | wc -l | tr -d ' ')
if [[ "${UNIQUE}" -gt 1 ]]; then
  pass "Round robin confirmed — saw ${UNIQUE} different models across 3 requests."
  info "  Sequence: ${MODELS[0]} → ${MODELS[1]} → ${MODELS[2]}"
  echo "══════════════════════════════════════════════════"
  echo ""
  exit 0
else
  fail "Round robin not detected — all 3 requests used '${MODELS[0]:-unknown}'."
  echo "══════════════════════════════════════════════════"
  echo ""
  exit 1
fi
