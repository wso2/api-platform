#!/usr/bin/env bash
# Scenario llm-provider-api-key: an OpenAI-template LLM provider with security.apiKey, which
# platform-api turns into the api-key-auth policy (flattened into the legacy policies list below
# 1.2.0). Keys created in platform-api reach every gateway as hashes: one created before the deploy
# by backfill at deploy time, one created after it by the live apikey.created event; a revoked key
# is refused within seconds and is not backfilled to the next gateway. The policy strips the key
# header before forwarding, so request-info must not see it.  Usage: llm-provider-api-key.sh <version>...
set -uo pipefail
S="$(cd "$(dirname "$0")/../scripts" && pwd)"; . "$S/lib.sh"
N=llm-provider-api-key
ensure_created llm-providers "{\"id\":\"e2e-llm-keyed\",\"displayName\":\"e2e-llm-keyed\",\"version\":\"v1.0\",\"template\":\"openai\",\"context\":\"/openai-keyed\",\"upstream\":{\"main\":{\"url\":\"$BACKEND_URL_FROM_GW/v1\"}},\"accessControl\":{\"mode\":\"allow_all\"},\"security\":{\"enabled\":true,\"apiKey\":{\"enabled\":true,\"key\":\"X-API-Key\",\"in\":\"header\"}}}"
# Key A exists before any deploy: every gateway gets it by backfill.
kv=$(api_key_create llm-providers e2e-llm-keyed) || exit 1; A=${kv##* }
BODY='{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}'
P=/openai-keyed/chat/completions
PREV_B=""
for v in "$@"; do
  "$S/deploy.sh" llm-providers e2e-llm-keyed "$v" >/dev/null || { check $N "$v" deploy DEPLOYED FAILED; continue; }
  out=$(invoke_until_code 30 200 "$v" POST $P "$BODY" -H "X-API-Key: $A")
  check $N "$v" http-with-key 200 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" upstream-path /v1/chat/completions "$(printf '%s\n' "$out" | echo_get URL.Path)"
  check $N "$v" key-header-stripped no "$(printf '%s\n' "$out" | echo_has_header X-API-Key)"
  out=$("$S/invoke.sh" "$v" POST $P "$BODY")
  check $N "$v" no-key-http 401 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" no-key-message "$UNAUTHORIZED_MSG" "$(printf '%s\n' "$out" | http_body | json_get message)"
  out=$("$S/invoke.sh" "$v" POST $P "$BODY" -H "X-API-Key: wrong-$RANDOM-$RANDOM")
  check $N "$v" wrong-key-http 401 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" gw-keys-backfilled 1 "$([ "$(gw_api_key_count "$v" llm-providers e2e-llm-keyed)" -ge 1 ] && echo 1 || echo 0)"
  if [ -n "$PREV_B" ]; then
    out=$("$S/invoke.sh" "$v" POST $P "$BODY" -H "X-API-Key: $PREV_B")
    check $N "$v" revoked-key-not-backfilled 401 "$(printf '%s\n' "$out" | http_code)"
  fi
  # Key B is created after the deploy: it reaches this gateway only through the live event.
  kv=$(api_key_create llm-providers e2e-llm-keyed) || { check $N "$v" live-key-create 201 failed; continue; }
  B_ID=${kv%% *}; B=${kv##* }
  out=$(invoke_until_code 20 200 "$v" POST $P "$BODY" -H "X-API-Key: $B")
  check $N "$v" live-key-http 200 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" gw-lists-live-key yes "$(gw_api_key_present "$v" llm-providers e2e-llm-keyed "$B_ID")"
  check $N "$v" revoke-http 204 "$(api_key_revoke llm-providers e2e-llm-keyed "$B_ID")"
  out=$(invoke_until_code 20 401 "$v" POST $P "$BODY" -H "X-API-Key: $B")
  check $N "$v" revoked-key-http 401 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" revoked-key-message "$UNAUTHORIZED_MSG" "$(printf '%s\n' "$out" | http_body | json_get message)"
  check $N "$v" gw-drops-revoked-key no "$(gw_api_key_present "$v" llm-providers e2e-llm-keyed "$B_ID")"
  out=$("$S/invoke.sh" "$v" POST $P "$BODY" -H "X-API-Key: $A")
  check $N "$v" other-key-still-valid 200 "$(printf '%s\n' "$out" | http_code)"
  PREV_B=$B
done
