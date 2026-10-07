#!/usr/bin/env bash
# Scenario llm-policies-flatten: an LLM provider with globalPolicies + operationPolicies (set-headers).
# Below 1.2.0 platform-api flattens both into the legacy policies list; every version must apply both.
# Known: gateway 1.0.0 keeps only the last same-named policy on a route, so on /chat/completions the
# operation-level set-headers hides the global one there; global-header-other-path still proves the
# flattened global policy is applied.  Usage: llm-policies-flatten.sh <version>...
set -uo pipefail
S="$(cd "$(dirname "$0")/../scripts" && pwd)"; . "$S/lib.sh"
N=llm-policies-flatten
BODY=$(python3 - "$BACKEND_URL_FROM_GW" <<'JSON'
import json,sys
hdr=lambda n,v: {"request": {"headers": [{"name": n, "value": v}]}}
print(json.dumps({
  "id": "e2e-llm-pol", "displayName": "e2e-llm-pol", "version": "v1.0", "template": "openai",
  "context": "/openai-pol", "upstream": {"main": {"url": sys.argv[1] + "/v1"}},
  "accessControl": {"mode": "allow_all"},
  "globalPolicies": [{"name": "set-headers", "version": "v1", "params": hdr("X-Global-Policy", "g")}],
  "operationPolicies": [{"name": "set-headers", "version": "v1", "paths": [
    {"path": "/chat/completions", "methods": ["POST"], "params": hdr("X-Op-Policy", "o")}]}],
}))
JSON
)
ensure_created llm-providers "$BODY"
for v in "$@"; do
  "$S/deploy.sh" llm-providers e2e-llm-pol "$v" >/dev/null || { check $N "$v" deploy DEPLOYED FAILED; continue; }
  out=$(invoke_until 20 Header.X-Op-Policy.0 o "$v" POST /openai-pol/chat/completions '{"model":"gpt-4o","messages":[]}')
  check $N "$v" http-status 200 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" operation-policy-header o "$(printf '%s\n' "$out" | echo_get Header.X-Op-Policy.0)"
  if [ "$v" = 1.0.0 ]; then
    log "skip global-policy-header on 1.0.0 /chat/completions: same-named policies collapse there (known gateway behaviour)"
  else
    check $N "$v" global-policy-header g "$(printf '%s\n' "$out" | echo_get Header.X-Global-Policy.0)"
  fi
  out=$(invoke_until 20 Header.X-Global-Policy.0 g "$v" POST /openai-pol/embeddings '{"model":"m","input":"x"}')
  check $N "$v" global-header-other-path g "$(printf '%s\n' "$out" | echo_get Header.X-Global-Policy.0)"
done
