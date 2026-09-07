#!/bin/sh
set -u
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GW_DIR="${SCRIPT_DIR}/wso2apip-ai-gateway-1.2.0"

echo "--> Stopping the gateway..."
[ -d "$GW_DIR" ] && (cd "$GW_DIR" && docker compose down >/dev/null 2>&1) || true

echo "--> Stopping the generated MCP server..."
docker rm -f rest-to-mcp-server >/dev/null 2>&1 || true

echo "--> Stopping sample-service..."
docker rm -f sample-service >/dev/null 2>&1 || true

echo "--> Removing the sample network..."
docker network rm rest-to-mcp >/dev/null 2>&1 || true

echo ""
echo "Done. To also remove the downloaded artifacts:"
echo "  rm -rf ${SCRIPT_DIR}/wso2apip-ai-gateway-1.2.0* ${SCRIPT_DIR}/bin ${SCRIPT_DIR}/generated ${SCRIPT_DIR}/.generate.log"
