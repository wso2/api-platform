#!/usr/bin/env bash
#
# Verifies the analytics pipeline up to the point where events leave the
# gateway: the publisher is configured, the Application ID reached the
# container, and traffic through both proxies carries token usage.
#
# Run ./setup.sh first.
#
set -uo pipefail

DIST_VERSION="1.2.0"
DIST_NAME="wso2apip-ai-gateway-${DIST_VERSION}"

GATEWAY_HOST="${GATEWAY_HOST:-http://localhost:8080}"
HEALTH_URL="${HEALTH_URL:-http://localhost:9094/health}"
ASSISTANT_URL="${GATEWAY_HOST}/assistant/chat/completions"
SUPPORT_URL="${GATEWAY_HOST}/support/chat/completions"

ASSISTANT_API_KEY="${ASSISTANT_API_KEY:-demo-assistant-key}"
SUPPORT_API_KEY="${SUPPORT_API_KEY:-demo-support-key}"

# Referenced by the compose override; a placeholder silences Compose warnings.
export MOESIF_APPLICATION_ID="${MOESIF_APPLICATION_ID:-unset}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GATEWAY_CONFIG="${SCRIPT_DIR}/${DIST_NAME}/configs/config.toml"
PLACEHOLDER_ID="your-moesif-application-id"

GREEN="\033[0;32m"; RED="\033[0;31m"; BLUE="\033[0;34m"; NC="\033[0m"
pass() { printf '%b[PASS]%b %s\n' "${GREEN}" "${NC}" "$*"; }
fail() { printf '%b[FAIL]%b %s\n' "${RED}" "${NC}" "$*"; failures=$(( failures + 1 )); }
info() { printf '%b[INFO]%b %s\n' "${BLUE}" "${NC}" "$*"; }
abort() { printf '%b[ERROR]%b %s\n' "${RED}" "${NC}" "$*" >&2; exit 1; }

section() {
  echo ""
  echo "══════════════════════════════════════════════════"
  echo " $*"
  echo "══════════════════════════════════════════════════"
}

failures=0

section "Pre-flight"

command -v jq >/dev/null 2>&1 \
  || abort "jq is required (macOS: brew install jq, Debian/Ubuntu: apt install jq)."

curl -sf --connect-timeout 5 --max-time 10 "${HEALTH_URL}" >/dev/null 2>&1 \
  || abort "Gateway is not running. Run ./setup.sh first."
pass "Gateway is healthy."

[[ -f "${GATEWAY_CONFIG}" ]] \
  || abort "${GATEWAY_CONFIG} not found. Run ./setup.sh first."

RUNTIME_CID=$(cd "${SCRIPT_DIR}/${DIST_NAME}" && docker compose ps -q gateway-runtime 2>/dev/null | head -1)
[[ -n "${RUNTIME_CID}" ]] \
  || abort "The gateway-runtime container is not running."

# --- Test 1 ----------------------------------------------------------------
section "Test 1: Analytics is configured for Moesif"

if grep -q '^\[analytics\]' "${GATEWAY_CONFIG}"; then
  pass "[analytics] section present."
else
  fail "No [analytics] section in config.toml."
fi

if grep -q 'enabled_publishers[[:space:]]*=.*moesif' "${GATEWAY_CONFIG}"; then
  pass "moesif is an enabled publisher."
else
  fail "moesif is not listed in enabled_publishers."
fi

# --- Test 2 ----------------------------------------------------------------
section "Test 2: The Application ID reached the gateway"

RUNTIME_APP_ID=$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "${RUNTIME_CID}" \
  | grep '^APIP_GW_ANALYTICS_PUBLISHERS_MOESIF_APPLICATION_ID=' | cut -d= -f2- | head -1)

if [[ -z "${RUNTIME_APP_ID}" ]]; then
  fail "The Application ID is not set on gateway-runtime.
        Check that ${DIST_NAME}/docker-compose.override.yaml exists, then re-run ./setup.sh."
elif [[ "${RUNTIME_APP_ID}" == *"${PLACEHOLDER_ID}"* ]]; then
  fail "gateway-runtime is using the placeholder Application ID, so no events are sent.
        Set MOESIF_APPLICATION_ID, then run ./teardown.sh and ./setup.sh."
else
  pass "Application ID is set on gateway-runtime."
fi

# --- Test 3 ----------------------------------------------------------------
section "Test 3: Responses carry token usage"

# assert_tokens <label> <url> <api-key> <expected-model>
assert_tokens() {
  local label="$1" url="$2" key="$3" expected_model="$4"
  local payload body total model

  payload=$(jq -nc --arg m "${expected_model}" \
    '{model: $m, messages: [{role: "user", content: "token usage check"}]}')

  body=$(curl -s --connect-timeout 5 --max-time 30 -X POST "${url}" \
    -H "Content-Type: application/json" \
    -H "api_key: ${key}" \
    -d "${payload}" 2>/dev/null)

  total=$(jq -r '.usage.total_tokens // empty' <<< "${body}" 2>/dev/null)
  model=$(jq -r '.model // empty' <<< "${body}" 2>/dev/null)

  if [[ -z "${total}" ]]; then
    fail "${label}: no usage.total_tokens in the response. Gateway returned: ${body:0:200}"
  elif [[ ! "${total}" =~ ^[0-9]+$ ]]; then
    fail "${label}: usage.total_tokens was not a number: ${total}"
  elif (( total <= 0 )); then
    fail "${label}: usage.total_tokens was ${total}."
  elif [[ "${model}" != "${expected_model}" ]]; then
    fail "${label}: expected model ${expected_model}, got ${model}."
  else
    pass "${label}: ${total} tokens on ${model}."
  fi
}

assert_tokens "Assistant proxy" "${ASSISTANT_URL}" "${ASSISTANT_API_KEY}" "gpt-4o-mini"
assert_tokens "Support proxy"   "${SUPPORT_URL}"   "${SUPPORT_API_KEY}"   "gpt-4.1"

# --- Test 4 ----------------------------------------------------------------
section "Test 4: Unauthenticated requests are rejected"

assert_rejected() {
  local label="$1" url="$2" status
  status=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 --max-time 30 \
    -X POST "${url}" \
    -H "Content-Type: application/json" \
    -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"no key"}]}' 2>/dev/null)

  if [[ "${status}" == "401" ]]; then
    pass "${label}: rejected with HTTP 401."
  else
    fail "${label}: expected HTTP 401 without an API key, got ${status}."
  fi
}

assert_rejected "Assistant proxy" "${ASSISTANT_URL}"
assert_rejected "Support proxy"   "${SUPPORT_URL}"

# --- Summary ---------------------------------------------------------------
echo ""
echo "══════════════════════════════════════════════════"
if (( failures == 0 )); then
  pass "The gateway is configured to publish analytics, and traffic carries token usage."
  cat <<'EOF'

 Confirm delivery in Moesif: https://www.moesif.com

   LLM Analytics        Token Usage and Estimated Cost for the run
   Chart                metadata.aiTokenUsage.totalTokens, grouped by
                        metadata.aiMetadata.model
   Same chart on cost   metadata.aiMetadata.llmCost, where the split
                        between the two models looks very different
EOF
  echo "══════════════════════════════════════════════════"
  exit 0
else
  fail "${failures} check(s) failed."
  echo "══════════════════════════════════════════════════"
  exit 1
fi
