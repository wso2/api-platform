#!/usr/bin/env bash
# Scenario llm-proxy-api-key: an LLM proxy in front of an api-key-secured LLM provider. The provider's
# key is issued by platform-api, stored as a platform secret and referenced from the proxy's
# provider.auth as {{ secret }}; the proxy is secured with its own key. On every gateway the proxy's
# upstream is a loopback into the provider route (re-entering the provider's api-key-auth), so a
# client holding only the proxy key must reach the backend, which must see the provider's upstream
# secret in Authorization and neither key header. Keys are bound to one artifact: the provider key
# is refused at the proxy and the proxy key at the provider. A key created after the deploy arrives
# by the live event; a revoked one is refused within seconds and not backfilled to the next gateway.
# Self-contained (own provider and secrets).  Usage: llm-proxy-api-key.sh <version>...
set -uo pipefail
S="$(cd "$(dirname "$0")/../scripts" && pwd)"; . "$S/lib.sh"
N=llm-proxy-api-key
DB="$WORK/platform-api/api_platform.db"
state_load
# Values kept per work dir: gateway 1.2.0 caches a secret until reconnect, and a key's plaintext is
# returned once, so reruns must reuse them rather than mint new ones.
UPKEY="${E2E_UPSTREAM_KEY:-sk-e2e-up-$(date +%s)}"; state_set E2E_UPSTREAM_KEY "$UPKEY"
ensure_secret e2e-upstream-key "$UPKEY"
ensure_created llm-providers "{\"id\":\"e2e-llm-up\",\"displayName\":\"e2e-llm-up\",\"version\":\"v1.0\",\"template\":\"openai\",\"context\":\"/openai-up\",\"upstream\":{\"main\":{\"url\":\"$BACKEND_URL_FROM_GW/v1\",\"auth\":{\"type\":\"api-key\",\"header\":\"Authorization\",\"value\":\"Bearer {{ secret \\\"e2e-upstream-key\\\" }}\"}}},\"accessControl\":{\"mode\":\"allow_all\"},\"security\":{\"enabled\":true,\"apiKey\":{\"enabled\":true,\"key\":\"X-API-Key\",\"in\":\"header\"}}}"
if [ -z "${E2E_PROVIDER_KEY:-}" ]; then
  kv=$(api_key_create llm-providers e2e-llm-up) || exit 1
  E2E_PROVIDER_KEY=${kv##* }; state_set E2E_PROVIDER_KEY "$E2E_PROVIDER_KEY"
fi
PK="$E2E_PROVIDER_KEY"
ensure_secret e2e-provider-key "$PK"
ensure_created llm-proxies "{\"id\":\"e2e-llm-proxy-keyed\",\"displayName\":\"e2e-llm-proxy-keyed\",\"version\":\"v1.0\",\"projectId\":\"default\",\"context\":\"/e2e-proxy-keyed\",\"provider\":{\"id\":\"e2e-llm-up\",\"auth\":{\"type\":\"api-key\",\"header\":\"X-API-Key\",\"value\":\"{{ secret \\\"e2e-provider-key\\\" }}\"}},\"security\":{\"enabled\":true,\"apiKey\":{\"enabled\":true,\"key\":\"X-Proxy-Key\",\"in\":\"header\"}}}"
# Proxy key X exists before any deploy: every gateway gets it by backfill.
kv=$(api_key_create llm-proxies e2e-llm-proxy-keyed) || exit 1; X=${kv##* }
BODY='{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}'
P=/e2e-proxy-keyed/chat/completions
PREV_B=""
for v in "$@"; do
  "$S/deploy.sh" llm-providers e2e-llm-up "$v" >/dev/null || { check $N "$v" provider-deploy DEPLOYED FAILED; continue; }
  dep=$("$S/deploy.sh" llm-proxies e2e-llm-proxy-keyed "$v") || { check $N "$v" deploy DEPLOYED FAILED; continue; }
  out=$(invoke_until_code 30 200 "$v" POST $P "$BODY" -H "X-Proxy-Key: $X")
  check $N "$v" http-with-proxy-key 200 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" upstream-path /v1/chat/completions "$(printf '%s\n' "$out" | echo_get URL.Path)"
  check $N "$v" upstream-auth "Bearer $UPKEY" "$(printf '%s\n' "$out" | echo_get Header.Authorization.0)"
  check $N "$v" provider-key-stripped no "$(printf '%s\n' "$out" | echo_has_header X-API-Key)"
  check $N "$v" proxy-key-stripped no "$(printf '%s\n' "$out" | echo_has_header X-Proxy-Key)"
  out=$("$S/invoke.sh" "$v" POST $P "$BODY")
  check $N "$v" no-key-http 401 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" no-key-message "$UNAUTHORIZED_MSG" "$(printf '%s\n' "$out" | http_body | json_get message)"
  out=$("$S/invoke.sh" "$v" POST $P "$BODY" -H "X-Proxy-Key: $PK")
  check $N "$v" provider-key-as-proxy-key 401 "$(printf '%s\n' "$out" | http_code)"
  out=$("$S/invoke.sh" "$v" POST /openai-up/chat/completions "$BODY" -H "X-API-Key: $X")
  check $N "$v" proxy-key-at-provider 401 "$(printf '%s\n' "$out" | http_code)"
  if command -v sqlite3 >/dev/null; then
    kept=$(sqlite3 "$DB" "select instr(content,'{{ secret') > 0 from deployments where uuid='${dep%% *}'")
    check $N "$v" stored-placeholder 1 "$kept"
  fi
  check $N "$v" gw-proxy-keys-backfilled 1 "$([ "$(gw_api_key_count "$v" llm-proxies e2e-llm-proxy-keyed)" -ge 1 ] && echo 1 || echo 0)"
  check $N "$v" gw-provider-keys-backfilled 1 "$([ "$(gw_api_key_count "$v" llm-providers e2e-llm-up)" -ge 1 ] && echo 1 || echo 0)"
  if [ -n "$PREV_B" ]; then
    out=$("$S/invoke.sh" "$v" POST $P "$BODY" -H "X-Proxy-Key: $PREV_B")
    check $N "$v" revoked-key-not-backfilled 401 "$(printf '%s\n' "$out" | http_code)"
  fi
  # Proxy key B is created after the deploy: it reaches this gateway only through the live event.
  kv=$(api_key_create llm-proxies e2e-llm-proxy-keyed) || { check $N "$v" live-key-create 201 failed; continue; }
  B_ID=${kv%% *}; B=${kv##* }
  out=$(invoke_until_code 20 200 "$v" POST $P "$BODY" -H "X-Proxy-Key: $B")
  check $N "$v" live-key-http 200 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" gw-lists-live-key yes "$(gw_api_key_present "$v" llm-proxies e2e-llm-proxy-keyed "$B_ID")"
  check $N "$v" revoke-http 204 "$(api_key_revoke llm-proxies e2e-llm-proxy-keyed "$B_ID")"
  out=$(invoke_until_code 20 401 "$v" POST $P "$BODY" -H "X-Proxy-Key: $B")
  check $N "$v" revoked-key-http 401 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" gw-drops-revoked-key no "$(gw_api_key_present "$v" llm-proxies e2e-llm-proxy-keyed "$B_ID")"
  out=$("$S/invoke.sh" "$v" POST $P "$BODY" -H "X-Proxy-Key: $X")
  check $N "$v" other-key-still-valid 200 "$(printf '%s\n' "$out" | http_code)"
  PREV_B=$B
done
