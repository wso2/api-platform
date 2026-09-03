#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
DIST_VERSION="1.2.0"
DIST_NAME="wso2apip-ai-gateway-${DIST_VERSION}"
DIST_ZIP="${DIST_NAME}.zip"
DIST_URL="https://github.com/wso2/api-platform/releases/download/ai-gateway/v${DIST_VERSION}/${DIST_ZIP}"

GATEWAY_MGMT_URL="http://localhost:9090/api/management/v1"
GATEWAY_HEALTH_URL="http://localhost:9094/health"
AUTH_HEADER="Authorization: Basic $(printf %s "${ADMIN_USERNAME:-admin}:${ADMIN_PASSWORD:-admin}" | base64 | tr -d '\r\n')"   # default admin/admin; override with ADMIN_USERNAME/ADMIN_PASSWORD

# Inbound API keys callers send in the `api_key` header, one per proxy.
ASSISTANT_API_KEY="${ASSISTANT_API_KEY:-demo-assistant-key}"
SUPPORT_API_KEY="${SUPPORT_API_KEY:-demo-support-key}"

MOCK_CONTAINER="mock-llm-openai"
MOCK_PORT="${MOCK_PORT:-8082}"

# Observability UIs exposed by the distribution's docker-compose.
GRAFANA_URL="http://localhost:3000"
JAEGER_URL="http://localhost:16686"
PROMETHEUS_URL="http://localhost:9092"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROVIDER_YAMLS=("${SCRIPT_DIR}/llm-provider.yaml" "${SCRIPT_DIR}/llm-provider-budgeted.yaml")
PROXY_YAMLS=("${SCRIPT_DIR}/llm-proxy-assistant.yaml" "${SCRIPT_DIR}/llm-proxy-support.yaml")
DASHBOARD_JSON="${SCRIPT_DIR}/observability/ai-gateway-overview.json"
PROMETHEUS_YML="${SCRIPT_DIR}/observability/prometheus.yml"
COMPOSE_OVERRIDE="${SCRIPT_DIR}/observability/docker-compose.override.yaml"
ADDITIONAL_CONFIG="${SCRIPT_DIR}/additional-config.toml"

# Prometheus, Grafana, Jaeger and the OTel collector sit behind these Compose profiles.
COMPOSE_PROFILES=(--profile metrics --profile tracing)

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info()    { echo "[INFO]  $*"; }
success() { echo "[OK]    $*"; }
error()   { echo "[ERROR] $*" >&2; exit 1; }

# Polls a health endpoint until it answers, giving up after a fixed number of tries.
wait_for_health() {
  local url="$1"
  local max_attempts=30
  local interval=5
  info "Waiting for gateway to be healthy at ${url} ..."
  for i in $(seq 1 "${max_attempts}"); do
    if curl -sf --connect-timeout 5 --max-time 10 "${url}" > /dev/null 2>&1; then
      success "Gateway is healthy."
      return 0
    fi
    echo "  attempt ${i}/${max_attempts} — retrying in ${interval}s ..."
    sleep "${interval}"
  done
  error "Gateway did not become healthy after $((max_attempts * interval))s."
}

# POSTs a resource YAML to the management API and reports the outcome by HTTP status.
deploy_resource() {
  local kind="$1"   # llm-providers or llm-proxies
  local file="$2"
  [[ -f "${file}" ]] || error "${file} not found."
  info "Deploying ${kind} from $(basename "${file}") ..."
  local status body
  body=$(mktemp)
  status=$(curl -s -o "${body}" -w "%{http_code}" \
    --connect-timeout 5 --max-time 30 \
    -X POST "${GATEWAY_MGMT_URL}/${kind}" \
    -H "Content-Type: application/yaml" \
    -H "${AUTH_HEADER}" \
    --data-binary "@${file}")
  local detail
  detail=$(cat "${body}"); rm -f "${body}"
  if [[ "${status}" =~ ^2 ]]; then
    success "Deployed $(basename "${file}") (HTTP ${status})."
  elif [[ "${status}" == "409" ]]; then
    error "$(basename "${file}") is already deployed on this gateway (HTTP 409).
        Gateway said: ${detail}
        Run ./teardown.sh first, then ./setup.sh — teardown drops the gateway's
        database volume, which is what clears previously registered resources."
  else
    error "Failed to deploy $(basename "${file}") (HTTP ${status}).
        Gateway said: ${detail}"
  fi
}

