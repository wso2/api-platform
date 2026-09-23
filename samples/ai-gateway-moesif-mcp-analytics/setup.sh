#!/usr/bin/env bash
set -euo pipefail

DIST_VERSION="1.2.0"
DIST_NAME="wso2apip-ai-gateway-${DIST_VERSION}"
DIST_ZIP="${DIST_NAME}.zip"
DIST_URL="https://github.com/wso2/api-platform/releases/download/ai-gateway/v${DIST_VERSION}/${DIST_ZIP}"

GATEWAY_MGMT_URL="http://localhost:9090/api/management/v1"
GATEWAY_HEALTH_URL="http://localhost:9094/health"
AUTH_HEADER="Authorization: Basic $(printf %s "${ADMIN_USERNAME:-admin}:${ADMIN_PASSWORD:-admin}" | base64 | tr -d '\r\n')"

MCP_CONTAINER="mcp-everything"
MCP_IMAGE="mcp-everything:sample"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=token.sh
source "${SCRIPT_DIR}/token.sh"
PROXY_YAMLS=("${SCRIPT_DIR}/mcp-proxy-toolbox.yaml" "${SCRIPT_DIR}/mcp-proxy-metered.yaml")
COLLECTOR_CONFIG="${SCRIPT_DIR}/collector-config.toml"
AUTH_CONFIG="${SCRIPT_DIR}/auth-config.toml"
KEY_DIR="${SCRIPT_DIR}/keys"
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

# A newly registered proxy takes a moment to start serving, so wait for it
# rather than letting the next script race it.
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
      2>/dev/null | grep -i '^mcp-session-id:' | tr -d '\r' | awk '{print $2}')
    [[ -n "${sid}" ]] && { success "${name} is answering."; return 0; }
    sleep 2
  done
  error "${name} did not start answering at ${url}.
        Check the logs with:
          cd ${DIST_NAME} && docker compose logs gateway-runtime"
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

# --- Body capture ----------------------------------------------------------
# The MCP method and tool name are read from the JSON-RPC bodies, so the
# collector has to be told to capture them.
require_file "${COLLECTOR_CONFIG}"
if grep -q 'request_body[[:space:]]*=[[:space:]]*true' "${GATEWAY_CONFIG}" \
  && grep -q 'response_body[[:space:]]*=[[:space:]]*true' "${GATEWAY_CONFIG}"; then
  info "Body capture already enabled, skipping."
elif grep -q '^\[collector\]' "${GATEWAY_CONFIG}"; then
  error "${GATEWAY_CONFIG} already has a [collector] section, but request_body and
        response_body are not both true. The MCP dashboard stays empty without them.
        Set both to true, or remove the section and run ./setup.sh again."
else
  info "Enabling request and response body capture ..."
  { echo ""; cat "${COLLECTOR_CONFIG}"; } >> "${GATEWAY_CONFIG}"
  success "Body capture enabled."
fi

# --- Token signing key -----------------------------------------------------
# The proxies require an access token. A real deployment trusts an identity
# provider's signatures; the sample signs its own so it needs no extra service.
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

# --- Trusted signatures ----------------------------------------------------
require_file "${AUTH_CONFIG}"
KEY_MANAGER_NAME="sample-key-manager"
if grep -q "${KEY_MANAGER_NAME}" "${GATEWAY_CONFIG}"; then
  # The configured key has to be the one token.sh signs with, or every token is
  # rejected with nothing to explain why.
  if grep -qF "$(sed -n '2p' "${KEY_DIR}/public.pem")" "${GATEWAY_CONFIG}"; then
    info "Key manager already configured, skipping."
  else
    error "${GATEWAY_CONFIG} has a ${KEY_MANAGER_NAME} entry, but its key is not the
        one in keys/public.pem, so every token would be rejected.
        Run ./teardown.sh --clean and then ./setup.sh."
  fi
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

# --- Compose override ------------------------------------------------------
require_file "${COMPOSE_OVERRIDE}"
cp "${COMPOSE_OVERRIDE}" "${DIST_NAME}/docker-compose.override.yaml"
success "Compose override installed."

# --- MCP server ------------------------------------------------------------
info "Building the MCP server image ..."
docker build -q -t "${MCP_IMAGE}" "${SCRIPT_DIR}/mcp-server" >/dev/null
success "Image ${MCP_IMAGE} built."

info "Starting the MCP server ..."
docker rm -f "${MCP_CONTAINER}" >/dev/null 2>&1 || true
docker run -d --name "${MCP_CONTAINER}" \
  "${MCP_IMAGE}" >/dev/null

for i in $(seq 1 60); do
  docker logs "${MCP_CONTAINER}" 2>&1 | grep -q 'listening on port' && break
  sleep 1
done
docker logs "${MCP_CONTAINER}" 2>&1 | grep -q 'listening on port' \
  || error "The MCP server did not start. Check: docker logs ${MCP_CONTAINER}"
success "MCP server running, reachable only from the gateway network."

# --- Gateway ---------------------------------------------------------------
[[ -x "${DIST_NAME}/scripts/setup.sh" ]] \
  || error "${DIST_NAME}/scripts/setup.sh is missing."

info "Starting the gateway ..."
(cd "${DIST_NAME}" \
  && ADMIN_USERNAME="${ADMIN_USERNAME:-admin}" ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin}" ./scripts/setup.sh)
(cd "${DIST_NAME}" && docker compose up -d)

wait_for_health "${GATEWAY_HEALTH_URL}"

# --- Attach the MCP server to the gateway network -------------------------
CONTROLLER_CID=$(cd "${DIST_NAME}" && docker compose ps -q gateway-controller 2>/dev/null | head -1)
[[ -n "${CONTROLLER_CID}" ]] || error "The gateway-controller container is not running."

GATEWAY_NETWORK=$(docker inspect -f '{{range $k, $v := .NetworkSettings.Networks}}{{println $k}}{{end}}' "${CONTROLLER_CID}" \
  | grep 'gateway-network' | head -1)
[[ -n "${GATEWAY_NETWORK}" ]] || error "The gateway-controller is not attached to a gateway network."

if connect_error=$(docker network connect "${GATEWAY_NETWORK}" "${MCP_CONTAINER}" 2>&1); then
  success "MCP server attached to ${GATEWAY_NETWORK}."
elif [[ "${connect_error}" == *"already exists in network"* ]]; then
  info "MCP server is already attached to ${GATEWAY_NETWORK}."
else
  error "Could not attach ${MCP_CONTAINER} to ${GATEWAY_NETWORK}.
        Docker said: ${connect_error}"
fi

# --- Resources -------------------------------------------------------------
for proxy_yaml in "${PROXY_YAMLS[@]}"; do
  deploy_resource "mcp-proxies" "${proxy_yaml}"
done

info "Waiting for both proxies to start serving ..."
wait_for_proxy "http://localhost:8080/toolbox/mcp" "Toolbox proxy"
wait_for_proxy "http://localhost:8080/metered/mcp" "Metered proxy"

cat <<EOF

============================================================
 Setup complete.

 MCP endpoints:
   Toolbox : http://localhost:8080/toolbox/mcp
             echo, get-sum, get-structured-content, get-resource-reference
   Metered : http://localhost:8080/metered/mcp
             echo, get-sum, limited to 15 calls per tool per minute

 Both require an access token. Mint one with:
   ./token.sh alice@example.com

 Next:
   ./load.sh     generate about a minute of MCP traffic
   ./test.sh     verify the analytics pipeline
   then open https://www.moesif.com and pick Analytics > MCP
============================================================
EOF
