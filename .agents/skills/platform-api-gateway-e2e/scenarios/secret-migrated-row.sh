#!/usr/bin/env bash
# Scenario secret-migrated-row: mirrors the dev incident artifact. An OpenAI-template LLM provider
# with api-key security whose upstream auth references a secret whose row looks like one written by
# the v1->v2 migration client (description / created_by / updated_by NULL). On a gateway that needs
# secrets inlined: deployment DEPLOYED; a request with a valid API key reaches the backend carrying
# the plaintext secret; a request without one gets 401 from the gateway (api-key-auth applied).
# Before the fix: the NULL columns break the secret read -> silent 500 -> FAILED/GATEWAY_PROCESSING_ERROR.
set -uo pipefail
S="$(cd "$(dirname "$0")/../scripts" && pwd)"; . "$S/lib.sh"
N=secret-migrated-row
DB="$WORK/platform-api/api_platform.db"
KEY="sk-e2e-migrated"
ensure_secret e2e-migrated-key "$KEY"
sqlite3 "$DB" "update secrets set description = NULL, created_by = NULL, updated_by = NULL where handle = 'e2e-migrated-key'"
ensure_created llm-providers "{\"id\":\"e2e-llm-migrated\",\"displayName\":\"e2e-llm-migrated\",\"version\":\"v1.0\",\"template\":\"openai\",\"context\":\"/openai-migrated\",\"upstream\":{\"main\":{\"url\":\"$BACKEND_URL_FROM_GW/v1\",\"auth\":{\"type\":\"api-key\",\"header\":\"Authorization\",\"value\":\"Bearer {{ secret \\\"e2e-migrated-key\\\" }}\"}}},\"accessControl\":{\"mode\":\"allow_all\"},\"security\":{\"enabled\":true,\"apiKey\":{\"enabled\":true,\"key\":\"X-API-Key\",\"in\":\"header\"}}}"
out=$(api POST "/llm-providers/e2e-llm-migrated/api-keys" "{\"displayName\":\"e2e-key-$(date +%s)\"}")
[ "$(printf '%s\n' "$out" | http_code)" = 201 ] || { printf '%s\n' "$out" >&2; die "api key creation failed"; }
APIKEY=$(printf '%s\n' "$out" | http_body | json_get apiKey); [ -n "$APIKEY" ] || die "no apiKey in response"
BODY='{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}'
for v in "$@"; do
  out=$("$S/deploy.sh" llm-providers e2e-llm-migrated "$v" 2>/dev/null); dep=${out%% *}; st=${out##* }
  if ! check $N "$v" deploy-status DEPLOYED "$st"; then
    gwlog=$(cd "$(gw_dir "$v")" && docker compose -p "$(gw_project "$v")" logs --no-color gateway-controller 2>/dev/null)
    code=$(printf '%s\n' "$gwlog" | grep -o 'LLM provider request failed with status [0-9]*' | tail -1 | awk '{print $NF}')
    check $N "$v" fetch-outcome delivered "statusReason=$(api GET "/llm-providers/e2e-llm-migrated/deployments/$dep" | http_body | json_get statusReason) gateway-http=${code:-none}"
    continue
  fi
  out=$(invoke_until 30 Header.Authorization.0 "Bearer $KEY" "$v" POST /openai-migrated/chat/completions "$BODY" -H "X-API-Key: $APIKEY")
  check $N "$v" with-key-http 200 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" upstream-auth "Bearer $KEY" "$(printf '%s\n' "$out" | echo_get Header.Authorization.0)"
  out=$("$S/invoke.sh" "$v" POST /openai-migrated/chat/completions "$BODY")
  check $N "$v" no-key-http 401 "$(printf '%s\n' "$out" | http_code)"
done
