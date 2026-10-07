#!/usr/bin/env bash
# Scenario mcp-spec-versions-fold: mcpSpecVersions [2025-06-18, 2025-11-25]. No LTS gateway knows the
# plural list, so platform-api folds it into the singular specVersion = newest supported
# (2025-11-25); the gateway's own view must show exactly that.  Usage: mcp-spec-versions-fold.sh <version>...
set -uo pipefail
S="$(cd "$(dirname "$0")/../scripts" && pwd)"; . "$S/lib.sh"
N=mcp-spec-versions-fold
ensure_created mcp-proxies "{\"id\":\"e2e-mcp-fold\",\"displayName\":\"e2e-mcp-fold\",\"version\":\"v1.0\",\"context\":\"/e2e-mcp-fold\",\"projectId\":\"default\",\"mcpSpecVersions\":[\"2025-06-18\",\"2025-11-25\"],\"upstream\":{\"main\":{\"url\":\"$BACKEND_URL_FROM_GW/fold/mcp\"}}}"
for v in "$@"; do
  "$S/deploy.sh" mcp-proxies e2e-mcp-fold "$v" >/dev/null || { check $N "$v" deploy DEPLOYED FAILED; continue; }
  check $N "$v" gateway-specVersion 2025-11-25 "$(gw_find "$v" mcp-proxies/e2e-mcp-fold specVersion)"
  check $N "$v" gateway-specVersions-list "<absent>" "$(gw_find "$v" mcp-proxies/e2e-mcp-fold specVersions)"
done
