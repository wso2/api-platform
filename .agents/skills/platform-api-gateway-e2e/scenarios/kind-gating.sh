#!/usr/bin/env bash
# Scenario kind-gating: a kind the gateway does not have is refused at deploy time with 400
# DEPLOYMENT_KIND_UNSUPPORTED_BY_GATEWAY and leaves no deployment behind:
# no LTS gateway release has the Agent kind, so every LTS release must refuse.
# Usage: kind-gating.sh <version>...
set -uo pipefail
S="$(cd "$(dirname "$0")/../scripts" && pwd)"; . "$S/lib.sh"
N=kind-gating
ensure_created agent-proxies "{\"id\":\"e2e-agent\",\"displayName\":\"e2e-agent\",\"version\":\"v1.0\",\"projectId\":\"default\",\"context\":\"/e2e-agent\",\"upstream\":{\"main\":{\"url\":\"$BACKEND_URL_FROM_GW/agent\"}},\"protocol\":\"a2a\",\"a2a\":{\"protocolVersion\":\"1.0\",\"operationConfigs\":{\"transports\":[{\"protocolBinding\":\"JSONRPC\",\"pathPrefix\":\"/rpc\"}]}}}"
for v in "$@"; do
  out=$(api POST /agent-proxies/e2e-agent/deployments "{\"name\":\"dep-$RANDOM\",\"base\":\"current\",\"gatewayId\":\"$(gw_handle "$v")\"}")
  check $N "$v" http-status 400 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" error-code DEPLOYMENT_KIND_UNSUPPORTED_BY_GATEWAY "$(printf '%s\n' "$out" | http_body | json_get code)"
  left=$(api GET "/agent-proxies/e2e-agent/deployments?gatewayId=$(gw_handle "$v")" | http_body | json_get count)
  check $N "$v" no-deployment 0 "$left"
done
