#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration — must match setup.sh
# ---------------------------------------------------------------------------
DIST_VERSION="1.2.0"
DIST_NAME="wso2apip-ai-gateway-${DIST_VERSION}"
DIST_ZIP="${DIST_NAME}.zip"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[[ -f "${SCRIPT_DIR}/.env" ]] && source "${SCRIPT_DIR}/.env"

# Must match setup.sh's own MGMT_PORT/HEALTH_PORT — the host ports the
# downloaded distribution's docker-compose.yaml hardcodes.
MGMT_PORT=9090
HEALTH_PORT=9094
GATEWAY_MGMT_URL="http://localhost:${MGMT_PORT}/api/management/v1"
AUTH_HEADER="Authorization: Basic $(printf %s "${ADMIN_USERNAME:-admin}:${ADMIN_PASSWORD:-guardrails-demo-admin-pw}" | base64 | tr -d '\r\n')"   # must match ADMIN_USERNAME/ADMIN_PASSWORD passed to setup.sh

PROVIDER_YAML="${SCRIPT_DIR}/llm-provider.yaml"
PROXY_YAML="${SCRIPT_DIR}/llm-proxy.yaml"

# Pass --clean to also remove the extracted directory and downloaded zip archive.
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

yaml_name() {
  # `|| echo ""` fallback: `command -v python3` at the call sites only checks
  # the interpreter exists, not that PyYAML is importable — if it's missing
  # (a stock python3 with no `pip install pyyaml`), this would otherwise raise
  # ModuleNotFoundError and, as a bare command substitution under `set -e`,
  # abort teardown before it ever reaches the container-stopping steps below.
  python3 -c "import yaml,sys; d=yaml.safe_load(open(sys.argv[1])); print(d['metadata']['name'])" "$1" 2>/dev/null || echo ""
}

delete_resource() {
  local kind="$1"   # llm-providers or llm-proxies
  local name="$2"
  info "Deleting ${kind}/${name} ..."
  # `|| true` + explicit fallback: if the gateway is unreachable, curl exits
  # non-zero (e.g. 7 = connection refused) *before* writing the %{http_code}
  # substitution — under `set -e`, a bare `VAR=$(curl ...)` assignment with no
  # `||`/`if` around it would abort the whole script right here instead of
  # letting teardown continue to stop the containers below.
  HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X DELETE "${GATEWAY_MGMT_URL}/${kind}/${name}" \
    -H "${AUTH_HEADER}" || echo "000")
  if [[ "${HTTP_STATUS}" =~ ^2 ]]; then
    success "Deleted ${kind}/${name} (HTTP ${HTTP_STATUS})."
  elif [[ "${HTTP_STATUS}" == "404" ]]; then
    warn "${kind}/${name} not found — already deleted?"
  elif [[ "${HTTP_STATUS}" == "000" ]]; then
    warn "Could not reach the management API to delete ${kind}/${name} — continuing teardown."
  else
    warn "Failed to delete ${kind}/${name} (HTTP ${HTTP_STATUS}) — continuing teardown."
  fi
}

# ---------------------------------------------------------------------------
# Step 1 — Delete the LLM proxy and provider
# ---------------------------------------------------------------------------
GATEWAY_REACHABLE=false
if curl -sf -o /dev/null "http://localhost:${HEALTH_PORT}/api/admin/v1/health" 2>/dev/null; then
  GATEWAY_REACHABLE=true
fi

if [[ "${GATEWAY_REACHABLE}" == false ]]; then
  warn "Gateway not reachable at http://localhost:${HEALTH_PORT}/api/admin/v1/health — skipping resource deletion, still stopping containers below."
fi

PYTHON3_AVAILABLE=false
command -v python3 > /dev/null 2>&1 && PYTHON3_AVAILABLE=true
if [[ "${GATEWAY_REACHABLE}" == true && "${PYTHON3_AVAILABLE}" == false ]]; then
  warn "python3 not found — cannot read resource names from YAML, skipping their deletion, continuing teardown."
fi

if [[ "${GATEWAY_REACHABLE}" == true && "${PYTHON3_AVAILABLE}" == true && -f "${PROXY_YAML}" ]]; then
  PROXY_NAME=$(yaml_name "${PROXY_YAML}")
  if [[ -n "${PROXY_NAME}" ]]; then
    delete_resource "llm-proxies" "${PROXY_NAME}"
  else
    warn "Could not read metadata.name from ${PROXY_YAML} (is PyYAML installed? \`pip install pyyaml\`) — skipping its deletion, continuing teardown."
  fi
fi

if [[ "${GATEWAY_REACHABLE}" == true && "${PYTHON3_AVAILABLE}" == true && -f "${PROVIDER_YAML}" ]]; then
  PROVIDER_NAME=$(yaml_name "${PROVIDER_YAML}")
  if [[ -n "${PROVIDER_NAME}" ]]; then
    delete_resource "llm-providers" "${PROVIDER_NAME}"
  else
    warn "Could not read metadata.name from ${PROVIDER_YAML} (is PyYAML installed? \`pip install pyyaml\`) — skipping its deletion, continuing teardown."
  fi
fi

# ---------------------------------------------------------------------------
# Step 2 — Remove the mock LLM backend
# ---------------------------------------------------------------------------
info "Removing mock LLM backend container ..."
docker rm -f mock-llm-openai > /dev/null 2>&1 || true
success "mock-llm-openai removed."

# ---------------------------------------------------------------------------
# Step 3 — Stop the Docker Compose stack
# ---------------------------------------------------------------------------
COMPOSE_DIR="${SCRIPT_DIR}/${DIST_NAME}"
if [[ -d "${COMPOSE_DIR}" ]]; then
  info "Stopping Docker Compose stack ..."
  (cd "${COMPOSE_DIR}" && docker compose down --volumes)
  success "Stack stopped and volumes removed."
else
  warn "Distribution directory not found at ${COMPOSE_DIR} — stack may not be running."
fi

# ---------------------------------------------------------------------------
# Step 4 — Optional cleanup of distribution files
# ---------------------------------------------------------------------------
if [[ "${CLEAN}" == true ]]; then
  if [[ -d "${COMPOSE_DIR}" ]]; then
    info "Removing extracted directory ${DIST_NAME}/ ..."
    rm -rf "${COMPOSE_DIR}"
    success "Removed ${DIST_NAME}/."
  fi
  if [[ -f "${SCRIPT_DIR}/${DIST_ZIP}" ]]; then
    info "Removing archive ${DIST_ZIP} ..."
    rm -f "${SCRIPT_DIR}/${DIST_ZIP}"
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
