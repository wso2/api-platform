#!/usr/bin/env bash
# Scenario llm-proxy-additional-providers: an LLM proxy over e2e-llm with one additional provider.
# Below 1.2.0 the list is stripped with one platform-api warning per provider; the proxy must still
# deploy and route to its primary provider. 1.2.0 keeps the list (no warning).
# Run llm-provider-secret first (it creates e2e-llm).  Usage: llm-proxy-additional-providers.sh <version>...
set -uo pipefail
S="$(cd "$(dirname "$0")/../scripts" && pwd)"; . "$S/lib.sh"
N=llm-proxy-additional-providers
ensure_created llm-providers "{\"id\":\"e2e-llm-b\",\"displayName\":\"e2e-llm-b\",\"version\":\"v1.0\",\"template\":\"openai\",\"context\":\"/openai-b\",\"upstream\":{\"main\":{\"url\":\"$BACKEND_URL_FROM_GW/b\"}},\"accessControl\":{\"mode\":\"allow_all\"}}"
ensure_created llm-proxies "{\"id\":\"e2e-llm-proxy\",\"displayName\":\"e2e-llm-proxy\",\"version\":\"v1.0\",\"projectId\":\"default\",\"context\":\"/e2e-proxy\",\"provider\":{\"id\":\"e2e-llm\"},\"additionalProviders\":[{\"id\":\"e2e-llm-b\",\"as\":\"alt\"}]}"
for v in "$@"; do
  "$S/deploy.sh" llm-providers e2e-llm "$v" >/dev/null 2>&1 || true
  "$S/deploy.sh" llm-providers e2e-llm-b "$v" >/dev/null 2>&1 || true
  "$S/deploy.sh" llm-proxies e2e-llm-proxy "$v" >/dev/null || { check $N "$v" deploy DEPLOYED FAILED; continue; }
  out=$(invoke_until 20 URL.Path /v1/chat/completions "$v" POST /e2e-proxy/chat/completions '{"model":"gpt-4o","messages":[]}')
  check $N "$v" http-status 200 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" routes-to-primary /v1/chat/completions "$(printf '%s\n' "$out" | echo_get URL.Path)"
  if ver_below "$v" 1.2.0; then
    check $N "$v" stripped-with-warning 1 "$( [ "$(warned "$v" spec.additionalProviders)" -gt 0 ] && echo 1 || echo 0)"
  else
    check $N "$v" kept-without-warning 0 "$(warned "$v" spec.additionalProviders)"
  fi
done
