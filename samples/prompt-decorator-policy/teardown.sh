#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration — must match setup.sh
# ---------------------------------------------------------------------------
DIST_VERSION="1.1.0"
DIST_NAME="wso2apip-ai-gateway-${DIST_VERSION}"
DIST_ZIP="${DIST_NAME}.zip"

GATEWAY_MGMT_URL="http://localhost:9090/api/management/v0.9"
AUTH_HEADER="Authorization: Basic $(printf %s "${ADMIN_USERNAME:-admin}:${ADMIN_PASSWORD:-admin}" | base64)"   # default admin/admin; override with ADMIN_USERNAME/ADMIN_PASSWORD

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROVIDER_YAML="${SCRIPT_DIR}/llm-provider.yaml"
PROXY_YAMLS=("${SCRIPT_DIR}/llm-proxy-persona.yaml" "${SCRIPT_DIR}/llm-proxy-suffix.yaml")

# Pass --clean to also remove the extracted directory and zip archive.
CLEAN=false
for arg in "$@"; do
  [[ "${arg}" == "--clean" ]] && CLEAN=true
done

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info()    { echo "[INFO]  $*"; }
success() { echo "[OK]    $*"; }
warn()    { echo "[WARN]  $*"; }
error()   { echo "[ERROR] $*" >&2; exit 1; }

yaml_name() {
  python3 -c "import yaml,sys; d=yaml.safe_load(open(sys.argv[1])); print(d['metadata']['name'])" "$1"
}

delete_resource() {
  local kind="$1"   # llm-providers or llm-proxies
  local name="$2"
  info "Deleting ${kind}/${name} ..."
  HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X DELETE "${GATEWAY_MGMT_URL}/${kind}/${name}" \
    -H "${AUTH_HEADER}")
  if [[ "${HTTP_STATUS}" =~ ^2 ]]; then
    success "Deleted ${kind}/${name} (HTTP ${HTTP_STATUS})."
  elif [[ "${HTTP_STATUS}" == "404" ]]; then
    warn "${kind}/${name} not found — already deleted?"
  else
    warn "Failed to delete ${kind}/${name} (HTTP ${HTTP_STATUS}); continuing teardown."
  fi
}

# ---------------------------------------------------------------------------
# Step 1 — Delete LLM proxies
# ---------------------------------------------------------------------------
for PROXY_YAML in "${PROXY_YAMLS[@]}"; do
  [[ -f "${PROXY_YAML}" ]] || error "Proxy file not found at ${PROXY_YAML}"
  PROXY_NAME=$(yaml_name "${PROXY_YAML}")
  delete_resource "llm-proxies" "${PROXY_NAME}"
done

# ---------------------------------------------------------------------------
# Step 2 — Delete LLM provider
# ---------------------------------------------------------------------------
[[ -f "${PROVIDER_YAML}" ]] || error "llm-provider.yaml not found at ${PROVIDER_YAML}"
PROVIDER_NAME=$(yaml_name "${PROVIDER_YAML}")
delete_resource "llm-providers" "${PROVIDER_NAME}"

# ---------------------------------------------------------------------------
# Step 3 — Stop Docker Compose stack
# ---------------------------------------------------------------------------
COMPOSE_FILE="${DIST_NAME}/docker-compose.yaml"
[[ -f "${COMPOSE_FILE}" ]] || COMPOSE_FILE="${DIST_NAME}/docker-compose.yml"

if [[ -f "${COMPOSE_FILE}" ]]; then
  info "Stopping Docker Compose stack ..."
  (cd "${DIST_NAME}" && docker compose down --volumes)
  success "Stack stopped and volumes removed."
else
  warn "docker-compose file not found in ${DIST_NAME}/ — stack may not be running."
fi

# ---------------------------------------------------------------------------
# Step 4 — Optional cleanup of distribution files
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
fi

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
echo ""
echo "============================================================"
echo " Teardown complete!"
if [[ "${CLEAN}" == false ]]; then
  echo " Tip: run with --clean to also remove ${DIST_NAME}/ and ${DIST_ZIP}"
fi
echo "============================================================"
