#!/bin/sh
set -eu

# Initialise tput colors if available
if command -v tput >/dev/null 2>&1 && [ -n "${TERM:-}" ] && tput setaf 2 >/dev/null 2>&1; then
  GREEN="$(tput setaf 2)"
  YELLOW="$(tput setaf 3)"
  RED="$(tput setaf 1)"
  BOLD="$(tput bold)"
  RESET="$(tput sgr0)"
else
  GREEN=""; YELLOW=""; RED=""; BOLD=""; RESET=""
fi

print_ok() {
  echo "${GREEN}✔  $1${RESET}"
}

print_info() {
  echo "-->  $1"
}

print_warn() {
  echo "${YELLOW}⚠   $1${RESET}"
}

print_error() {
  echo "${RED}✖  $1${RESET}"
}

print_title() {
  echo ""
  echo "${BOLD}${GREEN}=== $1 ===${RESET}"
  echo ""
}

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

if [ ! -f "${SCRIPT_DIR}/.env" ]; then
  if [ -f "${SCRIPT_DIR}/.env.example" ]; then
    cp "${SCRIPT_DIR}/.env.example" "${SCRIPT_DIR}/.env"
    print_ok "Created .env from .env.example"
  else
    print_warn "No .env or .env.example found — using built-in defaults."
  fi
fi

if [ -f "${SCRIPT_DIR}/.env" ]; then
  . "${SCRIPT_DIR}/.env"
fi

MGMT_PORT="${MGMT_PORT:-9090}"
HEALTH_PORT="${HEALTH_PORT:-9094}"
TRAFFIC_PORT="${TRAFFIC_PORT:-8443}"

MAX_RETRIES="${MAX_RETRIES:-30}"
RETRY_INTERVAL=2

ZIP_URL="https://github.com/wso2/api-platform/releases/download/ai-gateway/v1.1.0/wso2apip-ai-gateway-1.1.0.zip"
ZIP_FILE="${SCRIPT_DIR}/wso2apip-ai-gateway-1.1.0.zip"
BUNDLE_DIR="${SCRIPT_DIR}/wso2apip-ai-gateway-1.1.0"

print_title "Downloading WSO2 AI Gateway"
if [ -f "$ZIP_FILE" ]; then
  print_info "Distribution zip already exists, skipping download."
else
  print_info "Downloading official distribution package..."
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$ZIP_URL" -o "$ZIP_FILE"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$ZIP_URL" -O "$ZIP_FILE"
  else
    print_error "Neither curl nor wget found. Please install one and retry."
    exit 1
  fi
fi
print_info "Unzipping distribution..."
unzip -o "$ZIP_FILE" -d "${SCRIPT_DIR}"

print_title "Configuring Gateway for JWT Auth"
CONFIG_FILE="${BUNDLE_DIR}/configs/config.toml"
print_info "Injecting mock-jwks keymanager into gateway config..."
cat >> "$CONFIG_FILE" << 'TOML'

[[policy_configurations.jwtauth_v1.keymanagers]]
name = "mock-jwks"
issuer = "http://mock-mcp-reading-list:8080/token"

[policy_configurations.jwtauth_v1.keymanagers.jwks.remote]
uri = "http://mock-mcp-reading-list:8080/jwks"
TOML
print_ok "Keymanager injected into ${CONFIG_FILE}"

print_title "Starting Containers"
print_info "Starting WireMock mock MCP backend..."
docker rm -f mock-mcp-reading-list 2>/dev/null || true
docker run -d --name mock-mcp-reading-list \
  -p 8082:8080 \
  -v "${SCRIPT_DIR}/wiremock/mappings:/home/wiremock/mappings" \
  wiremock/wiremock:3.3.1 \
  --global-response-templating
print_ok "Mock MCP backend started (WireMock on host port 8082)"

print_info "Booting WSO2 AI Gateway stack..."
cd "${BUNDLE_DIR}"
docker compose up -d
print_ok "Gateway stack started"

print_title "Waiting for Gateway"
print_info "Waiting for gateway controller to be ready..."
retries=0
until [ "$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:${HEALTH_PORT:-9094}/health")" = "200" ]; do
  retries=$((retries + 1))
  if [ "$retries" -ge "$MAX_RETRIES" ]; then
    print_error "Gateway controller did not become healthy after $((MAX_RETRIES * RETRY_INTERVAL))s. Check: docker compose logs"
    exit 1
  fi
  sleep "$RETRY_INTERVAL"
done
print_ok "Gateway controller is healthy"

print_info "Connecting mock MCP backend to gateway network..."
GATEWAY_NETWORK=$(docker network ls --filter name=gateway-network --format "{{.Name}}" | head -1)
if [ -n "$GATEWAY_NETWORK" ]; then
  docker network connect "$GATEWAY_NETWORK" mock-mcp-reading-list 2>/dev/null || true
  print_ok "Connected mock-mcp-reading-list to network: ${GATEWAY_NETWORK}"
else
  print_warn "Could not detect gateway network — mock routing may not work."
fi

print_title "Registering Resources"
sh "${SCRIPT_DIR}/inject-mock.sh"

print_title "Waiting for Routes"
print_info "Polling gateway MCP endpoint until routes are live..."
retries=0
until [ "$(curl -sk -o /dev/null -w '%{http_code}' -X POST \
  "https://localhost:${TRAFFIC_PORT:-8443}/reading-list/mcp" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"probe","version":"1.0"}}}')" = "401" ]; do
  retries=$((retries + 1))
  if [ "$retries" -ge "$MAX_RETRIES" ]; then
    print_error "Routes did not become live after $((MAX_RETRIES * RETRY_INTERVAL))s. Check: docker compose logs"
    exit 1
  fi
  sleep "$RETRY_INTERVAL"
done
print_ok "Routes are live"

echo ""
print_ok "Setup complete."
echo ""
echo "  Run tests : sh test.sh"
echo "  Claude Desktop : sh configure-claude.sh"
echo ""
