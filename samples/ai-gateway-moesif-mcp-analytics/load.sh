#!/usr/bin/env bash
#
# Generates MCP traffic through both proxies so the Moesif MCP dashboard has
# something to show.
#
#   ./load.sh        run for 60 seconds
#   ./load.sh 120    run for 120 seconds
#
# Two things about MCP shape this script.
#
# First, a tool call is never a single request. The client introduces itself
# with "initialize", the server answers with a session id in the
# mcp-session-id response header, and every later call sends that id back.
#
# Second, the proxies require an access token, and the subject inside the
# token becomes the consumer name in analytics. So this script opens three
# sessions, each with its own client name and its own token subject, and they
# appear as three clients and three consumers.
#
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=token.sh
source "${SCRIPT_DIR}/token.sh"

DURATION="${1:-${LOAD_DURATION:-60}}"
INTERVAL="${LOAD_INTERVAL:-0.25}"

GATEWAY_HOST="${GATEWAY_HOST:-http://localhost:8080}"
HEALTH_URL="${HEALTH_URL:-http://localhost:9094/health}"
TOOLBOX_URL="${GATEWAY_HOST}/toolbox/mcp"
METERED_URL="${GATEWAY_HOST}/metered/mcp"

# Reported to Moesif as the calling application.
APP_NAME="${APP_NAME:-mcp-analytics-sample}"
APP_ID="${APP_ID:-mcp-analytics-sample}"

GREEN="\033[0;32m"; RED="\033[0;31m"; BLUE="\033[0;34m"; NC="\033[0m"
info() { printf '%b[INFO]%b %s\n' "${BLUE}" "${NC}" "$*"; }
ok()   { printf '%b[OK]%b   %s\n' "${GREEN}" "${NC}" "$*"; }
fail() { printf '%b[ERROR]%b %s\n' "${RED}" "${NC}" "$*" >&2; }

command -v jq >/dev/null 2>&1 || { fail "jq is required."; exit 1; }
command -v openssl >/dev/null 2>&1 || { fail "openssl is required."; exit 1; }

if ! curl -sf --connect-timeout 5 --max-time 10 "${HEALTH_URL}" >/dev/null 2>&1; then
  fail "Gateway is not running. Run ./setup.sh first."
  exit 1
fi

BODY_FILE="$(mktemp)"
HDR_FILE="$(mktemp)"
trap 'rm -f "${BODY_FILE}" "${HDR_FILE}"' EXIT

count_ok=0
count_tool_error=0
count_blocked=0
count_limited=0
count_unauth=0
count_other=0
total=0
rpc_id=0

# open_session <url> <client-name> <token>
# Runs the two-step handshake and prints the session id. Retries a few times,
# because a proxy registered moments ago may not be serving yet.
open_session() {
  local url="$1" client="$2" token="$3" sid attempt
  for attempt in 1 2 3 4 5; do
    sid=$(open_session_once "$@") && { echo "${sid}"; return 0; }
    sleep 2
  done
  return 1
}

open_session_once() {
  local url="$1" client="$2" token="$3" sid status

  curl -s -D "${HDR_FILE}" -o /dev/null --connect-timeout 5 --max-time 20 \
    -X POST "${url}" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -H "Authorization: Bearer ${token}" \
    -H "x-wso2-application-name: ${APP_NAME}" \
    -H "x-wso2-application-id: ${APP_ID}" \
    -d "$(jq -nc --arg c "${client}" \
        '{jsonrpc:"2.0", id:1, method:"initialize", params:{
            protocolVersion:"2025-06-18", capabilities:{},
            clientInfo:{name:$c, version:"1.0.0"}}}')" 2>/dev/null

  sid=$(grep -i '^mcp-session-id:' "${HDR_FILE}" | tr -d '\r' | awk '{print $2}')
  [[ -n "${sid}" ]] || return 1

  # The server expects this notification before it accepts other calls, so a
  # session is only usable once it has been accepted.
  status=$(curl -s -o /dev/null -w '%{http_code}' --connect-timeout 5 --max-time 20 \
    -X POST "${url}" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -H "Authorization: Bearer ${token}" \
    -H "mcp-session-id: ${sid}" \
    -d '{"jsonrpc":"2.0","method":"notifications/initialized"}' 2>/dev/null)
  [[ "${status}" == 2* ]] || return 1

  echo "${sid}"
}

