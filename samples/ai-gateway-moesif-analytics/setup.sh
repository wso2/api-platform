#!/usr/bin/env bash
set -euo pipefail

DIST_VERSION="1.2.0"
DIST_NAME="wso2apip-ai-gateway-${DIST_VERSION}"
DIST_ZIP="${DIST_NAME}.zip"
DIST_URL="https://github.com/wso2/api-platform/releases/download/ai-gateway/v${DIST_VERSION}/${DIST_ZIP}"

GATEWAY_MGMT_URL="http://localhost:9090/api/management/v1"
GATEWAY_HEALTH_URL="http://localhost:9094/health"
AUTH_HEADER="Authorization: Basic $(printf %s "${ADMIN_USERNAME:-admin}:${ADMIN_PASSWORD:-admin}" | base64 | tr -d '\r\n')"

ASSISTANT_API_KEY="${ASSISTANT_API_KEY:-demo-assistant-key}"
SUPPORT_API_KEY="${SUPPORT_API_KEY:-demo-support-key}"

MOCK_CONTAINER="mock-llm-openai"
MOCK_PORT="${MOCK_PORT:-8082}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROVIDER_YAMLS=("${SCRIPT_DIR}/llm-provider.yaml")
PROXY_YAMLS=("${SCRIPT_DIR}/llm-proxy-assistant.yaml" "${SCRIPT_DIR}/llm-proxy-support.yaml")
LLM_COST_CONFIG="${SCRIPT_DIR}/llm-cost-config.toml"
COMPOSE_OVERRIDE="${SCRIPT_DIR}/docker-compose.override.yaml"

info()    { echo "[INFO]  $*"; }
success() { echo "[OK]    $*"; }
error()   { echo "[ERROR] $*" >&2; exit 1; }

require_file() {
  [[ -f "$1" ]] || error "Required file not found: $1"
}

wait_for_health() {
  local url="$1" max_attempts=30 interval=5
  info "Waiting for the gateway at ${url} ..."
  for i in $(seq 1 "${max_attempts}"); do
    if curl -sf --connect-timeout 5 --max-time 10 "${url}" >/dev/null 2>&1; then
      success "Gateway is healthy."
      return 0
    fi
    echo "  attempt ${i}/${max_attempts}, retrying in ${interval}s ..."
    sleep "${interval}"
  done
  error "Gateway did not become healthy after $((max_attempts * interval))s."
}

# Reads metadata.name from a resource file.
yaml_name() {
  awk '/^metadata:/ { in_meta = 1; next }
       in_meta && /^[[:space:]]+name:/ {
         sub(/^[[:space:]]*name:[[:space:]]*/, ""); print; exit
       }
       /^[^[:space:]#]/ && !/^metadata:/ { in_meta = 0 }' "$1"
}

deploy_resource() {
  local kind="$1" file="$2" status body detail
  require_file "${file}"
  info "Deploying ${kind} from $(basename "${file}") ..."
  body=$(mktemp)
  status=$(curl -s -o "${body}" -w "%{http_code}" \
    --connect-timeout 5 --max-time 30 \
    -X POST "${GATEWAY_MGMT_URL}/${kind}" \
    -H "Content-Type: application/yaml" \
    -H "${AUTH_HEADER}" \
    --data-binary "@${file}")
  detail=$(cat "${body}"); rm -f "${body}"

  case "${status}" in
    2*)
      success "Deployed $(basename "${file}") (HTTP ${status})."
      ;;
    409)
      error "$(basename "${file}") is already deployed (HTTP ${status}).
        Gateway said: ${detail}
        Run ./teardown.sh before ./setup.sh."
      ;;
    *)
      error "Failed to deploy $(basename "${file}") (HTTP ${status}).
        Gateway said: ${detail}"
      ;;
  esac
}

register_api_key() {
  local proxy_yaml="$1" api_key="$2" proxy_name status body detail
  proxy_name=$(yaml_name "${proxy_yaml}")
  info "Registering the inbound API key for ${proxy_name} ..."
  body=$(mktemp)
  status=$(curl -s -o "${body}" -w "%{http_code}" \
    --connect-timeout 5 --max-time 30 \
    -X POST "${GATEWAY_MGMT_URL}/llm-proxies/${proxy_name}/api-keys" \
    -H "Content-Type: application/json" \
    -H "${AUTH_HEADER}" \
    -d "$(jq -nc --arg k "${api_key}" '{apiKey: $k}')")
  detail=$(cat "${body}"); rm -f "${body}"

  case "${status}" in
    2*)
      success "API key registered for ${proxy_name} (HTTP ${status})."
      ;;
    409)
      error "That API key value is already registered (HTTP ${status}).
        Gateway said: ${detail}
        Key values are unique per gateway. Run ./teardown.sh before ./setup.sh."
      ;;
    *)
      error "Failed to register the API key for ${proxy_name} (HTTP ${status}).
        Gateway said: ${detail}"
      ;;
  esac
}

cd "${SCRIPT_DIR}"

# --- Moesif Application ID -------------------------------------------------
if [[ -z "${MOESIF_APPLICATION_ID:-}" && -f "${SCRIPT_DIR}/.env" ]]; then
  # shellcheck disable=SC1091
  set -a; source "${SCRIPT_DIR}/.env"; set +a
fi

if [[ -z "${MOESIF_APPLICATION_ID:-}" ]]; then
  error "MOESIF_APPLICATION_ID is not set.

        Copy the Application ID from your Moesif portal (Settings > Installation),
        then either:

          cp .env.example .env     # and paste the ID into .env
          export MOESIF_APPLICATION_ID='<your-application-id>'

        See the README for details."
fi
export MOESIF_APPLICATION_ID
success "Moesif Application ID found."

