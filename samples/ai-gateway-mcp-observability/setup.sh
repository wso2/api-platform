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

MCP_CONTAINER="mcp-everything"
MCP_IMAGE="mcp-everything:sample"
MCP_PORT="${MCP_PORT:-3001}"

# Observability UIs exposed by the distribution's docker-compose.
GRAFANA_URL="http://localhost:3000"
JAEGER_URL="http://localhost:16686"
PROMETHEUS_URL="http://localhost:9092"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=token.sh
source "${SCRIPT_DIR}/token.sh"

PROXY_YAMLS=("${SCRIPT_DIR}/mcp-proxy-toolbox.yaml" "${SCRIPT_DIR}/mcp-proxy-metered.yaml")
ADDITIONAL_CONFIG="${SCRIPT_DIR}/additional-config.toml"
AUTH_CONFIG="${SCRIPT_DIR}/auth-config.toml"
KEY_DIR="${SCRIPT_DIR}/keys"
DASHBOARD_JSON="${SCRIPT_DIR}/observability/ai-gateway-mcp-overview.json"
COMPOSE_OVERRIDE="${SCRIPT_DIR}/observability/docker-compose.override.yaml"

# Prometheus, Grafana, Jaeger and the OTel collector sit behind these Compose profiles.
COMPOSE_PROFILES=(--profile metrics --profile tracing)

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info()    { echo "[INFO]  $*"; }
success() { echo "[OK]    $*"; }
error()   { echo "[ERROR] $*" >&2; exit 1; }

require_file() { [[ -f "$1" ]] || error "Required file not found: $1"; }

# Polls a health endpoint until it answers, giving up after a fixed number of tries.
wait_for_health() {
  local url="$1" max_attempts=30 interval=5
  info "Waiting for the gateway to be healthy at ${url} ..."
  for i in $(seq 1 "${max_attempts}"); do
    if curl -sf --connect-timeout 5 --max-time 10 "${url}" >/dev/null 2>&1; then
      success "Gateway is healthy."
      return 0
    fi
    echo "  attempt ${i}/${max_attempts} — retrying in ${interval}s ..."
    sleep "${interval}"
  done
  error "Gateway did not become healthy after $((max_attempts * interval))s."
}

# Reads metadata.name out of one of the sample's resource files.
yaml_name() {
  awk '/^metadata:/ { in_meta = 1; next }
       in_meta && /^[[:space:]]+name:/ {
         sub(/^[[:space:]]*name:[[:space:]]*/, ""); print; exit
       }
       /^[^[:space:]#]/ && !/^metadata:/ { in_meta = 0 }' "$1"
}

# POSTs a resource YAML to the management API and reports the outcome by HTTP status.
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
      error "$(basename "${file}") is already deployed on this gateway (HTTP ${status}).
        Gateway said: ${detail}
        Run ./teardown.sh first, then ./setup.sh — teardown drops the gateway's
        database volume, which is what clears previously registered resources."
      ;;
    *)
      error "Failed to deploy $(basename "${file}") (HTTP ${status}).
        Gateway said: ${detail}"
      ;;
  esac
}

# A newly registered proxy takes a moment before it starts serving. It is ready
# once it answers the handshake with a session id.
wait_for_proxy() {
  local url="$1" name="$2" token sid max_attempts=20
  token=$(mint_token "setup-check@example.com") || return 1
  for i in $(seq 1 "${max_attempts}"); do
    sid=$(curl -s -D - -o /dev/null --connect-timeout 5 --max-time 15 \
      -X POST "${url}" \
      -H "Content-Type: application/json" \
      -H "Accept: application/json, text/event-stream" \
      -H "Authorization: Bearer ${token}" \
      -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"setup-check","version":"1.0.0"}}}' \
      2>/dev/null | grep -i '^mcp-session-id:' | tr -d '\r' | awk '{print $2}') || sid=""
    [[ -n "${sid}" ]] && { success "${name} is answering."; return 0; }
    sleep 2
  done
  error "${name} did not start answering at ${url}.
        Check the logs with:
          cd ${DIST_NAME} && docker compose logs gateway-runtime"
}

cd "${SCRIPT_DIR}"

