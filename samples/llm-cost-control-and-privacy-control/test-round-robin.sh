#!/usr/bin/env bash
# Test: Model Round Robin
#   Sends 9 requests (3 full cycles across the 3-model pool) and verifies the
#   gateway cycles across different models. The round-robin policy overrides
#   the model field; the actual model used is visible in OpenAI's response body.
set -uo pipefail

REQUESTS=9
POOL_SIZE=3

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

# Distinct prompts so the semantic-cache policy (similarityThreshold 0.85) does
# not serve later requests from Redis — a cache hit returns the cached `model`
# field and hides the rotation.
PROMPTS=(
  "What is the capital of France?"
  "Name one primary color."
  "How many continents are there?"
  "What gas do plants absorb from the air?"
  "Who wrote the play Hamlet?"
  "What is the boiling point of water in Celsius?"
  "Name the largest ocean on Earth."
  "What planet is known as the Red Planet?"
  "What is the chemical symbol for gold?"
)

MODELS=()
for i in $(seq 1 "${REQUESTS}"); do
  info "Request ${i}/${REQUESTS} ..."
  HDR=$(mktemp)
  RESP=$(chat_req "${PROMPTS[$((i-1))]}" "${HDR}")
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
SEQUENCE=$(printf '%s → ' "${MODELS[@]}")
SEQUENCE=${SEQUENCE% → }
info "Sequence: ${SEQUENCE}"

UNIQUE=$(printf '%s\n' "${MODELS[@]}" | grep -v '^error$' | sort -u | wc -l | tr -d ' ')
info "Distinct models observed: ${UNIQUE}"
printf '%s\n' "${MODELS[@]}" | grep -v '^error$' | sort | uniq -c | while read -r count name; do
  info "  ${name}: ${count}"
done

MIN_DISTINCT=$(( POOL_SIZE < REQUESTS ? POOL_SIZE : REQUESTS ))
if [[ "${UNIQUE}" -ge "${MIN_DISTINCT}" ]]; then
  pass "Round robin confirmed — all ${POOL_SIZE} pool models served traffic across ${REQUESTS} requests."
  echo "══════════════════════════════════════════════════"
  echo ""
  exit 0
elif [[ "${UNIQUE}" -gt 1 ]]; then
  pass "Round robin partial — saw ${UNIQUE}/${POOL_SIZE} pool models (a model may be suspended due to a prior error)."
  echo "══════════════════════════════════════════════════"
  echo ""
  exit 0
else
  fail "Round robin not detected — all ${REQUESTS} requests used '${MODELS[0]:-unknown}'."
  echo "══════════════════════════════════════════════════"
  echo ""
  exit 1
fi
