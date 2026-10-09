#!/usr/bin/env bash
# Scenario upstream-auth-types (MCP): no LTS MCP validator accepts an auth block without a header and
# value (1.2.0 lists none/other in its schema but still requires both). So for every LTS gateway
# platform-api must drop auth type "none" (deploys, nothing sent upstream) and warn on "other", which
# LTS releases apply like api-key.  Usage: upstream-auth-types.sh <version>...
set -uo pipefail
S="$(cd "$(dirname "$0")/../scripts" && pwd)"; . "$S/lib.sh"
N=upstream-auth-types
ensure_created mcp-proxies "{\"id\":\"e2e-mcp-none\",\"displayName\":\"e2e-mcp-none\",\"version\":\"v1.0\",\"context\":\"/e2e-mcp-none\",\"projectId\":\"default\",\"mcpSpecVersion\":\"2025-06-18\",\"upstream\":{\"main\":{\"url\":\"$BACKEND_URL_FROM_GW/none/mcp\",\"auth\":{\"type\":\"none\"}}}}"
ensure_created mcp-proxies "{\"id\":\"e2e-mcp-other\",\"displayName\":\"e2e-mcp-other\",\"version\":\"v1.0\",\"context\":\"/e2e-mcp-other\",\"projectId\":\"default\",\"mcpSpecVersion\":\"2025-06-18\",\"upstream\":{\"main\":{\"url\":\"$BACKEND_URL_FROM_GW/other/mcp\",\"auth\":{\"type\":\"other\",\"header\":\"X-Other-Auth\",\"value\":\"other-value\"}}}}"
INIT='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"e2e","version":"1"}}}'
for v in "$@"; do
  if "$S/deploy.sh" mcp-proxies e2e-mcp-none "$v" >/dev/null; then
    check $N "$v" none-deploy DEPLOYED DEPLOYED
    out=$(invoke_until 20 URL.Path /none/mcp "$v" POST /e2e-mcp-none/mcp "$INIT" -H 'Accept: application/json, text/event-stream')
    check $N "$v" none-upstream-path /none/mcp "$(printf '%s\n' "$out" | echo_get URL.Path)"
    check $N "$v" none-gateway-auth-block "<absent>" "$(gw_find "$v" mcp-proxies/e2e-mcp-none auth)"
  else
    check $N "$v" none-deploy DEPLOYED FAILED
  fi
  st=$("$S/deploy.sh" mcp-proxies e2e-mcp-other "$v" 2>/dev/null | awk '{print $2}')
  check $N "$v" other-deploy DEPLOYED "${st:-REFUSED}"
  check $N "$v" other-warning 1 "$( [ "$(warned "$v" spec.upstream.auth)" -gt 0 ] && echo 1 || echo 0)"
  out=$(invoke_until 20 Header.X-Other-Auth.0 other-value "$v" POST /e2e-mcp-other/mcp "$INIT" -H 'Accept: application/json, text/event-stream')
  check $N "$v" other-applied-like-api-key other-value "$(printf '%s\n' "$out" | echo_get Header.X-Other-Auth.0)"
done