# ---------------------------------------------------------------------------
# Step 1 - Download and extract the distribution
# ---------------------------------------------------------------------------
if [[ -f "${DIST_ZIP}" ]]; then
  info "${DIST_ZIP} already present, skipping download."
elif command -v curl >/dev/null 2>&1; then
  info "Downloading ${DIST_ZIP} ..."
  curl -fSL --connect-timeout 10 --max-time 300 "${DIST_URL}" -o "${DIST_ZIP}"
elif command -v wget >/dev/null 2>&1; then
  info "Downloading ${DIST_ZIP} ..."
  wget -q "${DIST_URL}" -O "${DIST_ZIP}"
else
  error "Neither curl nor wget is available — install one and retry."
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

# ---------------------------------------------------------------------------
# Step 2 - Turn on the metrics endpoints and tracing
# ---------------------------------------------------------------------------
require_file "${ADDITIONAL_CONFIG}"
if grep -q '^\[tracing\]' "${GATEWAY_CONFIG}"; then
  info "Metrics and tracing already enabled in ${GATEWAY_CONFIG}, skipping."
else
  info "Merging additional-config.toml into ${GATEWAY_CONFIG} ..."
  TMP_MERGED=$(mktemp)
  { cat "${ADDITIONAL_CONFIG}"; echo ""; cat "${GATEWAY_CONFIG}"; } > "${TMP_MERGED}"
  mv "${TMP_MERGED}" "${GATEWAY_CONFIG}"
  success "Controller metrics, policy engine metrics and tracing enabled."
fi

# ---------------------------------------------------------------------------
# Step 3 - Token signing key
#
# Both proxies require an access token. A real deployment trusts an identity
# provider's signatures; the sample signs its own so it needs no extra service.
# ---------------------------------------------------------------------------
if [[ -f "${KEY_DIR}/private.pem" && -f "${KEY_DIR}/public.pem" ]]; then
  info "Signing key already present, skipping."
else
  info "Generating a token signing key ..."
  mkdir -p "${KEY_DIR}"
  openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 \
    -out "${KEY_DIR}/private.pem" 2>/dev/null \
    || error "Could not generate a signing key. Is openssl installed?"
  openssl rsa -in "${KEY_DIR}/private.pem" -pubout -out "${KEY_DIR}/public.pem" 2>/dev/null
  chmod 600 "${KEY_DIR}/private.pem"
  success "Signing key written to keys/ (git-ignored)."
fi

require_file "${AUTH_CONFIG}"
if grep -q 'policy_configurations\.jwtauth_v1\.keymanagers' "${GATEWAY_CONFIG}"; then
  info "Key manager already configured, skipping."
else
  info "Telling the gateway which signatures to trust ..."
  {
    echo ""
    awk -v keyfile="${KEY_DIR}/public.pem" \
      '/^__PUBLIC_KEY__$/ { while ((getline line < keyfile) > 0) print line; next } { print }' \
      "${AUTH_CONFIG}"
  } >> "${GATEWAY_CONFIG}"
  success "Key manager configured."
fi

# ---------------------------------------------------------------------------
# Step 4 - Provision the Grafana dashboard
#
# Grafana provisions whatever it finds in its dashboards folder, so copying the
# file in is enough. The compose override makes it the dashboard Grafana opens on.
# ---------------------------------------------------------------------------
require_file "${DASHBOARD_JSON}"
info "Copying the dashboard into ${DIST_NAME}/observability/grafana/dashboards/ ..."
cp "${DASHBOARD_JSON}" "${DIST_NAME}/observability/grafana/dashboards/"
success "Dashboard provisioned."

require_file "${COMPOSE_OVERRIDE}"
info "Pointing Grafana's home dashboard at the MCP overview ..."
cp "${COMPOSE_OVERRIDE}" "${DIST_NAME}/docker-compose.override.yaml"
success "Grafana home dashboard set."

# ---------------------------------------------------------------------------
# Step 5 - Start the MCP server
# ---------------------------------------------------------------------------
info "Building the MCP server image ..."
docker build -q -t "${MCP_IMAGE}" "${SCRIPT_DIR}/mcp-server" >/dev/null
success "Image ${MCP_IMAGE} built."

