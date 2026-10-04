#!/usr/bin/env bash
#
# Removes everything this sample created locally. Events already delivered to
# Moesif are unaffected.
#
#   ./teardown.sh            stop the containers and drop the volumes
#   ./teardown.sh --clean    also remove the extracted distribution and image
#
set -euo pipefail

DIST_VERSION="1.2.0"
DIST_NAME="wso2apip-ai-gateway-${DIST_VERSION}"
DIST_ZIP="${DIST_NAME}.zip"

GATEWAY_MGMT_URL="http://localhost:9090/api/management/v1"
AUTH_HEADER="Authorization: Basic $(printf %s "${ADMIN_USERNAME:-admin}:${ADMIN_PASSWORD:-admin}" | base64 | tr -d '\r\n')"

MCP_CONTAINER="mcp-everything"
MCP_IMAGE="mcp-everything:sample"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROXY_YAMLS=("${SCRIPT_DIR}/mcp-proxy-toolbox.yaml" "${SCRIPT_DIR}/mcp-proxy-metered.yaml")

CLEAN=false
for arg in "$@"; do
  [[ "${arg}" == "--clean" ]] && CLEAN=true
done

failures=0

info()    { echo "[INFO]  $*"; }
success() { echo "[OK]    $*"; }
warn()    { echo "[WARN]  $*"; }
error()   { echo "[ERROR] $*" >&2; exit 1; }

yaml_name() {
  awk '/^metadata:/ { in_meta = 1; next }
       in_meta && /^[[:space:]]+name:/ {
         sub(/^[[:space:]]*name:[[:space:]]*/, ""); print; exit
       }
       /^[^[:space:]#]/ && !/^metadata:/ { in_meta = 0 }' "$1"
}

delete_resource() {
  local kind="$1" name="$2" status
  info "Deleting ${kind}/${name} ..."
  # || true: an unreachable gateway must not abort teardown.
  status=$(curl -s -o /dev/null -w "%{http_code}" \
    --connect-timeout 5 --max-time 30 \
    -X DELETE "${GATEWAY_MGMT_URL}/${kind}/${name}" \
    -H "${AUTH_HEADER}" || true)

  case "${status}" in
    2*)  success "Deleted ${kind}/${name}." ;;
    404) warn "${kind}/${name} was not found." ;;
    000) warn "Gateway is not reachable; skipping ${kind}/${name}." ;;
    *)
      warn "Failed to delete ${kind}/${name} (HTTP ${status})."
      failures=$(( failures + 1 ))
      ;;
  esac
}

cd "${SCRIPT_DIR}"

for proxy_yaml in "${PROXY_YAMLS[@]}"; do
  [[ -f "${proxy_yaml}" ]] || error "Not found: ${proxy_yaml}"
  delete_resource "mcp-proxies" "$(yaml_name "${proxy_yaml}")"
done

if docker ps -aq -f "name=^${MCP_CONTAINER}$" 2>/dev/null | grep -q .; then
  info "Removing ${MCP_CONTAINER} ..."
  if docker rm -f "${MCP_CONTAINER}" >/dev/null 2>&1; then
    success "${MCP_CONTAINER} removed."
  else
    warn "Could not remove ${MCP_CONTAINER}."
    failures=$(( failures + 1 ))
  fi
else
  info "${MCP_CONTAINER} is not running."
fi

COMPOSE_FILE="${DIST_NAME}/docker-compose.yaml"
[[ -f "${COMPOSE_FILE}" ]] || COMPOSE_FILE="${DIST_NAME}/docker-compose.yml"

if [[ -f "${COMPOSE_FILE}" ]]; then
  info "Stopping the gateway ..."
  if (cd "${DIST_NAME}" \
      && MOESIF_APPLICATION_ID="${MOESIF_APPLICATION_ID:-unset}" docker compose down --volumes); then
    success "Gateway stopped and volumes removed."
  else
    warn "Failed to stop the gateway; containers or volumes may remain."
    failures=$(( failures + 1 ))
  fi
else
  warn "No compose file found in ${DIST_NAME}/."
fi

if [[ "${CLEAN}" == true ]]; then
  [[ -d "${DIST_NAME}" ]] && { rm -rf "${DIST_NAME}"; success "Removed ${DIST_NAME}/."; }
  [[ -f "${DIST_ZIP}" ]]  && { rm -f "${DIST_ZIP}";   success "Removed ${DIST_ZIP}."; }
  docker image rm -f "${MCP_IMAGE}" >/dev/null 2>&1 \
    && success "Removed the ${MCP_IMAGE} image."
  [[ -d "${SCRIPT_DIR}/keys" ]] && { rm -rf "${SCRIPT_DIR}/keys"; success "Removed the signing key."; }
fi

echo ""
echo "============================================================"
if (( failures == 0 )); then
  echo " Teardown complete."
  if [[ "${CLEAN}" == false ]]; then
    echo " Run with --clean to also remove ${DIST_NAME}/, ${DIST_ZIP} and the"
    echo " MCP server image, so the next ./setup.sh starts from scratch."
  fi
  echo "============================================================"
else
  echo " Teardown incomplete: ${failures} step(s) failed."
  echo " Check remaining containers with: docker ps -a"
  echo "============================================================"
  exit 1
fi
