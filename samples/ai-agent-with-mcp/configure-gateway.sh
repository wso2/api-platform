#!/usr/bin/env bash
# configure-gateway.sh
# Registers the Anthropic LLM provider, LLM proxy, and three MCP proxies
# with the WSO2 AI Gateway Management API (http://localhost:9090).
#
# Called automatically by setup.sh after the gateway is healthy.
# Requires: ANTHROPIC_API_KEY in the environment.

set -uo pipefail

# Load .env if ANTHROPIC_API_KEY is not already in the environment
if [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
  ENV_FILE="$(cd "$(dirname "$0")" && pwd)/.env"
  if [[ -f "$ENV_FILE" ]]; then
    set -o allexport; source "$ENV_FILE"; set +o allexport
  fi
fi

if [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
  echo "ERROR: ANTHROPIC_API_KEY is not set. Add it to .env."
  exit 1
fi

MGMT="http://localhost:9090/api/management/v0.9"
AUTH_HEADER="Authorization: Basic $(printf %s "${ADMIN_USERNAME:-admin}:${ADMIN_PASSWORD:-admin}" | base64 | tr -d '\r\n')"   # default admin/admin; override with ADMIN_USERNAME/ADMIN_PASSWORD

# Inbound API key clients use to call the LLM proxy.
# The gateway's api-key-auth policy checks the x-api-key header,
# which is what the Anthropic SDK sends as api_key.
INBOUND_API_KEY="${INBOUND_API_KEY:-demo-api-key}"

# Wait for management API to be ready
echo "==> Waiting for Management API to be ready..."
MAX_READY=90
READY_WAITED=0
while true; do
  HTTP=$(curl -s --max-time 5 -o /dev/null -w "%{http_code}" \
         -H "$AUTH_HEADER" "$MGMT/llm-providers" 2>/dev/null || echo "000")
  if [[ "$HTTP" =~ ^[1234] ]]; then
    echo "    Management API ready (HTTP $HTTP)."
    break
  fi
  if (( READY_WAITED >= MAX_READY )); then
    echo "ERROR: Management API not ready after ${MAX_READY}s (last HTTP: $HTTP)."
    exit 1
  fi
  printf "    %ds / %ds (HTTP %s)\r" "$READY_WAITED" "$MAX_READY" "$HTTP"
  sleep 3
  READY_WAITED=$((READY_WAITED + 3))
done

# Helper: upsert a YAML resource (DELETE existing, then POST)
# Usage: upsert_yaml "<label>" "<collection-url>" "<resource-id>" "<yaml-body>"
upsert_yaml() {
  local label="$1"
  local url="$2"
  local resource_id="$3"
  local body="$4"
  local TMPFILE
  TMPFILE=$(mktemp)

  # Delete if it already exists
  local DEL_HTTP
  DEL_HTTP=$(curl -s --max-time 10 -o /dev/null -w "%{http_code}" \
    -X DELETE "${url}/${resource_id}" \
    -H "$AUTH_HEADER" 2>/dev/null || echo "000")
  if [[ "$DEL_HTTP" =~ ^2 ]]; then
    sleep 1  
  fi

  # POST the resource
  local HTTP
  HTTP=$(printf '%s' "$body" | curl -s --max-time 15 \
    -o "$TMPFILE" -w "%{http_code}" \
    -X POST "$url" \
    -H "Content-Type: application/yaml" \
    -H "$AUTH_HEADER" \
    --data-binary @- 2>/dev/null || echo "000")
  if [[ "$HTTP" =~ ^2 ]]; then
    echo " done (HTTP $HTTP)."
  else
    echo " FAILED (HTTP $HTTP): $(cat "$TMPFILE")"
    rm -f "$TMPFILE"
    exit 1
  fi
  rm -f "$TMPFILE"
}

# Helper: upsert a JSON resource
upsert_json() {
  local label="$1"
  local url="$2"
  local resource_id="$3"
  local body="$4"
  local TMPFILE
  TMPFILE=$(mktemp)

  # Delete if it already exists
  local DEL_HTTP
  DEL_HTTP=$(curl -s --max-time 10 -o /dev/null -w "%{http_code}" \
    -X DELETE "${url}/${resource_id}" \
    -H "$AUTH_HEADER" 2>/dev/null || echo "000")
  if [[ "$DEL_HTTP" =~ ^2 ]]; then
    sleep 1
  fi

  # POST the resource
  local HTTP
  HTTP=$(printf '%s' "$body" | curl -s --max-time 15 \
    -o "$TMPFILE" -w "%{http_code}" \
    -X POST "$url" \
    -H "Content-Type: application/json" \
    -H "$AUTH_HEADER" \
    --data-binary @- 2>/dev/null || echo "000")
  if [[ "$HTTP" =~ ^2 ]]; then
    echo " done (HTTP $HTTP)."
  else
    echo " FAILED (HTTP $HTTP): $(cat "$TMPFILE")"
    rm -f "$TMPFILE"
    exit 1
  fi
  rm -f "$TMPFILE"
}

# Register resources

printf "==> Registering Anthropic LLM Provider..."
upsert_yaml "llm-provider" "$MGMT/llm-providers" "anthropic-provider" \
"apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: anthropic-provider
spec:
  displayName: Anthropic Claude Provider
  version: v1.0
  template: anthropic
  context: /anthropic
  upstream:
    url: https://api.anthropic.com
    auth:
      type: api-key
      header: x-api-key
      value: ${ANTHROPIC_API_KEY}
  accessControl:
    mode: deny_all
    exceptions:
      - path: /v1/messages
        methods: [POST]
      - path: /v1/models
        methods: [GET]"
sleep 3

printf "==> Registering LLM Proxy (claude-agent)..."
upsert_yaml "llm-proxy" "$MGMT/llm-proxies" "claude-agent-gateway" \
'apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProxy
metadata:
  name: claude-agent-gateway
spec:
  displayName: Claude Agent Gateway
  version: v1.0
  context: /claude-agent
  provider:
    id: anthropic-provider
  policies:
    - name: api-key-auth
      version: v1
      paths:
        - path: /v1/messages
          methods: [POST]
          params:
            key: x-api-key
            in: header'
sleep 3

printf "==> Registering inbound API key for LLM proxy..."
upsert_json "api-key" "$MGMT/llm-proxies/claude-agent-gateway/api-keys" "demo-api-key" \
  "{\"apiKey\": \"${INBOUND_API_KEY}\"}"
sleep 2

printf "==> Registering CRM MCP Proxy..."
upsert_yaml "crm-mcp" "$MGMT/mcp-proxies" "crm-mcp-v1.0" \
'apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Mcp
metadata:
  name: crm-mcp-v1.0
spec:
  displayName: CRM MCP Server
  version: v1.0
  context: /crm
  specVersion: "2025-06-18"
  upstream:
    url: http://crm-mcp:8080
  tools: []
  resources: []
  prompts: []'
sleep 2

printf "==> Registering Orders MCP Proxy..."
upsert_yaml "orders-mcp" "$MGMT/mcp-proxies" "orders-mcp-v1.0" \
'apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Mcp
metadata:
  name: orders-mcp-v1.0
spec:
  displayName: Orders MCP Server
  version: v1.0
  context: /orders
  specVersion: "2025-06-18"
  upstream:
    url: http://orders-mcp:8080
  tools: []
  resources: []
  prompts: []'
sleep 2

printf "==> Registering KB MCP Proxy..."
upsert_yaml "kb-mcp" "$MGMT/mcp-proxies" "kb-mcp-v1.0" \
'apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Mcp
metadata:
  name: kb-mcp-v1.0
spec:
  displayName: Knowledge Base MCP Server
  version: v1.0
  context: /kb
  specVersion: "2025-06-18"
  upstream:
    url: http://kb-mcp:8080
  tools: []
  resources: []
  prompts: []'

echo ""
echo "==> All gateway resources registered successfully."
echo "    LLM proxy inbound key: ${INBOUND_API_KEY}"
