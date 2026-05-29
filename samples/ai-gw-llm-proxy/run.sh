#!/bin/sh
set -eu

print_ok() {
  tput setaf 2
  echo "✔  $1"
  tput sgr0
}

print_info() {
  echo "-->  $1"
}

print_warn() {
  tput setaf 3
  echo "⚠   $1"
  tput sgr0
}

print_title() {
  echo
  tput bold
  tput setaf 2
  echo "=== $1 ==="
  tput sgr0
  echo
}

if [ -f .env ]; then
  . .env
else
  print_warn "No .env found — copy .env.example to .env, or defaults will be used."
  INBOUND_API_KEY="${INBOUND_API_KEY:-demo-unlocked-sample-key}"
  MGMT_PORT="${MGMT_PORT:-9090}"
  HEALTH_PORT="${HEALTH_PORT:-9094}"
  TRAFFIC_PORT="${TRAFFIC_PORT:-8443}"
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

print_title "Downloading WSO2 AI Gateway"
print_info "Downloading official distribution package..."
wget -qN https://github.com/wso2/api-platform/releases/download/ai-gateway/v1.1.0/wso2apip-ai-gateway-1.1.0.zip
print_info "Unzipping distribution..."
unzip -o wso2apip-ai-gateway-1.1.0.zip

print_title "Starting Containers"
print_info "Starting WireMock mock LLM backend..."
docker rm -f mock-llm-openai 2>/dev/null || true
docker run -d --name mock-llm-openai \
  -p 8082:8080 \
  -v "${SCRIPT_DIR}/wiremock/mappings:/home/wiremock/mappings" \
  wiremock/wiremock:3.3.1
print_ok "Mock LLM backend started (WireMock on host port 8082)"

print_info "Booting WSO2 AI Gateway stack..."
cd "${SCRIPT_DIR}/wso2apip-ai-gateway-1.1.0/"
docker compose up -d
print_ok "Gateway stack started"

print_title "Waiting for Gateway"
print_info "Waiting for gateway controller to be ready..."
until curl -s "http://localhost:${HEALTH_PORT:-9094}/health" > /dev/null 2>&1; do
  sleep 2
done
print_ok "Gateway controller is healthy"

print_info "Connecting mock LLM backend to gateway network..."
GATEWAY_NETWORK=$(docker network ls --filter name=gateway-network --format "{{.Name}}" | head -1)
if [ -n "$GATEWAY_NETWORK" ]; then
  docker network connect "$GATEWAY_NETWORK" mock-llm-openai 2>/dev/null || true
  print_ok "Connected mock-llm-openai to network: ${GATEWAY_NETWORK}"
else
  print_warn "Could not detect gateway network — mock routing may not work."
fi

print_title "Registering Resources"
sh "${SCRIPT_DIR}/inject-mock.sh"

print_title "Waiting for Routes"
print_info "Polling gateway traffic endpoint until routes are live..."
TRAFFIC_PORT="${TRAFFIC_PORT:-8443}"
INBOUND_API_KEY="${INBOUND_API_KEY:-demo-unlocked-sample-key}"
until [ "$(curl -sk -o /dev/null -w '%{http_code}' -X POST \
  "https://localhost:${TRAFFIC_PORT}/assistant/chat/completions" \
  -H "api_key: route-probe-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping"}]}')" = "401" ]; do
  sleep 2
done
print_ok "Routes are live"

echo ""
print_ok "Setup complete. Run: sh test.sh"
