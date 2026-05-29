#!/bin/sh
set -eu

# Initialise tput colors if available
if command -v tput >/dev/null 2>&1 && [ -n "${TERM:-}" ] && tput setaf 2 >/dev/null 2>&1; then
  GREEN="$(tput setaf 2)"
  RED="$(tput setaf 1)"
  BOLD="$(tput bold)"
  RESET="$(tput sgr0)"
else
  GREEN=""; RED=""; BOLD=""; RESET=""
fi

print_ok() {
  echo "${GREEN}✔  $1${RESET}"
}

print_error() {
  echo "${RED}✖  $1${RESET}"
}

print_title() {
  echo ""
  echo "${BOLD}${GREEN}=== $1 ===${RESET}"
  echo ""
}

print_result() {
  echo "${BOLD}$1${RESET}"
}

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ -f "${SCRIPT_DIR}/.env" ]; then
  . "${SCRIPT_DIR}/.env"
fi

TRAFFIC_PORT="${TRAFFIC_PORT:-8443}"
INBOUND_API_KEY="${INBOUND_API_KEY:-demo-unlocked-sample-key}"
TARGET="https://localhost:${TRAFFIC_PORT}/assistant/chat/completions"
PAYLOAD='{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Test call"}]}'

# Mask API key for display — show prefix and last 4 chars
KEY_MASKED="$(echo "$INBOUND_API_KEY" | sed 's/\(.\{4\}\).*/\1/') ****"

print_title "WSO2 AI Gateway — LLM Proxy Test"
echo "Target  : ${TARGET}"
echo "API Key : ${KEY_MASKED}"
echo "Payload : ${PAYLOAD}"

print_title "Test 1: Valid API key — expect HTTP 200"
echo "Running:"
echo "  curl -sk -X POST ${TARGET} \\"
echo "    -H 'api_key: ${KEY_MASKED}' \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '${PAYLOAD}'"
echo ""

FULL_1=$(curl -sk -w "\n%{http_code}" -X POST "$TARGET" \
  -H "api_key: ${INBOUND_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD") || {
  print_error "Could not reach gateway at ${TARGET}. Is the gateway running? Run: sh run.sh"
  exit 1
}
STATUS_1=$(echo "$FULL_1" | tail -1)
BODY_1=$(echo "$FULL_1" | sed '$d')

echo "Response : ${BODY_1}"
echo "Status   : HTTP ${STATUS_1}"
if [ "$STATUS_1" -eq 200 ]; then
  print_ok "HTTP 200 received as expected"
else
  print_error "Expected HTTP 200, got ${STATUS_1}"
fi

print_title "Test 2: Token quota exceeded — expect HTTP 429"
echo "Test 1 consumed the full 30-token quota."
echo "This call is made immediately after with the same key."
echo ""
echo "Running:"
echo "  curl -sk -X POST ${TARGET} \\"
echo "    -H 'api_key: ${KEY_MASKED}' \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '${PAYLOAD}'"
echo ""

FULL_2=$(curl -sk -w "\n%{http_code}" -X POST "$TARGET" \
  -H "api_key: ${INBOUND_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD") || {
  print_error "Could not reach gateway at ${TARGET}. Is the gateway running? Run: sh run.sh"
  exit 1
}
STATUS_2=$(echo "$FULL_2" | tail -1)
BODY_2=$(echo "$FULL_2" | sed '$d')

echo "Response : ${BODY_2}"
echo "Status   : HTTP ${STATUS_2}"
if [ "$STATUS_2" -eq 429 ]; then
  print_ok "HTTP 429 received as expected"
else
  print_error "Expected HTTP 429, got ${STATUS_2}"
fi

echo ""
if [ "$STATUS_1" -eq 200 ] && [ "$STATUS_2" -eq 429 ]; then
  print_result "✔  PASSED — Rate limiting is working correctly."
  exit 0
else
  print_result "✖  FAILED — Expected HTTP 200 then HTTP 429, got ${STATUS_1} then ${STATUS_2}."
  echo ""
  echo "Troubleshooting:"
  echo "  - Check setup completed: cd wso2apip-ai-gateway-1.1.0 && docker compose logs"
  echo "  - Verify containers are up: docker ps"
  echo "  - Re-run setup: sh run.sh"
  exit 1
fi
