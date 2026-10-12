#!/usr/bin/env bash
# Scenario secret-undecryptable: an LLM provider whose upstream auth references a secret that exists
# and is ACTIVE but whose ciphertext this platform-api cannot decrypt. On a gateway that needs secrets
# inlined (< 1.2.0; later releases are skipped) the fetch must be refused: deployment FAILED with statusReason
# SECRET_RESOLUTION_FAILED, platform-api logs the cause, the gateway log carries the 422 body.
# Before the fix: silent 500, statusReason GATEWAY_PROCESSING_ERROR (from the gateway's ack).
set -uo pipefail
S="$(cd "$(dirname "$0")/../scripts" && pwd)"; . "$S/lib.sh"
N=secret-undecryptable
DB="$WORK/platform-api/api_platform.db"
ensure_secret e2e-broken-key "sk-e2e-broken"
ensure_created llm-providers "{\"id\":\"e2e-llm-broken\",\"displayName\":\"e2e-llm-broken\",\"version\":\"v1.0\",\"template\":\"openai\",\"context\":\"/openai-broken\",\"upstream\":{\"main\":{\"url\":\"$BACKEND_URL_FROM_GW/v1\",\"auth\":{\"type\":\"api-key\",\"header\":\"Authorization\",\"value\":\"Bearer {{ secret \\\"e2e-broken-key\\\" }}\"}}},\"accessControl\":{\"mode\":\"allow_all\"}}"
# Same length, random bytes: the row stays ACTIVE and well-formed; AES-GCM just cannot open it.
sqlite3 "$DB" "update secrets set ciphertext = randomblob(length(ciphertext)) where handle = 'e2e-broken-key'"
for v in "$@"; do
  # Gateways from 1.2.0 (and STS builds) resolve secrets through their own sync, not this fetch.
  ver_below "$v" 1.2.0 || { log "$N: gateway $v syncs secrets itself; the inline-fetch refusal does not apply — skipped"; continue; }
  out=$("$S/deploy.sh" llm-providers e2e-llm-broken "$v" 2>/dev/null); dep=${out%% *}; st=${out##* }
  check $N "$v" deploy-status FAILED "$st"
  check $N "$v" status-reason SECRET_RESOLUTION_FAILED "$(api GET "/llm-providers/e2e-llm-broken/deployments/$dep" | http_body | json_get statusReason)"
  n=$(grep -c 'Refused deployment fetch: secret could not be resolved' "$LOGS/platform-api.log")
  check $N "$v" papi-logs-cause yes "$([ "${n:-0}" -gt 0 ] && echo yes || echo no)"
  gwlog=$(cd "$(gw_dir "$v")" && docker compose -p "$(gw_project "$v")" logs --no-color gateway-controller 2>/dev/null)
  code=$(printf '%s\n' "$gwlog" | grep -o 'LLM provider request failed with status [0-9]*' | tail -1 | awk '{print $NF}')
  check $N "$v" gateway-saw-http 422 "${code:-none}"
  check $N "$v" gateway-log-names-reason yes "$(printf '%s\n' "$gwlog" | grep -q SECRET_RESOLUTION_FAILED && echo yes || echo no)"
done
