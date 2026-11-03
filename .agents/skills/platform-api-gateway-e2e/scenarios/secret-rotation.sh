#!/usr/bin/env bash
# Scenario secret-rotation: update the secret behind the llm-provider-secret provider, redeploy, and
# check the new value reaches the backend.
#   1.0.0 / 1.1.0: platform-api inlines the value at delivery time, so a redeploy carries it.
#   1.2.0: the controller caches each handle after its first fetch (syncSecretRefsFromYAML skips
#          cached handles) and refreshes only on reconnect, so a redeploy alone keeps the OLD value
#          (logged, not failed). The scenario restarts the controller and asserts after reconnect.
# Run llm-provider-secret first.  Usage: secret-rotation.sh <version>...
set -uo pipefail
S="$(cd "$(dirname "$0")/../scripts" && pwd)"; . "$S/lib.sh"
N=secret-rotation
NEW="sk-e2e-rotated-$(date +%s)"
ensure_secret llm-key "$NEW"
for v in "$@"; do
  "$S/deploy.sh" llm-providers e2e-llm "$v" >/dev/null || { check $N "$v" deploy DEPLOYED FAILED; continue; }
  if ver_below "$v" 1.2.0; then
    out=$(invoke_until 20 Header.Authorization.0 "Bearer $NEW" "$v" POST /openai/chat/completions '{}')
    check $N "$v" after-redeploy "Bearer $NEW" "$(printf '%s\n' "$out" | echo_get Header.Authorization.0)"
    continue
  fi
  # Secret-syncing gateways keep the cached value until they reconnect.
  out=$(invoke_until 10 Header.Authorization.0 "Bearer $NEW" "$v" POST /openai/chat/completions '{}')
  got=$(printf '%s\n' "$out" | echo_get Header.Authorization.0)
  [ "$got" = "Bearer $NEW" ] && log "$v picked up the rotated secret on redeploy" \
    || log "$v kept the cached secret after redeploy (known gateway behaviour); restarting its controller"
  ( cd "$(gw_dir "$v")" && docker compose -p "$(gw_project "$v")" restart gateway-controller >/dev/null 2>&1 )
  wait_for 120 "gateway $v controller health" gw_healthy "$v"
  out=$(invoke_until 60 Header.Authorization.0 "Bearer $NEW" "$v" POST /openai/chat/completions '{}')
  check $N "$v" after-reconnect "Bearer $NEW" "$(printf '%s\n' "$out" | echo_get Header.Authorization.0)"
done
state_set E2E_LLM_KEY "$NEW"
