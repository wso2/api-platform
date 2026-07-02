#!/usr/bin/env bash
# setup.sh
#
# What it does:
#   1. Load .env
#   2. Download & unzip WSO2 AI Gateway v1.1.0 (cached after first run)
#   3. Start the gateway via its bundled docker-compose
#   4. Pull & start the three WireMock MCP backend containers
#   5. Connect the MCP containers to the gateway's Docker network
#   6. Wait for the gateway to become healthy
#   7. Register the Anthropic provider, LLM proxy, and three MCP proxies
#   8. Run the agent
#
# Prerequisites: docker (with compose plugin), curl, unzip, python 3.10+

set -euo pipefail

# Config
GATEWAY_VERSION="1.1.0"
GATEWAY_DIR="wso2apip-ai-gateway-${GATEWAY_VERSION}"
GATEWAY_ZIP="${GATEWAY_DIR}.zip"
DOWNLOAD_URL="https://github.com/wso2/api-platform/releases/download/ai-gateway/v${GATEWAY_VERSION}/${GATEWAY_ZIP}"

HEALTH_URL="http://localhost:9094/health"
MGMT_URL="http://localhost:9090/api/management/v0.9/llm-providers"

# Load .env 
ENV_FILE="$(dirname "$0")/.env"
if [[ ! -f "$ENV_FILE" ]]; then
  echo "ERROR: .env not found. Copy .env.example to .env and set ANTHROPIC_API_KEY."
  exit 1
fi
set -o allexport; source "$ENV_FILE"; set +o allexport

if [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
  echo "ERROR: ANTHROPIC_API_KEY is not set in .env."
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# Step 1: Download & unzip gateway
echo "==> Checking WSO2 AI Gateway..."
if [[ ! -d "$GATEWAY_DIR" ]]; then
  if [[ ! -f "$GATEWAY_ZIP" ]]; then
    echo "    Downloading ${GATEWAY_ZIP}..."
    curl -L --progress-bar "$DOWNLOAD_URL" -o "$GATEWAY_ZIP"
  fi
  echo "    Extracting..."
  unzip -q "$GATEWAY_ZIP"
fi
echo "    Gateway distribution ready."

# Step 2: Start gateway
echo "==> Starting WSO2 AI Gateway..."
(cd "$GATEWAY_DIR" && docker compose up -d --quiet-pull)

# Step 3: Pull & start WireMock MCP containers
echo "==> Starting WireMock MCP backend containers..."
docker compose up -d --quiet-pull

# Step 4: Connect MCP containers to gateway network
echo "==> Connecting MCP containers to gateway Docker network..."
GATEWAY_PROJECT=$(echo "$GATEWAY_DIR" | tr '[:upper:]' '[:lower:]' | tr -d '.')
GATEWAY_NETWORK="${GATEWAY_PROJECT}_gateway-network"

# Verify the network exists
WAITED_NET=0
until docker network ls --format "{{.Name}}" | grep -q "^${GATEWAY_NETWORK}$"; do
  if (( WAITED_NET >= 30 )); then
    echo "ERROR: Expected Docker network '${GATEWAY_NETWORK}' not found."
    echo "       Available networks:"
    docker network ls
    exit 1
  fi
  sleep 3
  WAITED_NET=$((WAITED_NET + 3))
done
echo "    Using network: $GATEWAY_NETWORK"

for CONTAINER in crm-mcp orders-mcp kb-mcp; do
  # Connect only if not already connected
  if ! docker network inspect "$GATEWAY_NETWORK" \
       --format '{{range .Containers}}{{.Name}} {{end}}' \
       | grep -q "$CONTAINER"; then
    docker network connect "$GATEWAY_NETWORK" "$CONTAINER"
    echo "    Connected $CONTAINER."
  else
    echo "    $CONTAINER already on network."
  fi
done

# Step 5: Wait for gateway health
echo "==> Waiting for gateway to become healthy..."
MAX_WAIT=120
WAITED=0
until curl -sf "$HEALTH_URL" > /dev/null 2>&1; do
  if (( WAITED >= MAX_WAIT )); then
    echo "ERROR: Gateway did not become healthy after ${MAX_WAIT}s."
    exit 1
  fi
  printf "    %ds / %ds\r" "$WAITED" "$MAX_WAIT"
  sleep 3
  WAITED=$((WAITED + 3))
done
echo "    Gateway is healthy."

# Step 6: Wait for Management API
echo "==> Waiting for Management API (port 9090)..."
MAX_MGMT=90
MGMT_WAITED=0
until curl -s --max-time 3 -o /dev/null -w "%{http_code}" "http://localhost:9090/" 2>/dev/null \
      | grep -qE "^[0-9]{3}$"; do
  if (( MGMT_WAITED >= MAX_MGMT )); then
    echo "    WARNING: Port 9090 not responding after ${MAX_MGMT}s — attempting configure anyway."
    break
  fi
  printf "    %ds / %ds\r" "$MGMT_WAITED" "$MAX_MGMT"
  sleep 3
  MGMT_WAITED=$((MGMT_WAITED + 3))
done
echo "    Management API ready."

# Step 7: Configure gateway
echo "==> Configuring gateway resources..."
bash "$SCRIPT_DIR/configure-gateway.sh"

# Step 8: Wait for MCP routes to be live
echo "==> Waiting for MCP routes to be live..."
sleep 5 

# Quick smoke-test: POST an MCP initialize request to the CRM proxy
MAX_MCP_WAIT=60
MCP_WAITED=0
until curl -sf -o /dev/null -X POST "http://localhost:8080/crm/mcp" \
      -H "Content-Type: application/json" \
      -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"probe","version":"0.1"}}}' \
      2>/dev/null; do
  if (( MCP_WAITED >= MAX_MCP_WAIT )); then
    echo "WARNING: CRM MCP route not responding after ${MAX_MCP_WAIT}s — proceeding anyway."
    break
  fi
  printf "    %ds / %ds\r" "$MCP_WAITED" "$MAX_MCP_WAIT"
  sleep 3
  MCP_WAITED=$((MCP_WAITED + 3))
done
echo "    Routes appear live."

# Step 9: Run the agent
echo ""
echo "==========================================================="
echo " Running agent..."
echo "==========================================================="
echo ""

pip install -q -r "$SCRIPT_DIR/requirements.txt"
python "$SCRIPT_DIR/agent.py"
