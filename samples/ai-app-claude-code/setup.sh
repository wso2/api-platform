#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
DIST_VERSION="1.1.0"
DIST_NAME="wso2apip-api-gateway-${DIST_VERSION}"
DIST_ZIP="${DIST_NAME}.zip"
DIST_URL="https://github.com/wso2/api-platform/releases/download/gateway/v${DIST_VERSION}/${DIST_ZIP}"

GATEWAY_MGMT_URL="http://localhost:9090/api/management/v0.9"
GATEWAY_HEALTH_URL="http://localhost:9094/health"
AUTH_HEADER="Authorization: Basic $(printf %s "${ADMIN_USERNAME:-admin}:${ADMIN_PASSWORD:-admin}" | base64 | tr -d '\r\n')"   # default admin/admin; override with ADMIN_USERNAME/ADMIN_PASSWORD

API_NAME="reading-list-api"
API_KEY_NAME="claude-code-key"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
API_YAML="${SCRIPT_DIR}/reading-list-api.yaml"

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

# ---------------------------------------------------------------------------
# Step 1 — Download gateway distribution
# ---------------------------------------------------------------------------
info "Downloading ${DIST_ZIP} ..."
if [[ -f "${SCRIPT_DIR}/${DIST_ZIP}" ]]; then
  info "Archive already exists, skipping download."
else
  curl -fL --progress-bar "${DIST_URL}" -o "${SCRIPT_DIR}/${DIST_ZIP}"
  success "Downloaded ${DIST_ZIP}."
fi

# ---------------------------------------------------------------------------
# Step 2 — Unzip
# ---------------------------------------------------------------------------
if [[ -d "${SCRIPT_DIR}/${DIST_NAME}" ]]; then
  info "Distribution directory '${DIST_NAME}' already exists, skipping unzip."
else
  info "Unzipping ${DIST_ZIP} ..."
  unzip -q "${SCRIPT_DIR}/${DIST_ZIP}" -d "${SCRIPT_DIR}"
  success "Extracted to ${DIST_NAME}/."
fi

# ---------------------------------------------------------------------------
# Step 3 — Start the gateway stack
# ---------------------------------------------------------------------------
info "Starting Docker Compose stack in ${DIST_NAME}/ ..."
# Bring down any previous instance to avoid stale network/port conflicts.
(cd "${SCRIPT_DIR}/${DIST_NAME}" && docker compose down -v --remove-orphans 2>/dev/null || true)
# Provision the gateway's listener cert, encryption key, api-platform.env, and admin credentials.
# The gateway no longer ships a default admin:admin and fails closed without a credential; this
# provisions admin/admin (matching AUTH_HEADER above). Override via ADMIN_USERNAME/ADMIN_PASSWORD.
(cd "${SCRIPT_DIR}/${DIST_NAME}" && ADMIN_USERNAME="${ADMIN_USERNAME:-admin}" ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin}" ./scripts/setup.sh)
(cd "${SCRIPT_DIR}/${DIST_NAME}" && docker compose up -d)
success "Docker Compose stack started."

# ---------------------------------------------------------------------------
# Step 4 — Health check
# ---------------------------------------------------------------------------
wait_for_health "${GATEWAY_HEALTH_URL}"

# ---------------------------------------------------------------------------
# Step 5 — Deploy the Reading List API
# ---------------------------------------------------------------------------
info "Deploying Reading List API from ${API_YAML} ..."
[[ -f "${API_YAML}" ]] || error "reading-list-api.yaml not found at ${API_YAML}"

HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X POST "${GATEWAY_MGMT_URL}/rest-apis" \
  -H "Content-Type: application/yaml" \
  -H "${AUTH_HEADER}" \
  --data-binary "@${API_YAML}")

if [[ "${HTTP_STATUS}" == "201" ]]; then
  success "Reading List API deployed (HTTP ${HTTP_STATUS})."
elif [[ "${HTTP_STATUS}" == "409" ]]; then
  info "Reading List API already exists, skipping."
else
  error "Failed to deploy Reading List API (HTTP ${HTTP_STATUS})."
fi

# ---------------------------------------------------------------------------
# Step 6 — Generate API key
# ---------------------------------------------------------------------------
info "Generating API key '${API_KEY_NAME}' ..."

KEY_RESPONSE=$(curl -s \
  -X POST "${GATEWAY_MGMT_URL}/rest-apis/${API_NAME}/api-keys" \
  -H "Content-Type: application/json" \
  -H "${AUTH_HEADER}" \
  -d "{\"name\": \"${API_KEY_NAME}\"}")

HTTP_STATUS=$(echo "${KEY_RESPONSE}" | python3 -c \
  "import sys,json; d=json.load(sys.stdin); print('ok' if d.get('status')=='success' else 'fail')" \
  2>/dev/null || echo "fail")

if [[ "${HTTP_STATUS}" == "fail" ]]; then
  error "Failed to generate API key: ${KEY_RESPONSE}"
fi

API_KEY_VALUE=$(echo "${KEY_RESPONSE}" | python3 -c \
  "import sys,json; print(json.load(sys.stdin)['apiKey']['apiKey'])")
success "API key generated."

# ---------------------------------------------------------------------------
# Step 7 — Write .claude/settings.json
# ---------------------------------------------------------------------------
CLAUDE_DIR="${SCRIPT_DIR}/.claude"
SETTINGS_FILE="${CLAUDE_DIR}/settings.json"

mkdir -p "${CLAUDE_DIR}"

cat > "${SETTINGS_FILE}" <<EOF
{
  "env": {
    "API_BASE_URL": "http://localhost:8080/reading-list/v1",
    "API_KEY": "${API_KEY_VALUE}"
  }
}
EOF
success "Wrote ${SETTINGS_FILE}."

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
echo ""
echo "============================================================"
echo " Setup complete!"
echo ""
echo " Gateway health : ${GATEWAY_HEALTH_URL}"
echo " Management API : ${GATEWAY_MGMT_URL}"
echo " Reading List   : http://localhost:8080/reading-list/v1/books"
echo ""
echo " Quick test:"
echo "   curl http://localhost:8080/reading-list/v1/books \\"
echo "        -H 'X-API-Key: ${API_KEY_VALUE}'"
echo ""
echo " Run full test suite : ./test.sh"
echo " Start Claude Code   : cd ${SCRIPT_DIR} && claude"
echo "============================================================"
