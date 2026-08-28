#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration — must match setup.sh
# ---------------------------------------------------------------------------
DIST_VERSION="1.2.0"
DIST_NAME="wso2apip-ai-gateway-${DIST_VERSION}"
DIST_ZIP="${DIST_NAME}.zip"

GATEWAY_MGMT_URL="http://localhost:9090/api/management/v1"
AUTH_HEADER="Authorization: Basic $(printf %s "${ADMIN_USERNAME:-admin}:${ADMIN_PASSWORD:-admin}" | base64 | tr -d '\r\n')"   # default admin/admin; override with ADMIN_USERNAME/ADMIN_PASSWORD

MOCK_CONTAINER="mock-llm-openai"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROVIDER_YAMLS=("${SCRIPT_DIR}/llm-provider.yaml" "${SCRIPT_DIR}/llm-provider-budgeted.yaml")
PROXY_YAMLS=("${SCRIPT_DIR}/llm-proxy-assistant.yaml" "${SCRIPT_DIR}/llm-proxy-support.yaml")

COMPOSE_PROFILES=(--profile metrics --profile tracing)

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

cd "${SCRIPT_DIR}"

# ---------------------------------------------------------------------------
# Step 1 — Delete LLM proxies
# ---------------------------------------------------------------------------
for PROXY_YAML in "${PROXY_YAMLS[@]}"; do
  [[ -f "${PROXY_YAML}" ]] || error "Proxy file not found at ${PROXY_YAML}"
  delete_resource "llm-proxies" "$(yaml_name "${PROXY_YAML}")"
done

# ---------------------------------------------------------------------------
# Step 2 — Delete LLM providers
# ---------------------------------------------------------------------------
for PROVIDER_YAML in "${PROVIDER_YAMLS[@]}"; do
  [[ -f "${PROVIDER_YAML}" ]] || error "Provider file not found at ${PROVIDER_YAML}"
  delete_resource "llm-providers" "$(yaml_name "${PROVIDER_YAML}")"
done

# ---------------------------------------------------------------------------
# Step 3 — Stop the mock LLM backend
# ---------------------------------------------------------------------------
info "Removing ${MOCK_CONTAINER} ..."
docker rm -f "${MOCK_CONTAINER}" >/dev/null 2>&1 || true
success "${MOCK_CONTAINER} removed."

# ---------------------------------------------------------------------------
# Step 4 — Stop the stack
#
# --volumes matters here: Prometheus and Grafana keep their data in volumes, and
# metrics left over from a previous run make the next dashboard confusing.
# ---------------------------------------------------------------------------
COMPOSE_FILE="${DIST_NAME}/docker-compose.yaml"
[[ -f "${COMPOSE_FILE}" ]] || COMPOSE_FILE="${DIST_NAME}/docker-compose.yml"

if [[ -f "${COMPOSE_FILE}" ]]; then
  info "Stopping Docker Compose stack ..."
  (cd "${DIST_NAME}" && docker compose "${COMPOSE_PROFILES[@]}" down --volumes)
  success "Stack stopped and volumes removed."
else
  warn "docker-compose file not found in ${DIST_NAME}/ — stack may not be running."
fi

# ---------------------------------------------------------------------------
# Step 5 — Optional cleanup of distribution files
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
  echo "      (the config edits from setup.sh live in there — --clean guarantees"
  echo "       the next run starts from the pristine distribution)"
fi
echo "============================================================"
