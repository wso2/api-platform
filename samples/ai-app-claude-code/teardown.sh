#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
DIST_VERSION="1.1.0"
DIST_NAME="wso2apip-api-gateway-${DIST_VERSION}"
DIST_ZIP="${DIST_NAME}.zip"

GATEWAY_MGMT_URL="http://localhost:9090/api/management/v0.9"
AUTH_HEADER="Authorization: Basic $(printf %s "${ADMIN_USERNAME:-admin}:${ADMIN_PASSWORD:-admin}" | base64)"   # default admin/admin; override with ADMIN_USERNAME/ADMIN_PASSWORD

API_NAME="reading-list-api"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info()    { echo "[INFO]  $*"; }
success() { echo "[OK]    $*"; }

# ---------------------------------------------------------------------------
# Step 1 — Delete the API from the gateway (best-effort)
# ---------------------------------------------------------------------------
info "Deleting Reading List API from gateway ..."
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X DELETE "${GATEWAY_MGMT_URL}/rest-apis/${API_NAME}" \
  -H "${AUTH_HEADER}" 2>/dev/null || echo "000")

if [[ "${HTTP_STATUS}" =~ ^2 ]]; then
  success "API deleted (HTTP ${HTTP_STATUS})."
else
  info "Could not delete API (HTTP ${HTTP_STATUS}) — gateway may already be down."
fi

# ---------------------------------------------------------------------------
# Step 2 — Stop Docker Compose stack
# ---------------------------------------------------------------------------
COMPOSE_FILE="${SCRIPT_DIR}/${DIST_NAME}/docker-compose.yaml"
[[ -f "${COMPOSE_FILE}" ]] || COMPOSE_FILE="${SCRIPT_DIR}/${DIST_NAME}/docker-compose.yml"

if [[ -f "${COMPOSE_FILE}" ]]; then
  info "Stopping Docker Compose stack ..."
  (cd "${SCRIPT_DIR}/${DIST_NAME}" && docker compose down --volumes)
  success "Stack stopped."
else
  info "docker-compose file not found — stack may already be stopped."
fi

# ---------------------------------------------------------------------------
# Step 3 — Optional deep clean (--clean flag)
# ---------------------------------------------------------------------------
if [[ "${1:-}" == "--clean" ]]; then
  info "Removing extracted distribution and downloaded zip ..."
  rm -rf "${SCRIPT_DIR}/${DIST_NAME}"
  rm -f  "${SCRIPT_DIR}/${DIST_ZIP}"
  rm -f  "${SCRIPT_DIR}/.claude/settings.json"
  success "Clean complete."
fi

echo ""
echo "============================================================"
echo " Teardown complete."
echo " Run './setup.sh' to start fresh."
echo "============================================================"
