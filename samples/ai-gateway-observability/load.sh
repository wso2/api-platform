#!/usr/bin/env bash
# Generates about a minute of mixed traffic through both proxies so the dashboard
# has something real to show: steady successes, a slow tail, upstream failures,
# rejected keys, and — once the support proxy's token budget runs out — 429s.
#
#   ./load.sh          # default 60 seconds
#   ./load.sh 120      # run for 120 seconds
set -uo pipefail

DURATION="${1:-${LOAD_DURATION:-60}}"
# One key per proxy.
ASSISTANT_API_KEY="${ASSISTANT_API_KEY:-demo-assistant-key}"
SUPPORT_API_KEY="${SUPPORT_API_KEY:-demo-support-key}"
GATEWAY_HOST="${GATEWAY_HOST:-http://localhost:8080}"
ASSISTANT_URL="${GATEWAY_HOST}/assistant/chat/completions"
SUPPORT_URL="${GATEWAY_HOST}/support/chat/completions"
HEALTH_URL="${HEALTH_URL:-http://localhost:9094/health}"
INTERVAL="${LOAD_INTERVAL:-0.25}"   # seconds between requests

GREEN="\033[0;32m"; RED="\033[0;31m"; BLUE="\033[0;34m"; NC="\033[0m"
info() { printf '%b[INFO]%b %s\n' "${BLUE}" "${NC}" "$*"; }
ok()   { printf '%b[OK]%b   %s\n' "${GREEN}" "${NC}" "$*"; }
fail() { printf '%b[ERROR]%b %s\n' "${RED}" "${NC}" "$*" >&2; }

if ! curl -sf "${HEALTH_URL}" >/dev/null 2>&1; then
  fail "Gateway is not running. Run ./setup.sh first."
  exit 1
fi

TOTAL=0
COUNT_2XX=0
COUNT_401=0
COUNT_429=0
COUNT_5XX=0
COUNT_OTHER=0

send() {
  local url="$1" key="$2" prompt="$3"
  local body status
  body=$(printf '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"%s"}]}' "${prompt}")
  status=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${url}" \
    -H "Content-Type: application/json" \
    -H "api_key: ${key}" \
    -d "${body}" 2>/dev/null)
  TOTAL=$(( TOTAL + 1 ))
  case "${status}" in
    2*)  COUNT_2XX=$(( COUNT_2XX + 1 )) ;;
    401) COUNT_401=$(( COUNT_401 + 1 )) ;;
    429) COUNT_429=$(( COUNT_429 + 1 )) ;;
    5*)  COUNT_5XX=$(( COUNT_5XX + 1 )) ;;
    *)   COUNT_OTHER=$(( COUNT_OTHER + 1 )) ;;
  esac
}

echo ""
echo "══════════════════════════════════════════════════"
echo " Generating traffic for ${DURATION}s"
echo "══════════════════════════════════════════════════"
info "Assistant : ${ASSISTANT_URL}"
info "Support   : ${SUPPORT_URL}   (token budget — expect 429s part-way through)"
echo ""

END=$(( $(date +%s) + DURATION ))
i=0

while [[ $(date +%s) -lt ${END} ]]; do
  i=$(( i + 1 ))

  # Mostly ordinary traffic, with a slow reply, an upstream failure and a rejected
  # key mixed in so every panel on the dashboard has a line.
  case $(( i % 10 )) in
    3) send "${ASSISTANT_URL}" "${ASSISTANT_API_KEY}" "SLOW — summarise the quarterly report" ;;
    6) send "${ASSISTANT_URL}" "${ASSISTANT_API_KEY}" "FAIL — this one breaks upstream" ;;
    9) send "${ASSISTANT_URL}" "not-a-valid-key"      "Rejected before it reaches the model" ;;
    0|2|4|8) send "${SUPPORT_URL}" "${SUPPORT_API_KEY}" "How do I reset my password?" ;;
    *) send "${ASSISTANT_URL}" "${ASSISTANT_API_KEY}" "Give me a one-line status update" ;;
  esac

  if (( i % 10 == 0 )); then
    printf '  sent %3d requests — %ds left\n' "${TOTAL}" "$(( END - $(date +%s) ))"
  fi
  sleep "${INTERVAL}"
done

echo ""
echo "══════════════════════════════════════════════════"
echo " Done — ${TOTAL} requests"
echo "══════════════════════════════════════════════════"
printf '  2xx  success           %4d\n' "${COUNT_2XX}"
printf '  401  key rejected      %4d\n' "${COUNT_401}"
printf '  429  token budget spent %4d\n' "${COUNT_429}"
printf '  5xx  upstream failure  %4d\n' "${COUNT_5XX}"
[[ "${COUNT_OTHER}" -gt 0 ]] && printf '  other                  %4d\n' "${COUNT_OTHER}"
echo ""
ok "Now open the dashboard and a trace:"
echo "   Grafana : http://localhost:3000   (admin / admin)"
echo "   Jaeger  : http://localhost:16686  — pick the router service, then any trace"
echo ""
echo " Or verify the same thing from the terminal: ./test.sh"
echo ""
