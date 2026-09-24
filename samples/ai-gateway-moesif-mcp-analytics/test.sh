#!/usr/bin/env bash
#
# Verifies the analytics pipeline up to the point where events leave the
# gateway: the publisher is configured, the Application ID and body capture
# reached the running container, and MCP traffic behaves as the sample
# describes.
#
# Run ./setup.sh first.
#
set -uo pipefail

DIST_VERSION="1.2.0"
DIST_NAME="wso2apip-ai-gateway-${DIST_VERSION}"

GATEWAY_HOST="${GATEWAY_HOST:-http://localhost:8080}"
HEALTH_URL="${HEALTH_URL:-http://localhost:9094/health}"
TOOLBOX_URL="${GATEWAY_HOST}/toolbox/mcp"
METERED_URL="${GATEWAY_HOST}/metered/mcp"

# Referenced by the compose override; a placeholder silences Compose warnings.
export MOESIF_APPLICATION_ID="${MOESIF_APPLICATION_ID:-unset}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=token.sh
source "${SCRIPT_DIR}/token.sh"

GATEWAY_CONFIG="${SCRIPT_DIR}/${DIST_NAME}/configs/config.toml"
PUBLIC_KEY="${SCRIPT_DIR}/keys/public.pem"
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
HDR_FILE="$(mktemp)"
BODY_FILE="$(mktemp)"
trap 'rm -f "${HDR_FILE}" "${BODY_FILE}"' EXIT

# Replies arrive as server-sent events, so the JSON payload follows "data: ".
payload_of() {
  local p
  p=$(sed -n 's/^data: //p' "$1" | head -1)
  [[ -n "${p}" ]] && echo "${p}" || tr -d '\n' < "$1"
}

section "Pre-flight"

for c in jq openssl; do
  command -v "$c" >/dev/null 2>&1 || abort "$c is required."
done

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
section "Test 3: Body capture is live in the running gateway"

info "MCP detail is read from the JSON-RPC bodies. Without body capture the"
info "events still reach Moesif, but the MCP dashboard stays empty."

if grep -q '^\[collector\]' "${GATEWAY_CONFIG}"; then
  pass "[collector] is present in config.toml."
else
  fail "[collector] is missing from config.toml. Re-run ./setup.sh."
fi

CONTAINER_CONFIG=$(cd "${SCRIPT_DIR}/${DIST_NAME}" \
  && docker compose exec -T gateway-runtime cat /etc/policy-engine/config.toml 2>/dev/null)

if [[ -z "${CONTAINER_CONFIG}" ]]; then
  info "Could not read the config inside the container; skipping that check."
elif grep -q 'request_body[[:space:]]*=[[:space:]]*true' <<< "${CONTAINER_CONFIG}" \
  && grep -q 'response_body[[:space:]]*=[[:space:]]*true' <<< "${CONTAINER_CONFIG}"; then
  pass "The running gateway has body capture switched on."
else
  fail "The running gateway does not have body capture.
        The config was edited but the container was not restarted.
        Run ./teardown.sh and ./setup.sh."
fi

# --- Test 4 ----------------------------------------------------------------
section "Test 4: The gateway trusts the sample's signing key"

if [[ ! -f "${PUBLIC_KEY}" ]]; then
  fail "No public key at keys/public.pem. Re-run ./setup.sh."
elif ! grep -q 'jwtauth_v1.keymanagers' "${GATEWAY_CONFIG}"; then
  fail "No key manager in config.toml. Re-run ./setup.sh."
else
  pass "A key manager is configured."
  # The key in the config has to be the one the scripts sign with, otherwise
  # every token is rejected for no obvious reason.
  KEY_LINE=$(sed -n '2p' "${PUBLIC_KEY}")
  if grep -qF "${KEY_LINE}" "${GATEWAY_CONFIG}"; then
    pass "The configured key matches keys/public.pem."
  else
    fail "The key in config.toml is not the one in keys/public.pem, so tokens
        will be rejected. Run ./teardown.sh --clean and ./setup.sh."
  fi
fi

# --- Test 5 ----------------------------------------------------------------
section "Test 5: Calls without a token are refused"

STATUS=$(curl -s -o "${BODY_FILE}" -w '%{http_code}' --connect-timeout 5 --max-time 20 \
  -X POST "${TOOLBOX_URL}" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"sample-test","version":"1.0.0"}}}' \
  2>/dev/null)

if [[ "${STATUS}" == "401" || "${STATUS}" == "403" ]]; then
  pass "Rejected with HTTP ${STATUS} before reaching the server."
else
  fail "Expected HTTP 401 or 403 without a token, got ${STATUS}."
fi

# --- Test 6 ----------------------------------------------------------------
section "Test 6: The MCP handshake works with a token"

TEST_TOKEN=$(mint_token "sample-test@example.com") \
  || abort "Could not mint a token. Run ./setup.sh first."
info "Signed a token for sample-test@example.com"

