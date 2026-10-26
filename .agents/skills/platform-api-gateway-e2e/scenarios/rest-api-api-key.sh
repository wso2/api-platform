#!/usr/bin/env bash
# Scenario rest-api-api-key: a REST API with the api-key-auth policy. Keys are created in platform-api
# and reach every gateway as hashes only: a key created before the deploy arrives by backfill at
# deploy time, one created after it by the live apikey.created event; a revoked key is refused within
# seconds and is not backfilled to the next gateway. The policy strips the key header before
# forwarding, so request-info must not see it. REST APIs also accept a caller-supplied key value.
# Usage: rest-api-api-key.sh <version>...
set -uo pipefail
S="$(cd "$(dirname "$0")/../scripts" && pwd)"; . "$S/lib.sh"
N=rest-api-api-key
ensure_created rest-apis "{\"id\":\"e2e-rest-keyed\",\"displayName\":\"e2e-rest-keyed\",\"context\":\"/e2e-keyed\",\"version\":\"v1\",\"projectId\":\"default\",\"upstream\":{\"main\":{\"url\":\"$BACKEND_URL_FROM_GW/keyed\"}},\"policies\":[{\"name\":\"api-key-auth\",\"version\":\"v1\",\"params\":{\"key\":\"X-API-Key\",\"in\":\"header\"}}]}"
# Key A and a caller-supplied key exist before any deploy: every gateway gets them by backfill.
kv=$(api_key_create rest-apis e2e-rest-keyed) || exit 1; A=${kv##* }
SUP="e2e-supplied-$(date +%s)-$RANDOM-0123456789abcdef0123456789abcdef"
api_key_create rest-apis e2e-rest-keyed "" "$SUP" >/dev/null || exit 1
PREV_B=""
for v in "$@"; do
  "$S/deploy.sh" rest-apis e2e-rest-keyed "$v" >/dev/null || { check $N "$v" deploy DEPLOYED FAILED; continue; }
  out=$(invoke_until_code 30 200 "$v" GET /e2e-keyed/hello "" -H "X-API-Key: $A")
  check $N "$v" http-with-key 200 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" upstream-path /keyed/hello "$(printf '%s\n' "$out" | echo_get URL.Path)"
  check $N "$v" key-header-stripped no "$(printf '%s\n' "$out" | echo_has_header X-API-Key)"
  out=$("$S/invoke.sh" "$v" GET /e2e-keyed/hello)
  check $N "$v" no-key-http 401 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" no-key-message "$UNAUTHORIZED_MSG" "$(printf '%s\n' "$out" | http_body | json_get message)"
  out=$("$S/invoke.sh" "$v" GET /e2e-keyed/hello "" -H "X-API-Key: wrong-$RANDOM-$RANDOM")
  check $N "$v" wrong-key-http 401 "$(printf '%s\n' "$out" | http_code)"
  out=$(invoke_until_code 20 200 "$v" GET /e2e-keyed/hello "" -H "X-API-Key: $SUP")
  check $N "$v" supplied-key-http 200 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" gw-keys-backfilled 1 "$([ "$(gw_api_key_count "$v" rest-apis e2e-rest-keyed)" -ge 2 ] && echo 1 || echo 0)"
  if [ -n "$PREV_B" ]; then
    out=$("$S/invoke.sh" "$v" GET /e2e-keyed/hello "" -H "X-API-Key: $PREV_B")
    check $N "$v" revoked-key-not-backfilled 401 "$(printf '%s\n' "$out" | http_code)"
  fi
  # Key B is created after the deploy: it reaches this gateway only through the live event.
  kv=$(api_key_create rest-apis e2e-rest-keyed) || { check $N "$v" live-key-create 201 failed; continue; }
  B_ID=${kv%% *}; B=${kv##* }
  out=$(invoke_until_code 20 200 "$v" GET /e2e-keyed/hello "" -H "X-API-Key: $B")
  check $N "$v" live-key-http 200 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" gw-lists-live-key yes "$(gw_api_key_present "$v" rest-apis e2e-rest-keyed "$B_ID")"
  check $N "$v" revoke-http 204 "$(api_key_revoke rest-apis e2e-rest-keyed "$B_ID")"
  out=$(invoke_until_code 20 401 "$v" GET /e2e-keyed/hello "" -H "X-API-Key: $B")
  check $N "$v" revoked-key-http 401 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" revoked-key-message "$UNAUTHORIZED_MSG" "$(printf '%s\n' "$out" | http_body | json_get message)"
  check $N "$v" gw-drops-revoked-key no "$(gw_api_key_present "$v" rest-apis e2e-rest-keyed "$B_ID")"
  out=$("$S/invoke.sh" "$v" GET /e2e-keyed/hello "" -H "X-API-Key: $A")
  check $N "$v" other-key-still-valid 200 "$(printf '%s\n' "$out" | http_code)"
  PREV_B=$B
done
