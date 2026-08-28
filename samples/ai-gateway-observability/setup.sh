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

# Inbound API keys callers send in the `api_key` header — one per proxy.
# The gateway enforces a global unique constraint on the key value, so two proxies
# cannot share one key: the second registration comes back 409.
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

# The observability containers (Prometheus, Grafana, Jaeger, OTel collector) ship
# with the distribution but sit behind Compose profiles, so they only start when
# these profiles are requested.
COMPOSE_PROFILES=(--profile metrics --profile tracing)

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info()    { echo "[INFO]  $*"; }
success() { echo "[OK]    $*"; }
error()   { echo "[ERROR] $*" >&2; exit 1; }

wait_for_health() {
  local url="$1"
  local max_attempts=30
  local interval=5
  info "Waiting for gateway to be healthy at ${url} ..."
  for i in $(seq 1 "${max_attempts}"); do
    if curl -sf "${url}" > /dev/null 2>&1; then
      success "Gateway is healthy."
      return 0
    fi
    echo "  attempt ${i}/${max_attempts} — retrying in ${interval}s ..."
    sleep "${interval}"
  done
  error "Gateway did not become healthy after $((max_attempts * interval))s."
}

deploy_resource() {
  local kind="$1"   # llm-providers or llm-proxies
  local file="$2"
  [[ -f "${file}" ]] || error "${file} not found."
  info "Deploying ${kind} from $(basename "${file}") ..."
  local status body
  body=$(mktemp)
  status=$(curl -s -o "${body}" -w "%{http_code}" \
    -X POST "${GATEWAY_MGMT_URL}/${kind}" \
    -H "Content-Type: application/yaml" \
    -H "${AUTH_HEADER}" \
    --data-binary "@${file}")
  # Keep the response body: on a failure the gateway's own message names the cause,
  # and a bare status code sends you hunting for it.
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

# metadata.name out of one of the sample's own resource files. Deliberately not
# python+PyYAML: PyYAML is not part of a stock macOS python3, and these files are
# small and fixed in shape.
yaml_name() {
  awk '/^metadata:/ { in_meta = 1; next }
       in_meta && /^[[:space:]]+name:/ {
         sub(/^[[:space:]]*name:[[:space:]]*/, ""); print; exit
       }
       /^[^[:space:]#]/ && !/^metadata:/ { in_meta = 0 }' "$1"
}

# ---------------------------------------------------------------------------
# Step 1 — Download distribution
# ---------------------------------------------------------------------------
cd "${SCRIPT_DIR}"
info "Downloading ${DIST_ZIP} ..."
if [[ -f "${DIST_ZIP}" ]]; then
  info "Archive already exists, skipping download."
else
  # curl first: it ships with macOS and with most Linux distributions, whereas wget
  # is not installed on macOS at all by default. wget's --show-progress is a GNU
  # extension, so it is not used in the fallback either.
  if command -v curl >/dev/null 2>&1; then
    curl -fSL "${DIST_URL}" -o "${DIST_ZIP}"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "${DIST_URL}" -O "${DIST_ZIP}"
  else
    error "Neither curl nor wget is available — install one and retry."
  fi
  success "Downloaded ${DIST_ZIP}."
fi

# ---------------------------------------------------------------------------
# Step 2 — Unzip
# ---------------------------------------------------------------------------
if [[ -d "${DIST_NAME}" ]]; then
  info "Distribution directory '${DIST_NAME}' already exists, skipping unzip."
else
  info "Unzipping ${DIST_ZIP} ..."
  unzip -q "${DIST_ZIP}"
  success "Extracted to ${DIST_NAME}/."
fi

# ---------------------------------------------------------------------------
# Step 3 — Switch on the controller's metrics endpoint and tracing
#
# additional-config.toml holds two sections the distribution leaves off:
# [controller.metrics] (its endpoint defaults to disabled, so Prometheus's
# gateway-controller job would never come up) and [tracing] (which points the
# gateway at the OTel collector). The block is prepended rather than appended so
# its keys stay bound to their own sections instead of being swallowed by
# whichever section happens to end config.toml.
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
# Step 4 — Provision the Grafana dashboard and fix the scrape targets
#
# The distribution already mounts observability/grafana/dashboards into Grafana
# and provisions everything it finds there, so dropping the file in is enough.
# The compose patch below only decides which dashboard Grafana opens on load.
# ---------------------------------------------------------------------------
[[ -f "${DASHBOARD_JSON}" ]] || error "Dashboard not found at ${DASHBOARD_JSON}"
info "Copying dashboard into ${DIST_NAME}/observability/grafana/dashboards/ ..."
cp "${DASHBOARD_JSON}" "${DIST_NAME}/observability/grafana/dashboards/"
success "Dashboard provisioned."

# The distribution's prometheus.yml scrapes hosts named `policy-engine` and
# `router`, which the compose file never defines — both run inside the
# `gateway-runtime` service. Replace it so all three targets actually come up.
[[ -f "${PROMETHEUS_YML}" ]] || error "prometheus.yml not found at ${PROMETHEUS_YML}"
info "Replacing the distribution's prometheus.yml with corrected scrape targets ..."
cp "${PROMETHEUS_YML}" "${DIST_NAME}/observability/prometheus/prometheus.yml"
success "Scrape targets set."

COMPOSE_FILE="${DIST_NAME}/docker-compose.yaml"
[[ -f "${COMPOSE_FILE}" ]] || COMPOSE_FILE="${DIST_NAME}/docker-compose.yml"
[[ -f "${COMPOSE_FILE}" ]] || error "docker-compose file not found in ${DIST_NAME}/"

# Which dashboard Grafana opens on load. Shipped as a compose override file rather
# than an edit to the distribution's own compose file — docker compose merges it
# automatically, and nothing of theirs gets rewritten.
[[ -f "${COMPOSE_OVERRIDE}" ]] || error "docker-compose.override.yaml not found at ${COMPOSE_OVERRIDE}"
info "Pointing Grafana's home dashboard at the AI Gateway overview ..."
cp "${COMPOSE_OVERRIDE}" "${DIST_NAME}/docker-compose.override.yaml"
success "Grafana home dashboard set."

# ---------------------------------------------------------------------------
# Step 5 — Start the mock LLM backend
#
# WireMock stands in for the OpenAI API so the sample needs no API key and no
# network egress. Its mappings return a normal reply, a deliberately slow reply,
# and a 500 — the three shapes load.sh needs to make the dashboard interesting.
# ---------------------------------------------------------------------------
info "Starting mock LLM backend (WireMock) ..."
docker rm -f "${MOCK_CONTAINER}" >/dev/null 2>&1 || true
docker run -d --name "${MOCK_CONTAINER}" \
  -p "${MOCK_PORT}:8080" \
  -v "${SCRIPT_DIR}/wiremock/mappings:/home/wiremock/mappings" \
  wiremock/wiremock:3.3.1 >/dev/null
success "Mock LLM backend started on host port ${MOCK_PORT}."

# ---------------------------------------------------------------------------
# Step 6 — Start the stack (gateway + observability)
# ---------------------------------------------------------------------------
info "Starting Docker Compose stack in ${DIST_NAME}/ ..."
# Provision the gateway's listener cert, encryption key, api-platform.env, and admin
# credentials. The gateway ships no default admin user and fails closed without one,
# and compose requires api-platform.env to exist, so this step is not optional.
# Passing the credentials in keeps it non-interactive; it needs openssl, plus either
# htpasswd or docker to bcrypt the password.
[[ -x "${DIST_NAME}/scripts/setup.sh" ]] || error "${DIST_NAME}/scripts/setup.sh is missing — is this the ${DIST_VERSION} distribution?"
(cd "${DIST_NAME}" && ADMIN_USERNAME="${ADMIN_USERNAME:-admin}" ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin}" ./scripts/setup.sh)
(cd "${DIST_NAME}" && docker compose "${COMPOSE_PROFILES[@]}" up -d)
success "Gateway, Prometheus, Grafana, Jaeger and the OTel collector are starting."

# ---------------------------------------------------------------------------
# Step 7 — Health check
# ---------------------------------------------------------------------------
wait_for_health "${GATEWAY_HEALTH_URL}"

# ---------------------------------------------------------------------------
# Step 8 — Put the mock backend on the gateway's network
# ---------------------------------------------------------------------------
# Ask the running controller which network it is on, rather than matching on name.
# The distribution pins a per-copy COMPOSE_PROJECT_NAME, so the network is named
# <project>_gateway-network — and a leftover stack from an earlier run has a network
# whose name matches just as well. Connecting the mock to the wrong one leaves the
# proxies unable to reach their backend, with everything else looking fine.
CONTROLLER_CID=$(cd "${DIST_NAME}" && docker compose "${COMPOSE_PROFILES[@]}" ps -q gateway-controller 2>/dev/null | head -1)
[[ -n "${CONTROLLER_CID}" ]] || error "Could not find the running gateway-controller container."
GATEWAY_NETWORK=$(docker inspect -f '{{range $k, $v := .NetworkSettings.Networks}}{{println $k}}{{end}}' "${CONTROLLER_CID}" \
  | grep 'gateway-network' | head -1)
[[ -n "${GATEWAY_NETWORK}" ]] || error "The gateway-controller is not attached to a gateway network."
docker network connect "${GATEWAY_NETWORK}" "${MOCK_CONTAINER}" 2>/dev/null || true
success "Connected ${MOCK_CONTAINER} to network ${GATEWAY_NETWORK}."

# ---------------------------------------------------------------------------
# Step 9 — Deploy providers and proxies
#
# Two providers share the same mock upstream: one plain, one with a token budget.
# That gives the dashboard two proxies whose behaviour differs under load —
# /support starts returning 429 once its budget is spent, /assistant keeps serving.
# ---------------------------------------------------------------------------
for PROVIDER_YAML in "${PROVIDER_YAMLS[@]}"; do
  deploy_resource "llm-providers" "${PROVIDER_YAML}"
done

for PROXY_YAML in "${PROXY_YAMLS[@]}"; do
  deploy_resource "llm-proxies" "${PROXY_YAML}"
done

# ---------------------------------------------------------------------------
# Step 10 — Register the inbound API key on each proxy
# ---------------------------------------------------------------------------
register_api_key() {
  local proxy_yaml="$1" api_key="$2"
  local proxy_name status body detail
  proxy_name=$(yaml_name "${proxy_yaml}")
  info "Registering inbound API key for ${proxy_name} ..."
  body=$(mktemp)
  status=$(curl -s -o "${body}" -w "%{http_code}" \
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