# handshake <url> -> prints the session id
handshake() {
  local url="$1" sid status
  curl -s -D "${HDR_FILE}" -o /dev/null --connect-timeout 5 --max-time 20 \
    -X POST "${url}" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -H "Authorization: Bearer ${TEST_TOKEN}" \
    -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"sample-test","version":"1.0.0"}}}' \
    2>/dev/null
  sid=$(grep -i '^mcp-session-id:' "${HDR_FILE}" | tr -d '\r' | awk '{print $2}')
  [[ -n "${sid}" ]] || return 1
  status=$(curl -s -o /dev/null -w '%{http_code}' --connect-timeout 5 --max-time 20 \
    -X POST "${url}" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -H "Authorization: Bearer ${TEST_TOKEN}" \
    -H "mcp-session-id: ${sid}" \
    -d '{"jsonrpc":"2.0","method":"notifications/initialized"}' 2>/dev/null)
  [[ "${status}" == 2* ]] || return 1
  echo "${sid}"
}

TOOLBOX_SID=$(handshake "${TOOLBOX_URL}")
if [[ -n "${TOOLBOX_SID}" ]]; then
  pass "Toolbox proxy accepted the token and returned a session id."
else
  fail "Toolbox proxy returned no session id, so no other MCP call can work."
fi

METERED_SID=$(handshake "${METERED_URL}")
if [[ -n "${METERED_SID}" ]]; then
  pass "Metered proxy accepted the token and returned a session id."
else
  fail "Metered proxy returned no session id."
fi

# --- Test 7 ----------------------------------------------------------------
section "Test 7: Allowed tools work, and only the allowed ones are listed"

# call <url> <session-id> <method> <params-json>; prints the http status
call() {
  local url="$1" sid="$2" method="$3" params="$4"
  curl -s -o "${BODY_FILE}" -w '%{http_code}' \
    --connect-timeout 5 --max-time 30 \
    -X POST "${url}" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -H "Authorization: Bearer ${TEST_TOKEN}" \
    -H "mcp-session-id: ${sid}" \
    -d "$(jq -nc --arg m "${method}" --argjson p "${params}" \
        '{jsonrpc:"2.0", id:99, method:$m, params:$p}')" 2>/dev/null
}

if [[ -n "${TOOLBOX_SID}" ]]; then
  call "${TOOLBOX_URL}" "${TOOLBOX_SID}" "tools/call" \
    '{"name":"get-sum","arguments":{"a":2,"b":3}}' >/dev/null
  TEXT=$(payload_of "${BODY_FILE}" | jq -r '.result.content[0].text // empty' 2>/dev/null)
  if [[ "${TEXT}" == *"5"* ]]; then
    pass "get-sum returned a result through the toolbox proxy."
  else
    fail "get-sum did not return a usable result. Gateway returned: $(payload_of "${BODY_FILE}" | head -c 200)"
  fi

  # assert_tool_count <label> <url> <session-id> <expected>
  assert_tool_count() {
    local label="$1" url="$2" sid="$3" expected="$4" names count
    call "${url}" "${sid}" "tools/list" '{}' >/dev/null
    names=$(payload_of "${BODY_FILE}" | jq -r '[.result.tools[]?.name] | join(", ")' 2>/dev/null)
    count=$(payload_of "${BODY_FILE}" | jq -r '[.result.tools[]?.name] | length' 2>/dev/null)
    if [[ "${count}" == "${expected}" ]]; then
      pass "${label}: ${count} tools listed (${names})."
    else
      fail "${label}: expected ${expected} tools, got ${count:-none} (${names:-none}).
        The allowlist should filter the list response as well as the calls."
    fi
  }

  assert_tool_count "Toolbox proxy" "${TOOLBOX_URL}" "${TOOLBOX_SID}" 4
  [[ -n "${METERED_SID}" ]] \
    && assert_tool_count "Metered proxy" "${METERED_URL}" "${METERED_SID}" 2
fi

# --- Test 8 ----------------------------------------------------------------
section "Test 8: Tools outside the allowlist are refused"

if [[ -n "${TOOLBOX_SID}" ]]; then
  STATUS=$(call "${TOOLBOX_URL}" "${TOOLBOX_SID}" "tools/call" \
    '{"name":"get-env","arguments":{}}')
  if [[ "${STATUS}" == 4* ]]; then
    pass "get-env was refused with HTTP ${STATUS} before reaching the server."
  else
    fail "get-env returned HTTP ${STATUS}; expected a 4xx from the allowlist."
  fi
fi

# --- Summary ---------------------------------------------------------------
echo ""
echo "══════════════════════════════════════════════════"
if (( failures == 0 )); then
  pass "The gateway publishes MCP analytics, and the proxies behave as described."
  cat <<'EOF'

 Confirm delivery in Moesif: https://www.moesif.com

   Analytics > MCP      set the range to the last hour
   Top Tools by Calls   the tools ./load.sh exercised
   Unique Sessions      one per client that ran the handshake
   Client Distribution  the clientInfo name each client sent
   Unique Consumers     the subject inside each token
EOF
  echo "══════════════════════════════════════════════════"
  exit 0
else
  fail "${failures} check(s) failed."
  echo "══════════════════════════════════════════════════"
  exit 1
fi