# send <url> <session-id> <token-or-empty> <method> <params-json>
send() {
  local url="$1" sid="$2" token="$3" method="$4" params="$5" status payload
  rpc_id=$(( rpc_id + 1 ))

  local auth_header="Authorization: Bearer ${token}"
  [[ -z "${token}" ]] && auth_header="X-No-Token: 1"

  status=$(curl -s -o "${BODY_FILE}" -w '%{http_code}' \
    --connect-timeout 5 --max-time 30 \
    -X POST "${url}" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -H "${auth_header}" \
    -H "mcp-session-id: ${sid}" \
    -H "x-wso2-application-name: ${APP_NAME}" \
    -H "x-wso2-application-id: ${APP_ID}" \
    -d "$(jq -nc --arg m "${method}" --argjson p "${params}" --argjson i "${rpc_id}" \
        '{jsonrpc:"2.0", id:$i, method:$m, params:$p}')" 2>/dev/null)

  total=$(( total + 1 ))

  # Replies arrive as server-sent events, so the JSON payload is the part of
  # the body that follows "data: ".
  payload=$(sed -n 's/^data: //p' "${BODY_FILE}" | head -1)
  [[ -n "${payload}" ]] || payload=$(tr -d '\n' < "${BODY_FILE}")

  case "${status}" in
    2*)
      if jq -e '.result.isError == true' >/dev/null 2>&1 <<< "${payload}"; then
        count_tool_error=$(( count_tool_error + 1 ))
      else
        count_ok=$(( count_ok + 1 ))
      fi
      ;;
    401|403) count_unauth=$(( count_unauth + 1 )) ;;
    429)     count_limited=$(( count_limited + 1 )) ;;
    4*)      count_blocked=$(( count_blocked + 1 )) ;;
    *)       count_other=$(( count_other + 1 )) ;;
  esac
}

echo ""
echo "══════════════════════════════════════════════════"
echo " Minting tokens and opening MCP sessions"
echo "══════════════════════════════════════════════════"

# Two people and a service account, so the consumer panel has a realistic mix.
TOKEN_ALICE=$(mint_token "alice@example.com") || exit 1
TOKEN_BOB=$(mint_token "bob@example.com")     || exit 1
TOKEN_BATCH=$(mint_token "svc-nightly-batch") || exit 1

SID_DESKTOP=$(open_session "${TOOLBOX_URL}" "desktop-client" "${TOKEN_ALICE}") \
  || { fail "Could not open a session on ${TOOLBOX_URL}. Run ./test.sh to see why."; exit 1; }
SID_CONSOLE=$(open_session "${TOOLBOX_URL}" "web-console" "${TOKEN_BOB}") \
  || { fail "Could not open a session on ${TOOLBOX_URL}."; exit 1; }
SID_BATCH=$(open_session "${METERED_URL}" "batch-agent" "${TOKEN_BATCH}") \
  || { fail "Could not open a session on ${METERED_URL}."; exit 1; }

ok "desktop-client on the toolbox proxy, as alice@example.com"
ok "web-console   on the toolbox proxy, as bob@example.com"
ok "batch-agent   on the metered proxy, as svc-nightly-batch"

echo ""
echo "══════════════════════════════════════════════════"
echo " Generating traffic for ${DURATION}s"
echo "══════════════════════════════════════════════════"
info "Toolbox : ${TOOLBOX_URL}"
info "Metered : ${METERED_URL}"
echo ""

end=$(( $(date +%s) + DURATION ))
i=0