# --- Distribution ----------------------------------------------------------
if [[ -f "${DIST_ZIP}" ]]; then
  info "${DIST_ZIP} already present, skipping download."
elif command -v curl >/dev/null 2>&1; then
  info "Downloading ${DIST_ZIP} ..."
  curl -fSL --connect-timeout 10 --max-time 300 "${DIST_URL}" -o "${DIST_ZIP}"
elif command -v wget >/dev/null 2>&1; then
  info "Downloading ${DIST_ZIP} ..."
  wget -q "${DIST_URL}" -O "${DIST_ZIP}"
else
  error "Neither curl nor wget is available."
fi

if [[ -d "${DIST_NAME}" ]]; then
  info "${DIST_NAME}/ already present, skipping unzip."
else
  info "Extracting ${DIST_ZIP} ..."
  unzip -q "${DIST_ZIP}"
  success "Extracted to ${DIST_NAME}/."
fi

GATEWAY_CONFIG="${DIST_NAME}/configs/config.toml"
require_file "${GATEWAY_CONFIG}"

# --- Analytics configuration -----------------------------------------------
info "Checking the analytics configuration ..."
grep -q '^\[analytics\]' "${GATEWAY_CONFIG}" \
  || error "No [analytics] section in ${GATEWAY_CONFIG}."
grep -q 'enabled_publishers[[:space:]]*=.*moesif' "${GATEWAY_CONFIG}" \
  || error "moesif is not listed in enabled_publishers in ${GATEWAY_CONFIG}."
grep -q 'APIP_GW_ANALYTICS_PUBLISHERS_MOESIF_APPLICATION_ID' "${GATEWAY_CONFIG}" \
  || error "${GATEWAY_CONFIG} does not read the Application ID from the environment."
success "Analytics is enabled with Moesif as the publisher."

# --- Cost pricing ----------------------------------------------------------
require_file "${LLM_COST_CONFIG}"
if grep -q '^\[policy_configurations\.llm_cost_v1\]' "${GATEWAY_CONFIG}"; then
  info "Cost pricing already configured, skipping."
else
  info "Configuring cost pricing ..."
  { echo ""; cat "${LLM_COST_CONFIG}"; } >> "${GATEWAY_CONFIG}"
  success "Cost pricing configured."
fi

# --- Compose override ------------------------------------------------------
require_file "${COMPOSE_OVERRIDE}"
cp "${COMPOSE_OVERRIDE}" "${DIST_NAME}/docker-compose.override.yaml"
success "Compose override installed."

# --- Mock model backend ----------------------------------------------------
info "Starting the mock model backend ..."
docker rm -f "${MOCK_CONTAINER}" >/dev/null 2>&1 || true
docker run -d --name "${MOCK_CONTAINER}" \
  -p "${MOCK_PORT}:8080" \
  -v "${SCRIPT_DIR}/wiremock/mappings:/home/wiremock/mappings" \
  wiremock/wiremock:3.3.1 >/dev/null
success "Mock model backend running on port ${MOCK_PORT}."

# --- Gateway ---------------------------------------------------------------
[[ -x "${DIST_NAME}/scripts/setup.sh" ]] \
  || error "${DIST_NAME}/scripts/setup.sh is missing."

info "Starting the gateway ..."
(cd "${DIST_NAME}" \
  && ADMIN_USERNAME="${ADMIN_USERNAME:-admin}" ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin}" ./scripts/setup.sh)
(cd "${DIST_NAME}" && docker compose up -d)

wait_for_health "${GATEWAY_HEALTH_URL}"

# --- Attach the mock backend to the gateway network ------------------------
CONTROLLER_CID=$(cd "${DIST_NAME}" && docker compose ps -q gateway-controller 2>/dev/null | head -1)
[[ -n "${CONTROLLER_CID}" ]] || error "The gateway-controller container is not running."

GATEWAY_NETWORK=$(docker inspect -f '{{range $k, $v := .NetworkSettings.Networks}}{{println $k}}{{end}}' "${CONTROLLER_CID}" \
  | grep 'gateway-network' | head -1)
[[ -n "${GATEWAY_NETWORK}" ]] || error "The gateway-controller is not attached to a gateway network."

if connect_error=$(docker network connect "${GATEWAY_NETWORK}" "${MOCK_CONTAINER}" 2>&1); then
  success "Mock backend attached to ${GATEWAY_NETWORK}."
elif [[ "${connect_error}" == *"already exists in network"* ]]; then
  info "Mock backend is already attached to ${GATEWAY_NETWORK}."
else
  error "Could not attach ${MOCK_CONTAINER} to ${GATEWAY_NETWORK}.
        Docker said: ${connect_error}"
fi

# --- Resources -------------------------------------------------------------
for provider_yaml in "${PROVIDER_YAMLS[@]}"; do
  deploy_resource "llm-providers" "${provider_yaml}"
done

for proxy_yaml in "${PROXY_YAMLS[@]}"; do
  deploy_resource "llm-proxies" "${proxy_yaml}"
done

register_api_key "${SCRIPT_DIR}/llm-proxy-assistant.yaml" "${ASSISTANT_API_KEY}"
register_api_key "${SCRIPT_DIR}/llm-proxy-support.yaml"   "${SUPPORT_API_KEY}"

cat <<EOF

============================================================
 Setup complete.

 Proxy endpoints:
   Assistant : http://localhost:8080/assistant/chat/completions   (gpt-4o-mini)
               api_key: ${ASSISTANT_API_KEY}
   Support   : http://localhost:8080/support/chat/completions     (gpt-4.1)
               api_key: ${SUPPORT_API_KEY}

 Next:
   ./load.sh     generate about a minute of traffic
   ./test.sh     verify the analytics pipeline
   then open https://www.moesif.com
============================================================
EOF
