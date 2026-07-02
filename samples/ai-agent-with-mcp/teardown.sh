#!/usr/bin/env bash
# teardown.sh — tear down all containers started by setup.sh

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

GATEWAY_DIR="wso2apip-ai-gateway-1.1.0"
GATEWAY_PROJECT=$(echo "$GATEWAY_DIR" | tr '[:upper:]' '[:lower:]' | tr -d '.')
GATEWAY_NETWORK="${GATEWAY_PROJECT}_gateway-network"
GATEWAY_VOLUME="${GATEWAY_PROJECT}_controller-data"

# Disconnect MCP containers from the gateway network before stopping them.
echo "==> Disconnecting MCP containers from gateway network..."
for CONTAINER in crm-mcp orders-mcp kb-mcp; do
  docker network disconnect -f "$GATEWAY_NETWORK" "$CONTAINER" 2>/dev/null && \
    echo "    Disconnected $CONTAINER." || true
done

echo "==> Stopping WireMock MCP containers..."
docker compose down --remove-orphans 2>/dev/null || true

if [[ -d "$GATEWAY_DIR" ]]; then
  echo "==> Stopping WSO2 AI Gateway..."
  (cd "$GATEWAY_DIR" && docker compose down --remove-orphans 2>/dev/null || true)
fi

# Remove the gateway controller volume so the next setup.sh gets a clean slate.
echo "==> Removing gateway state volume..."
docker volume rm "$GATEWAY_VOLUME" 2>/dev/null && \
  echo "    Volume $GATEWAY_VOLUME removed." || \
  echo "    Volume not found (already clean)."

# If the network still exists, force-remove it.
if docker network ls --format "{{.Name}}" | grep -q "^${GATEWAY_NETWORK}$"; then
  echo "==> Force-removing gateway network..."
  docker network rm "$GATEWAY_NETWORK" 2>/dev/null || \
    echo "    (network still in use by other containers — skipping)"
fi

echo "==> Done."
