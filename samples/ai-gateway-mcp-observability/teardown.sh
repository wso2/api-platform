#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration - must match setup.sh
# ---------------------------------------------------------------------------
DIST_VERSION="1.2.0"
DIST_NAME="wso2apip-ai-gateway-${DIST_VERSION}"
DIST_ZIP="${DIST_NAME}.zip"

GATEWAY_MGMT_URL="http://localhost:9090/api/management/v1"
AUTH_HEADER="Authorization: Basic $(printf %s "${ADMIN_USERNAME:-admin}:${ADMIN_PASSWORD:-admin}" | base64 | tr -d '\r\n')"   # default admin/admin; override with ADMIN_USERNAME/ADMIN_PASSWORD

MCP_CONTAINER="mcp-everything"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROXY_YAMLS=("${SCRIPT_DIR}/mcp-proxy-toolbox.yaml" "${SCRIPT_DIR}/mcp-proxy-metered.yaml")
KEY_DIR="${SCRIPT_DIR}/keys"

COMPOSE_PROFILES=(--profile metrics --profile tracing)

# Pass --clean to also remove the extracted directory, the zip archive and the
# generated signing key.
CLEAN=false
for arg in "$@"; do
  [[ "${arg}" == "--clean" ]] && CLEAN=true
done

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
# Counts cleanup steps that genuinely failed. Teardown always runs to the end;
# this decides the exit status.
FAILURES=0

info()    { echo "[INFO]  $*"; }
success() { echo "[OK]    $*"; }
warn()    { echo "[WARN]  $*"; }
error()   { echo "[ERROR] $*" >&2; exit 1; }

# Reads metadata.name out of one of the sample's resource files.
yaml_name() {
  awk '/^metadata:/ { in_meta = 1; next }
       in_meta && /^[[:space:]]+name:/ {
         sub(/^[[:space:]]*name:[[:space:]]*/, ""); print; exit
       }
       /^[^[:space:]#]/ && !/^metadata:/ { in_meta = 0 }' "$1"
}

# Deletes a resource, treating an absent one as already gone so teardown can continue.
delete_resource() {
  local kind="$1" name="$2" http_status
  info "Deleting ${kind}/${name} ..."
  # Keep going when the gateway is unreachable, so the container and volume cleanup
  # below still runs. curl reports 000 in that case.
  http_status=$(curl -s -o /dev/null -w "%{http_code}" \
    --connect-timeout 5 --max-time 30 \
    -X DELETE "${GATEWAY_MGMT_URL}/${kind}/${name}" \
    -H "${AUTH_HEADER}" || true)
  if [[ "${http_status}" =~ ^2 ]]; then
    success "Deleted ${kind}/${name} (HTTP ${http_status})."
  elif [[ "${http_status}" == "404" ]]; then
    info "${kind}/${name} was not registered."
  elif [[ "${http_status}" == "000" ]]; then
    warn "Gateway is not reachable — skipping ${kind}/${name}."
  else
    warn "Could not delete ${kind}/${name} (HTTP ${http_status})."
    FAILURES=$(( FAILURES + 1 ))
  fi
}

# ---------------------------------------------------------------------------
# Step 1 - Delete the MCP proxies
# ---------------------------------------------------------------------------
for PROXY_YAML in "${PROXY_YAMLS[@]}"; do
  [[ -f "${PROXY_YAML}" ]] || error "Proxy file not found at ${PROXY_YAML}"
  delete_resource "mcp-proxies" "$(yaml_name "${PROXY_YAML}")"
done

# ---------------------------------------------------------------------------
# Step 2 - Stop the MCP server
# ---------------------------------------------------------------------------
if docker ps -aq -f "name=^${MCP_CONTAINER}$" 2>/dev/null | grep -q .; then
  info "Removing ${MCP_CONTAINER} ..."
  if docker rm -f "${MCP_CONTAINER}" >/dev/null 2>&1; then
    success "${MCP_CONTAINER} removed."
  else
    warn "Could not remove ${MCP_CONTAINER}; remove it manually."
    FAILURES=$(( FAILURES + 1 ))
  fi
else
  info "${MCP_CONTAINER} is not present."
fi

# ---------------------------------------------------------------------------
# Step 3 - Stop the stack
#
# --volumes drops the Prometheus and Grafana data, so the next run starts clean.
# ---------------------------------------------------------------------------
COMPOSE_FILE="${DIST_NAME}/docker-compose.yaml"
[[ -f "${COMPOSE_FILE}" ]] || COMPOSE_FILE="${DIST_NAME}/docker-compose.yml"

if [[ -f "${COMPOSE_FILE}" ]]; then
  info "Stopping Docker Compose stack ..."
  if (cd "${DIST_NAME}" && docker compose "${COMPOSE_PROFILES[@]}" down --volumes); then
    success "Stack stopped and volumes removed."
  else
    warn "Failed to stop the Compose stack; containers or volumes may remain."
    FAILURES=$(( FAILURES + 1 ))
  fi
else
  warn "docker-compose file not found in ${DIST_NAME}/ — stack may not be running."
fi

# ---------------------------------------------------------------------------
# Step 4 - Optional cleanup of generated files
# ---------------------------------------------------------------------------
if [[ "${CLEAN}" == true ]]; then
  if [[ -d "${DIST_NAME}" ]]; then
    info "Removing extracted directory ${DIST_NAME}/ ..."
    rm -rf "${DIST_NAME}"
    success "Removed ${DIST_NAME}/."
  fi
  if [[ -f "${DIST_ZIP}" ]]; then
    info "Removing archive ${DIST_ZIP} ..."
    rm -f "${DIST_ZIP}"
    success "Removed ${DIST_ZIP}."
  fi
  if [[ -d "${KEY_DIR}" ]]; then
    info "Removing the generated signing key ..."
    rm -rf "${KEY_DIR}"
    success "Removed keys/."
  fi
fi

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
echo ""
echo "============================================================"
if [[ "${FAILURES}" -eq 0 ]]; then
  echo " Teardown complete!"
  if [[ "${CLEAN}" == false ]]; then
    echo " Tip: run with --clean to also remove ${DIST_NAME}/, ${DIST_ZIP} and keys/"
    echo "      (the config edits from setup.sh live in there — --clean guarantees"
    echo "       the next run starts from the pristine distribution)"
  fi
  echo "============================================================"
else
  echo " Teardown incomplete: ${FAILURES} step(s) failed."
  echo " Some resources may still exist. Check with: docker ps -a"
  echo "============================================================"
  exit 1
fi
