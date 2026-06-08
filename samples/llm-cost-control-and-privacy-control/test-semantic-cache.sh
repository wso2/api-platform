#!/usr/bin/env bash
# Test: Semantic Cache
#   Sends the same question twice. The second request should be served from
#   the semantic cache (Redis + Mistral embeddings), confirmed via a cache
#   response header or a significantly faster response time.
#
#   NOTE: requires embedding_provider_api_key in additional-config.toml.
set -uo pipefail

CHAT_URL="http://localhost:8080/openai-proxy/chat/completions"
HEALTH_URL="http://localhost:9094/health"

GREEN="\033[0;32m"; RED="\033[0;31m"; BLUE="\033[0;34m"; YELLOW="\033[0;33m"; NC="\033[0m"
pass() { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; }
info() { echo -e "${BLUE}[INFO]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }

ms() { python3 -c "import time; print(int(time.time()*1000))"; }

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
echo " Test: Semantic Cache"
echo "══════════════════════════════════════════════════"

CACHE_MSG="What is the boiling point of water in Celsius?"
info "Using question: \"${CACHE_MSG}\""

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

echo ""
echo "══════════════════════════════════════════════════"
HIT_BY_HEADER=$(echo "${CACHE_HDR2}" | grep -i "hit" || true)
if [[ -n "${HIT_BY_HEADER}" ]]; then
  pass "Semantic cache HIT confirmed via response header."
  echo "══════════════════════════════════════════════════"
  echo ""
  exit 0
elif [[ "${T1}" -gt 500 && "${T2}" -gt 0 && "${T2}" -lt $((T1 / 3)) ]]; then
  pass "Semantic cache HIT inferred: ${T1}ms → ${T2}ms ($(( T1 / T2 ))× speedup)."
  echo "══════════════════════════════════════════════════"
  echo ""
  exit 0
else
  warn "Cache hit not detected. Times: ${T1}ms → ${T2}ms."
  warn "Check that embedding_provider_api_key is set in additional-config.toml."
  echo "══════════════════════════════════════════════════"
  echo ""
  exit 1
fi
