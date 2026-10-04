#!/usr/bin/env bash
#
# Generates traffic across both proxies so the analytics in Moesif have two
# models to compare.
#
#   ./load.sh        run for 60 seconds
#   ./load.sh 120    run for 120 seconds
#
set -uo pipefail

DURATION="${1:-${LOAD_DURATION:-60}}"
INTERVAL="${LOAD_INTERVAL:-0.25}"

GATEWAY_HOST="${GATEWAY_HOST:-http://localhost:8080}"
HEALTH_URL="${HEALTH_URL:-http://localhost:9094/health}"
ASSISTANT_URL="${GATEWAY_HOST}/assistant/chat/completions"
SUPPORT_URL="${GATEWAY_HOST}/support/chat/completions"

ASSISTANT_API_KEY="${ASSISTANT_API_KEY:-demo-assistant-key}"
SUPPORT_API_KEY="${SUPPORT_API_KEY:-demo-support-key}"

ASSISTANT_MODEL="gpt-4o-mini"
SUPPORT_MODEL="gpt-4.1"

GREEN="\033[0;32m"; RED="\033[0;31m"; BLUE="\033[0;34m"; NC="\033[0m"
info() { printf '%b[INFO]%b %s\n' "${BLUE}" "${NC}" "$*"; }
ok()   { printf '%b[OK]%b   %s\n' "${GREEN}" "${NC}" "$*"; }
fail() { printf '%b[ERROR]%b %s\n' "${RED}" "${NC}" "$*" >&2; }

command -v jq >/dev/null 2>&1 || { fail "jq is required."; exit 1; }

if ! curl -sf --connect-timeout 5 --max-time 10 "${HEALTH_URL}" >/dev/null 2>&1; then
  fail "Gateway is not running. Run ./setup.sh first."
  exit 1
fi

total=0
count_2xx=0
count_401=0
count_5xx=0
count_other=0
tokens_assistant=0
tokens_support=0

# send <url> <api-key> <model> <prompt> <token-bucket>
send() {
  local url="$1" key="$2" model="$3" prompt="$4" bucket="$5"
  local payload response status body tokens

  payload=$(jq -nc --arg m "${model}" --arg c "${prompt}" \
    '{model: $m, messages: [{role: "user", content: $c}]}')

  response=$(curl -s -w $'\n%{http_code}' --connect-timeout 5 --max-time 30 \
    -X POST "${url}" \
    -H "Content-Type: application/json" \
    -H "api_key: ${key}" \
    -d "${payload}" 2>/dev/null)

  status="${response##*$'\n'}"
  body="${response%$'\n'*}"
  total=$(( total + 1 ))

  case "${status}" in
    2*)
      count_2xx=$(( count_2xx + 1 ))
      tokens=$(jq -r '.usage.total_tokens // 0' <<< "${body}" 2>/dev/null)
      [[ "${tokens}" =~ ^[0-9]+$ ]] || tokens=0
      if [[ "${bucket}" == "assistant" ]]; then
        tokens_assistant=$(( tokens_assistant + tokens ))
      else
        tokens_support=$(( tokens_support + tokens ))
      fi
      ;;
    401) count_401=$(( count_401 + 1 )) ;;
    5*)  count_5xx=$(( count_5xx + 1 )) ;;
    *)   count_other=$(( count_other + 1 )) ;;
  esac
}

echo ""
echo "══════════════════════════════════════════════════"
echo " Generating traffic for ${DURATION}s"
echo "══════════════════════════════════════════════════"
info "Assistant : ${ASSISTANT_URL}   ${ASSISTANT_MODEL}"
info "Support   : ${SUPPORT_URL}     ${SUPPORT_MODEL}"
echo ""

end=$(( $(date +%s) + DURATION ))
i=0

# A fixed ten-request cycle keeps the ratios identical on every run: four calls
# per model, one upstream failure, one rejected key.
while [[ $(date +%s) -lt ${end} ]]; do
  i=$(( i + 1 ))
  case $(( i % 10 )) in
    6)       send "${ASSISTANT_URL}" "${ASSISTANT_API_KEY}" "${ASSISTANT_MODEL}" "FAIL - simulated upstream error" "assistant" ;;
    9)       send "${ASSISTANT_URL}" "invalid-key"          "${ASSISTANT_MODEL}" "Rejected before reaching the model" "assistant" ;;
    0|2|4|8) send "${SUPPORT_URL}"   "${SUPPORT_API_KEY}"   "${SUPPORT_MODEL}"   "Draft a reply to a customer complaint" "support" ;;
    *)       send "${ASSISTANT_URL}" "${ASSISTANT_API_KEY}" "${ASSISTANT_MODEL}" "Give me a one-line status update" "assistant" ;;
  esac

  if (( i % 10 == 0 )); then
    printf '  sent %3d requests, %ds left\n' "${total}" "$(( end - $(date +%s) ))"
  fi
  sleep "${INTERVAL}"
done

echo ""
echo "══════════════════════════════════════════════════"
echo " Done: ${total} requests"
echo "══════════════════════════════════════════════════"
printf '  2xx  success          %5d\n' "${count_2xx}"
printf '  401  key rejected     %5d\n' "${count_401}"
printf '  5xx  upstream failure %5d\n' "${count_5xx}"
(( count_other > 0 )) && printf '  other                 %5d\n' "${count_other}"
echo ""
printf '  tokens, %-12s %5d\n' "${ASSISTANT_MODEL}" "${tokens_assistant}"
printf '  tokens, %-12s %5d\n' "${SUPPORT_MODEL}"   "${tokens_support}"
echo ""
ok "Moesif should show these totals per model within a few seconds."
echo "   https://www.moesif.com"
echo ""
echo " To verify from the terminal instead: ./test.sh"
echo ""
