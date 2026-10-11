#!/usr/bin/env bash
# Scenario mcp-proxy-upstream-path: an MCP proxy whose upstream URL ends in /mcp. Gateways before
# 1.2.0 append the /mcp operation path to the upstream, so platform-api strips the trailing /mcp for
# them; every version must reach the backend at exactly /api/mcp (never /api/mcp/mcp).
# request-info is not an MCP server: the assertion is on the path it received.
# Usage: mcp-proxy-upstream-path.sh <version>...
set -uo pipefail
S="$(cd "$(dirname "$0")/../scripts" && pwd)"; . "$S/lib.sh"
N=mcp-proxy-upstream-path
ensure_created mcp-proxies "{\"id\":\"e2e-mcp\",\"displayName\":\"e2e-mcp\",\"version\":\"v1.0\",\"context\":\"/e2e-mcp\",\"projectId\":\"default\",\"mcpSpecVersion\":\"2025-06-18\",\"upstream\":{\"main\":{\"url\":\"$BACKEND_URL_FROM_GW/api/mcp\"}}}"
INIT='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"e2e","version":"1"}}}'
for v in "$@"; do
  "$S/deploy.sh" mcp-proxies e2e-mcp "$v" >/dev/null || { check $N "$v" deploy DEPLOYED FAILED; continue; }
  out=$(invoke_until 20 URL.Path /api/mcp "$v" POST /e2e-mcp/mcp "$INIT" -H 'Accept: application/json, text/event-stream')
  check $N "$v" http-status 200 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" upstream-path /api/mcp "$(printf '%s\n' "$out" | echo_get URL.Path)"
done