# Reads metadata.name out of one of the sample's resource files.
yaml_name() {
  awk '/^metadata:/ { in_meta = 1; next }
       in_meta && /^[[:space:]]+name:/ {
         sub(/^[[:space:]]*name:[[:space:]]*/, ""); print; exit
       }
       /^[^[:space:]#]/ && !/^metadata:/ { in_meta = 0 }' "$1"
}

# ---------------------------------------------------------------------------
# Step 1 - Download distribution
# ---------------------------------------------------------------------------
cd "${SCRIPT_DIR}"
info "Downloading ${DIST_ZIP} ..."
if [[ -f "${DIST_ZIP}" ]]; then
  info "Archive already exists, skipping download."
else
  if command -v curl >/dev/null 2>&1; then
    curl -fSL --connect-timeout 10 --max-time 300 "${DIST_URL}" -o "${DIST_ZIP}"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "${DIST_URL}" -O "${DIST_ZIP}"
  else
    error "Neither curl nor wget is available — install one and retry."
  fi
  success "Downloaded ${DIST_ZIP}."
fi

# ---------------------------------------------------------------------------
# Step 2 - Unzip
# ---------------------------------------------------------------------------
if [[ -d "${DIST_NAME}" ]]; then
  info "Distribution directory '${DIST_NAME}' already exists, skipping unzip."
else
  info "Unzipping ${DIST_ZIP} ..."
  unzip -q "${DIST_ZIP}"
  success "Extracted to ${DIST_NAME}/."
fi

# ---------------------------------------------------------------------------
# Step 3 - Enable the metrics endpoints and tracing
#
# additional-config.toml is prepended to the gateway's config.toml so each setting
# stays under its own section header.
# ---------------------------------------------------------------------------
GATEWAY_CONFIG="${DIST_NAME}/configs/config.toml"
[[ -f "${ADDITIONAL_CONFIG}" ]] || error "additional-config.toml not found at ${ADDITIONAL_CONFIG}"
[[ -f "${GATEWAY_CONFIG}" ]]    || error "Gateway config.toml not found at ${GATEWAY_CONFIG}"

if grep -q '^\[tracing\]' "${GATEWAY_CONFIG}"; then
  info "Observability config already merged into ${GATEWAY_CONFIG}, skipping."
else
  info "Merging ${ADDITIONAL_CONFIG} into ${GATEWAY_CONFIG} ..."
  TMP_MERGED=$(mktemp)
  { cat "${ADDITIONAL_CONFIG}"; echo ""; cat "${GATEWAY_CONFIG}"; } > "${TMP_MERGED}"
  mv "${TMP_MERGED}" "${GATEWAY_CONFIG}"
  success "Controller metrics and tracing enabled in gateway config."
fi

# ---------------------------------------------------------------------------
# Step 4 - Provision the Grafana dashboard and the scrape targets
#
# Grafana provisions whatever it finds in its dashboards folder, so copying the
# file in is enough.
# ---------------------------------------------------------------------------
[[ -f "${DASHBOARD_JSON}" ]] || error "Dashboard not found at ${DASHBOARD_JSON}"
info "Copying dashboard into ${DIST_NAME}/observability/grafana/dashboards/ ..."
cp "${DASHBOARD_JSON}" "${DIST_NAME}/observability/grafana/dashboards/"
success "Dashboard provisioned."

# The policy engine and the Envoy router both run inside `gateway-runtime`, which is
# the hostname this prometheus.yml scrapes them at.
[[ -f "${PROMETHEUS_YML}" ]] || error "prometheus.yml not found at ${PROMETHEUS_YML}"
info "Replacing the distribution's prometheus.yml with corrected scrape targets ..."
cp "${PROMETHEUS_YML}" "${DIST_NAME}/observability/prometheus/prometheus.yml"
success "Scrape targets set."

COMPOSE_FILE="${DIST_NAME}/docker-compose.yaml"
[[ -f "${COMPOSE_FILE}" ]] || COMPOSE_FILE="${DIST_NAME}/docker-compose.yml"
[[ -f "${COMPOSE_FILE}" ]] || error "docker-compose file not found in ${DIST_NAME}/"

# Sets which dashboard Grafana opens on load. Compose merges override files
# automatically, so the distribution's own compose file is left untouched.
[[ -f "${COMPOSE_OVERRIDE}" ]] || error "docker-compose.override.yaml not found at ${COMPOSE_OVERRIDE}"
info "Pointing Grafana's home dashboard at the AI Gateway overview ..."
cp "${COMPOSE_OVERRIDE}" "${DIST_NAME}/docker-compose.override.yaml"
success "Grafana home dashboard set."

# ---------------------------------------------------------------------------
# Step 5 - Start the mock LLM backend
#
# WireMock stands in for the OpenAI API, so no API key or network access is needed.
# Its mappings return a normal reply, a slow reply and a 500.
# ---------------------------------------------------------------------------
info "Starting mock LLM backend (WireMock) ..."
docker rm -f "${MOCK_CONTAINER}" >/dev/null 2>&1 || true
docker run -d --name "${MOCK_CONTAINER}" \
  -p "${MOCK_PORT}:8080" \
  -v "${SCRIPT_DIR}/wiremock/mappings:/home/wiremock/mappings" \
  wiremock/wiremock:3.3.1 >/dev/null
success "Mock LLM backend started on host port ${MOCK_PORT}."

# ---------------------------------------------------------------------------
# Step 6 - Start the stack (gateway + observability)
# ---------------------------------------------------------------------------
info "Starting Docker Compose stack in ${DIST_NAME}/ ..."
# The distribution's own setup script provisions its listener cert, encryption key,
# api-platform.env and admin credentials. Credentials are passed in to keep it
# non-interactive.
[[ -x "${DIST_NAME}/scripts/setup.sh" ]] || error "${DIST_NAME}/scripts/setup.sh is missing — is this the ${DIST_VERSION} distribution?"
(cd "${DIST_NAME}" && ADMIN_USERNAME="${ADMIN_USERNAME:-admin}" ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin}" ./scripts/setup.sh)
(cd "${DIST_NAME}" && docker compose "${COMPOSE_PROFILES[@]}" up -d)
success "Gateway, Prometheus, Grafana, Jaeger and the OTel collector are starting."

# ---------------------------------------------------------------------------
# Step 7 - Health check
# ---------------------------------------------------------------------------
wait_for_health "${GATEWAY_HEALTH_URL}"

# ---------------------------------------------------------------------------
# Step 8 - Put the mock backend on the gateway's network
# ---------------------------------------------------------------------------
# Resolve the network from the running controller rather than guessing its name.
CONTROLLER_CID=$(cd "${DIST_NAME}" && docker compose "${COMPOSE_PROFILES[@]}" ps -q gateway-controller 2>/dev/null | head -1)
[[ -n "${CONTROLLER_CID}" ]] || error "Could not find the running gateway-controller container."
GATEWAY_NETWORK=$(docker inspect -f '{{range $k, $v := .NetworkSettings.Networks}}{{println $k}}{{end}}' "${CONTROLLER_CID}" \
  | grep 'gateway-network' | head -1)
[[ -n "${GATEWAY_NETWORK}" ]] || error "The gateway-controller is not attached to a gateway network."
docker network connect "${GATEWAY_NETWORK}" "${MOCK_CONTAINER}" 2>/dev/null || true
success "Connected ${MOCK_CONTAINER} to network ${GATEWAY_NETWORK}."

# ---------------------------------------------------------------------------
# Step 9 - Deploy providers and proxies
#
# Two providers share the mock upstream: one plain, one with a token budget. Under
# load /support starts returning 429 while /assistant keeps serving.
# ---------------------------------------------------------------------------
for PROVIDER_YAML in "${PROVIDER_YAMLS[@]}"; do
  deploy_resource "llm-providers" "${PROVIDER_YAML}"
done

for PROXY_YAML in "${PROXY_YAMLS[@]}"; do
  deploy_resource "llm-proxies" "${PROXY_YAML}"
done

# ---------------------------------------------------------------------------
# Step 10 - Register the inbound API key on each proxy
# ---------------------------------------------------------------------------
# Registers an inbound API key on a proxy, naming the cause when the gateway rejects it.
register_api_key() {
  local proxy_yaml="$1" api_key="$2"
  local proxy_name status body detail
  proxy_name=$(yaml_name "${proxy_yaml}")
  info "Registering inbound API key for ${proxy_name} ..."
  body=$(mktemp)
  status=$(curl -s -o "${body}" -w "%{http_code}" \
    --connect-timeout 5 --max-time 30 \
    -X POST "${GATEWAY_MGMT_URL}/llm-proxies/${proxy_name}/api-keys" \
    -H "Content-Type: application/json" \
    -H "${AUTH_HEADER}" \
    -d "{\"apiKey\": \"${api_key}\"}")
  detail=$(cat "${body}"); rm -f "${body}"
  if [[ "${status}" =~ ^2 ]]; then
    success "API key registered for ${proxy_name} (HTTP ${status})."
  elif [[ "${status}" == "409" ]]; then
    error "That API key value is already registered on this gateway (HTTP 409).
        Gateway said: ${detail}
        Key values are unique gateway-wide. Run ./teardown.sh, then ./setup.sh."
  else
    error "Failed to register API key for ${proxy_name} (HTTP ${status}).
        Gateway said: ${detail}"
  fi
}

register_api_key "${SCRIPT_DIR}/llm-proxy-assistant.yaml" "${ASSISTANT_API_KEY}"
register_api_key "${SCRIPT_DIR}/llm-proxy-support.yaml"   "${SUPPORT_API_KEY}"

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
echo ""
echo "============================================================"
echo " Setup complete!"
echo ""
echo " Proxy endpoints:"
echo "   Assistant : http://localhost:8080/assistant/chat/completions"
echo "               api_key: ${ASSISTANT_API_KEY}"
echo "   Support   : http://localhost:8080/support/chat/completions   (token budget)"
echo "               api_key: ${SUPPORT_API_KEY}"
echo ""
echo " Observability:"
echo "   Grafana    : ${GRAFANA_URL}      (admin / admin)"
echo "   Jaeger     : ${JAEGER_URL}"
echo "   Prometheus : ${PROMETHEUS_URL}"
echo ""
echo " Next:"
echo "   ./load.sh     # generate ~1 minute of traffic, then watch the dashboard"
echo "   ./test.sh     # assert metrics and traces are actually flowing"
echo "============================================================"
