#!/bin/sh
set -eu

# Initialise tput colors if available
if command -v tput >/dev/null 2>&1 && [ -n "${TERM:-}" ] && tput setaf 2 >/dev/null 2>&1; then
  GREEN="$(tput setaf 2)"; RESET="$(tput sgr0)"
else
  GREEN=""; RESET=""
fi

print_ok() {
  echo "${GREEN}✔  $1${RESET}"
}

print_info() {
  echo "-->  $1"
}

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ -f "${SCRIPT_DIR}/.env" ]; then
  . "${SCRIPT_DIR}/.env"
fi

MGMT_PORT="${MGMT_PORT:-9090}"
INBOUND_API_KEY="${INBOUND_API_KEY:-demo-unlocked-sample-key}"
MGMT_BASE="http://localhost:${MGMT_PORT}/api/management/v0.9"
AUTH="Authorization: Basic YWRtaW46YWRtaW4="

print_info "Registering LLM Provider (upstream: mock-llm-openai, policy: token-based-ratelimit)..."
curl -sf -X POST "${MGMT_BASE}/llm-providers" \
  -H "Content-Type: application/yaml" \
  -H "${AUTH}" \
  --data-binary @"${SCRIPT_DIR}/provider.yaml"
echo ""
print_ok "LLM Provider registered"

sleep 3

print_info "Registering LLM Proxy (context: /assistant, policy: api-key-auth)..."
curl -sf -X POST "${MGMT_BASE}/llm-proxies" \
  -H "Content-Type: application/yaml" \
  -H "${AUTH}" \
  --data-binary @"${SCRIPT_DIR}/proxy.yaml"
echo ""
print_ok "LLM Proxy registered"

sleep 3

print_info "Registering inbound API key for proxy 'openai-assistant'..."
curl -sf -X POST "${MGMT_BASE}/llm-proxies/openai-assistant/api-keys" \
  -H "Content-Type: application/json" \
  -H "${AUTH}" \
  -d "{\"apiKey\": \"${INBOUND_API_KEY}\"}"
echo ""
print_ok "API key registered"