info "Starting the MCP server ..."
docker rm -f "${MCP_CONTAINER}" >/dev/null 2>&1 || true
docker run -d --name "${MCP_CONTAINER}" \
  -p "${MCP_PORT}:3001" \
  "${MCP_IMAGE}" >/dev/null

for i in $(seq 1 60); do
  docker logs "${MCP_CONTAINER}" 2>&1 | grep -q 'listening on port' && break
  sleep 1
done
docker logs "${MCP_CONTAINER}" 2>&1 | grep -q 'listening on port' \
  || error "The MCP server did not start. Check: docker logs ${MCP_CONTAINER}"
success "MCP server running on host port ${MCP_PORT}."

# ---------------------------------------------------------------------------
# Step 6 - Start the stack (gateway + observability)
# ---------------------------------------------------------------------------
[[ -x "${DIST_NAME}/scripts/setup.sh" ]] \
  || error "${DIST_NAME}/scripts/setup.sh is missing — is this the ${DIST_VERSION} distribution?"

info "Starting the Docker Compose stack in ${DIST_NAME}/ ..."
# The distribution's own setup script provisions its listener cert, encryption key,
# api-platform.env and admin credentials. Credentials are passed in to keep it
# non-interactive.
(cd "${DIST_NAME}" \
  && ADMIN_USERNAME="${ADMIN_USERNAME:-admin}" ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin}" ./scripts/setup.sh)
(cd "${DIST_NAME}" && docker compose "${COMPOSE_PROFILES[@]}" up -d)
success "Gateway, Prometheus, Grafana, Jaeger and the OTel collector are starting."

wait_for_health "${GATEWAY_HEALTH_URL}"

# ---------------------------------------------------------------------------
# Step 7 - Put the MCP server on the gateway's network
# ---------------------------------------------------------------------------
# The network name carries the Compose project name, so read it from the running
# controller.
CONTROLLER_CID=$(cd "${DIST_NAME}" && docker compose "${COMPOSE_PROFILES[@]}" ps -q gateway-controller 2>/dev/null | head -1)
[[ -n "${CONTROLLER_CID}" ]] || error "Could not find the running gateway-controller container."

GATEWAY_NETWORK=$(docker inspect -f '{{range $k, $v := .NetworkSettings.Networks}}{{println $k}}{{end}}' "${CONTROLLER_CID}" \
  | grep 'gateway-network' | head -1) || GATEWAY_NETWORK=""
[[ -n "${GATEWAY_NETWORK}" ]] || error "The gateway-controller is not attached to a gateway network."

if connect_error=$(docker network connect "${GATEWAY_NETWORK}" "${MCP_CONTAINER}" 2>&1); then
  success "MCP server attached to ${GATEWAY_NETWORK}."
elif [[ "${connect_error}" == *"already exists in network"* ]]; then
  info "MCP server is already attached to ${GATEWAY_NETWORK}."
else
  error "Could not attach ${MCP_CONTAINER} to ${GATEWAY_NETWORK}.
        Docker said: ${connect_error}"
fi

# ---------------------------------------------------------------------------
# Step 8 - Deploy the proxies
# ---------------------------------------------------------------------------
for proxy_yaml in "${PROXY_YAMLS[@]}"; do
  deploy_resource "mcp-proxies" "${proxy_yaml}"
done

info "Waiting for both proxies to start serving ..."
wait_for_proxy "http://localhost:8080/toolbox/mcp" "Toolbox proxy"
wait_for_proxy "http://localhost:8080/metered/mcp" "Metered proxy"

cat <<SUMMARY

============================================================
 Setup complete.

 MCP endpoints:
   Toolbox : http://localhost:8080/toolbox/mcp
             echo, get-sum, get-structured-content, get-resource-reference
   Metered : http://localhost:8080/metered/mcp
             echo, get-sum, limited to 15 calls per tool per minute

 Both require an access token. Mint one with:
   ./token.sh alice@example.com

 Observability:
   Grafana    : ${GRAFANA_URL}      (admin / admin)
   Jaeger     : ${JAEGER_URL}
   Prometheus : ${PROMETHEUS_URL}

 Next:
   ./load.sh     generate about a minute of MCP traffic, then watch the dashboard
   ./test.sh     assert metrics and traces are actually flowing
============================================================
SUMMARY