# A fixed twelve-step cycle, so every run produces the same mix.
while [[ $(date +%s) -lt ${end} ]]; do
  i=$(( i + 1 ))
  case $(( i % 12 )) in
    1)  send "${TOOLBOX_URL}" "${SID_DESKTOP}" "${TOKEN_ALICE}" "tools/call" \
          '{"name":"echo","arguments":{"message":"status check"}}' ;;
    2)  send "${TOOLBOX_URL}" "${SID_DESKTOP}" "${TOKEN_ALICE}" "tools/call" \
          '{"name":"get-sum","arguments":{"a":12,"b":30}}' ;;
    3)  send "${TOOLBOX_URL}" "${SID_DESKTOP}" "${TOKEN_ALICE}" "tools/call" \
          '{"name":"get-structured-content","arguments":{"location":"Chicago"}}' ;;
    4)  send "${TOOLBOX_URL}" "${SID_CONSOLE}" "${TOKEN_BOB}" "tools/call" \
          '{"name":"get-resource-reference","arguments":{"resourceType":"Text","resourceId":3}}' ;;
    # Valid tool, invalid argument. The server runs it and reports a failure,
    # which is what fills Tool Call Execution Errors.
    5)  send "${TOOLBOX_URL}" "${SID_DESKTOP}" "${TOKEN_ALICE}" "tools/call" \
          '{"name":"get-sum","arguments":{"a":"twelve","b":30}}' ;;
    # Not on the allowlist, so the gateway refuses it before the server sees it.
    6)  send "${TOOLBOX_URL}" "${SID_CONSOLE}" "${TOKEN_BOB}" "tools/call" \
          '{"name":"get-env","arguments":{}}' ;;
    7)  send "${METERED_URL}" "${SID_BATCH}" "${TOKEN_BATCH}" "tools/call" \
          '{"name":"echo","arguments":{"message":"batch item"}}' ;;
    8)  send "${METERED_URL}" "${SID_BATCH}" "${TOKEN_BATCH}" "tools/call" \
          '{"name":"get-sum","arguments":{"a":7,"b":35}}' ;;
    9)  send "${METERED_URL}" "${SID_BATCH}" "${TOKEN_BATCH}" "tools/call" \
          '{"name":"echo","arguments":{"message":"batch item"}}' ;;
    10) send "${METERED_URL}" "${SID_BATCH}" "${TOKEN_BATCH}" "tools/call" \
          '{"name":"get-sum","arguments":{"a":9,"b":11}}' ;;
    11) send "${TOOLBOX_URL}" "${SID_CONSOLE}" "${TOKEN_BOB}" "tools/list" '{}' ;;
    # No token at all, so the gateway rejects it at the door.
    0)  send "${TOOLBOX_URL}" "${SID_DESKTOP}" "" "tools/call" \
          '{"name":"echo","arguments":{"message":"no token"}}' ;;
  esac

  if (( i % 12 == 0 )); then
    printf '  sent %3d requests, %ds left\n' "${total}" "$(( end - $(date +%s) ))"
  fi
  sleep "${INTERVAL}"
done

echo ""
echo "══════════════════════════════════════════════════"
echo " Done: ${total} requests, 3 sessions, 3 consumers"
echo "══════════════════════════════════════════════════"
printf '  succeeded                    %5d\n' "${count_ok}"
printf '  tool execution errors        %5d   valid tool, invalid argument\n' "${count_tool_error}"
printf '  refused by the allowlist     %5d   get-env, HTTP 400\n' "${count_blocked}"
printf '  rate limited                 %5d   metered proxy, HTTP 429\n' "${count_limited}"
printf '  rejected, no token           %5d   HTTP 401\n' "${count_unauth}"
(( count_other > 0 )) && printf '  other                        %5d\n' "${count_other}"
echo ""
ok "Moesif should show these within a few seconds."
echo "   https://www.moesif.com  then Analytics > MCP"
echo ""
echo " To verify from the terminal instead: ./test.sh"
echo ""
