#!/usr/bin/env bash
# Test: Text Prompt Decoration (Mode 1 — append)
#   Sends a plain question. The suffix-proxy APPENDS an instruction to the
#   user's message telling the model to end its reply with the exact tag
#   [ABC-HORIZON-OK]. The caller never asks for that tag, so its presence
#   in the reply proves the gateway appended the decoration before
#   forwarding the request to OpenAI.
set -uo pipefail

CHAT_URL="http://localhost:8080/suffix-proxy/chat/completions"
HEALTH_URL="http://localhost:9094/health"
MARKER="[ABC-HORIZON-OK]"

GREEN="\033[0;32m"; RED="\033[0;31m"; BLUE="\033[0;34m"; NC="\033[0m"
# Use printf, not `echo -e`: some shells' echo prints "-e" literally and/or
# interprets backslash escapes, which corrupts JSON piped to jq.
pass() { printf '%b[PASS]%b %s\n' "${GREEN}" "${NC}" "$*"; }
fail() { printf '%b[FAIL]%b %s\n' "${RED}" "${NC}" "$*"; }
info() { printf '%b[INFO]%b %s\n' "${BLUE}" "${NC}" "$*"; }

chat_req() {
  local msg="$1"
  local body
  body=$(jq -n --arg m "${msg}" \
    '{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":$m}],"max_tokens":150}')
  # Append the HTTP status on its own trailing line so the caller can tell a
  # gateway/upstream failure from a real answer.
  curl -s -w $'\n%{http_code}' -X POST "${CHAT_URL}" \
    -H "Content-Type: application/json" \
    -d "${body}"
}

echo ""
echo "══════════════════════════════════════════════════"
echo " Pre-flight checks"
echo "══════════════════════════════════════════════════"

if ! command -v jq >/dev/null 2>&1; then
  printf '%b[ERROR]%b jq is required: brew install jq\n' "${RED}" "${NC}" >&2; exit 1
fi

info "Checking gateway health at ${HEALTH_URL} ..."
if ! curl -sf "${HEALTH_URL}" >/dev/null 2>&1; then
  printf '%b[ERROR]%b Gateway is not running. Run ./setup.sh first.\n' "${RED}" "${NC}" >&2; exit 1
fi
printf '%b[OK]%b    Gateway is healthy.\n' "${GREEN}" "${NC}"

echo ""
echo "══════════════════════════════════════════════════"
echo " Test: Text Prompt Decoration (appended suffix)"
echo "══════════════════════════════════════════════════"

USER_MSG="What is the capital of France? Answer in one short sentence."
info "Sending plain question (no formatting instructions): \"${USER_MSG}\""

RAW=$(chat_req "${USER_MSG}")
HTTP_CODE="${RAW##*$'\n'}"   # last line
RESP="${RAW%$'\n'*}"        # everything before it

echo ""
echo "══════════════════════════════════════════════════"
if [[ "${HTTP_CODE}" != 2* ]] || jq -e 'type=="object" and has("error")' >/dev/null 2>&1 <<< "${RESP}"; then
  # The gateway returns {"error":"..."} (string); upstream APIs return
  # {"error":{"message":"..."}} (object). Handle both, else show the raw body.
  ERR=$(jq -r '
    if (.error|type)=="object" then (.error.message // "unknown error")
    elif (.error|type)=="string" then .error
    else "unknown error" end' 2>/dev/null <<< "${RESP}")
  [[ -z "${ERR}" || "${ERR}" == "null" ]] && ERR="${RESP}"
  fail "Request failed (HTTP ${HTTP_CODE}): ${ERR}"
  echo "══════════════════════════════════════════════════"; echo ""; exit 1
fi

CONTENT=$(jq -r '.choices[0].message.content // ""' <<< "${RESP}")
info "AI response: ${CONTENT}"
echo ""

if printf '%s' "${CONTENT}" | grep -qF "${MARKER}"; then
  pass "Text decoration applied — appended instruction honored ('${MARKER}' present though the caller never sent it)."
  echo "══════════════════════════════════════════════════"; echo ""; exit 0
else
  fail "Tag not detected — '${MARKER}' absent. The suffix may not have been appended."
  echo "══════════════════════════════════════════════════"; echo ""; exit 1
fi
