#!/usr/bin/env bash
# Scenario llm-provider-secret: an OpenAI-template LLM provider whose upstream auth references a
# platform secret. Every gateway must send the plaintext key upstream: 1.0.0/1.1.0 get it inlined by
# platform-api at delivery time, 1.2.0 resolves it through secret sync. The stored deployment must
# keep the {{ secret }} placeholder.  Usage: llm-provider-secret.sh <version>...
set -uo pipefail
S="$(cd "$(dirname "$0")/../scripts" && pwd)"; . "$S/lib.sh"
N=llm-provider-secret
# One key per work dir (kept in state): changing it would be a rotation, which gateway 1.2.0 only
# picks up on reconnect — that case is the secret-rotation scenario, not this one.
state_load
KEY="${E2E_LLM_KEY:-sk-e2e-$(date +%s)}"; state_set E2E_LLM_KEY "$KEY"
ensure_secret llm-key "$KEY"
ensure_created llm-providers "{\"id\":\"e2e-llm\",\"displayName\":\"e2e-llm\",\"version\":\"v1.0\",\"template\":\"openai\",\"context\":\"/openai\",\"upstream\":{\"main\":{\"url\":\"$BACKEND_URL_FROM_GW/v1\",\"auth\":{\"type\":\"api-key\",\"header\":\"Authorization\",\"value\":\"Bearer {{ secret \\\"llm-key\\\" }}\"}}},\"accessControl\":{\"mode\":\"allow_all\"}}"
DB="$WORK/platform-api/api_platform.db"
for v in "$@"; do
  dep=$("$S/deploy.sh" llm-providers e2e-llm "$v") || { check $N "$v" deploy DEPLOYED FAILED; continue; }
  out=$(invoke_until 20 Header.Authorization.0 "Bearer $KEY" "$v" POST /openai/chat/completions '{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}')
  check $N "$v" http-status 200 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" upstream-path /v1/chat/completions "$(printf '%s\n' "$out" | echo_get URL.Path)"
  check $N "$v" upstream-auth "Bearer $KEY" "$(printf '%s\n' "$out" | echo_get Header.Authorization.0)"
  if command -v sqlite3 >/dev/null; then
    kept=$(sqlite3 "$DB" "select instr(content,'{{ secret') > 0 from deployments where uuid='${dep%% *}'")
    check $N "$v" stored-placeholder 1 "$kept"
  fi
done
