#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
DIST_VERSION="1.1.0"
DIST_NAME="wso2apip-ai-gateway-${DIST_VERSION}"
DIST_ZIP="${DIST_NAME}.zip"
DIST_URL="https://github.com/wso2/api-platform/releases/download/ai-gateway/v${DIST_VERSION}/${DIST_ZIP}"

GATEWAY_MGMT_URL="http://localhost:9090/api/management/v0.9"
GATEWAY_HEALTH_URL="http://localhost:9094/health"
AUTH_HEADER="Authorization: Basic YWRtaW46YWRtaW4="   # admin:admin

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROVIDER_YAML="${SCRIPT_DIR}/llm-provider.yaml"
PROXY_YAMLS=("${SCRIPT_DIR}/llm-proxy-persona.yaml" "${SCRIPT_DIR}/llm-proxy-suffix.yaml")

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info()    { echo "[INFO]  $*"; }
success() { echo "[OK]    $*"; }
error()   { echo "[ERROR] $*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# Resolve Anthropic API key — arg > env var > interactive prompt (required).
# OPENAI_API_KEY is still honored for backward compatibility.
# ---------------------------------------------------------------------------
API_KEY="${1:-${ANTHROPIC_API_KEY:-${OPENAI_API_KEY:-}}}"
if [[ -z "${API_KEY}" ]]; then
  read -rsp "Enter your Anthropic API key (sk-ant-...): " API_KEY
  echo
fi
[[ -n "${API_KEY}" ]] || error "Anthropic API key is required."

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
# Step 1 — Download distribution
# ---------------------------------------------------------------------------
info "Downloading ${DIST_ZIP} ..."
if [[ -f "${DIST_ZIP}" ]]; then
  info "Archive already exists, skipping download."
else
  wget -q --show-progress "${DIST_URL}" -O "${DIST_ZIP}"
  success "Downloaded ${DIST_ZIP}."
fi

# ---------------------------------------------------------------------------
# Step 2 — Unzip
# ---------------------------------------------------------------------------
if [[ -d "${DIST_NAME}" ]]; then
  info "Distribution directory '${DIST_NAME}' already exists, skipping unzip."
else
  info "Unzipping ${DIST_ZIP} ..."
  unzip -q "${DIST_ZIP}"
  success "Extracted to ${DIST_NAME}/."
fi

# ---------------------------------------------------------------------------
# Step 3 — Start the stack
# ---------------------------------------------------------------------------
info "Starting Docker Compose stack in ${DIST_NAME}/ ..."
(cd "${DIST_NAME}" && docker compose up -d)
success "Docker Compose stack started."

# ---------------------------------------------------------------------------
# Step 4 — Health check
# ---------------------------------------------------------------------------
wait_for_health "${GATEWAY_HEALTH_URL}"

# ---------------------------------------------------------------------------
# Step 5 — Deploy LLM provider
# ---------------------------------------------------------------------------
info "Deploying LLM provider from ${PROVIDER_YAML} ..."
[[ -f "${PROVIDER_YAML}" ]] || error "llm-provider.yaml not found at ${PROVIDER_YAML}"

PROVIDER_YAML_TMP=$(mktemp)
trap 'rm -f "${PROVIDER_YAML_TMP}"' EXIT
sed "s|<API_KEY>|Bearer ${API_KEY}|g" "${PROVIDER_YAML}" > "${PROVIDER_YAML_TMP}"

HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X POST "${GATEWAY_MGMT_URL}/llm-providers" \
  -H "Content-Type: application/yaml" \
  -H "${AUTH_HEADER}" \
  --data-binary "@${PROVIDER_YAML_TMP}")

if [[ "${HTTP_STATUS}" =~ ^2 ]]; then
  success "LLM provider deployed (HTTP ${HTTP_STATUS})."
else
  error "Failed to deploy LLM provider (HTTP ${HTTP_STATUS})."
fi

# ---------------------------------------------------------------------------
# Step 6 — Deploy LLM proxies (one per decoration mode)
# ---------------------------------------------------------------------------
for PROXY_YAML in "${PROXY_YAMLS[@]}"; do
  info "Deploying LLM proxy from ${PROXY_YAML} ..."
  [[ -f "${PROXY_YAML}" ]] || error "Proxy file not found at ${PROXY_YAML}"

  HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "${GATEWAY_MGMT_URL}/llm-proxies" \
    -H "Content-Type: application/yaml" \
    -H "${AUTH_HEADER}" \
    --data-binary "@${PROXY_YAML}")

  if [[ "${HTTP_STATUS}" =~ ^2 ]]; then
    success "LLM proxy deployed (HTTP ${HTTP_STATUS})."
  else
    error "Failed to deploy LLM proxy from ${PROXY_YAML} (HTTP ${HTTP_STATUS})."
  fi
done

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
echo ""
echo "============================================================"
echo " Setup complete!"
echo "  Gateway health : ${GATEWAY_HEALTH_URL}"
echo "  Management API : ${GATEWAY_MGMT_URL}"
echo ""
echo " Proxy endpoints:"
echo "   Chat decoration : http://localhost:8080/persona-proxy/chat/completions"
echo "   Text decoration : http://localhost:8080/suffix-proxy/chat/completions"
echo ""
echo " Run the tests:"
echo "   ./test-chat-decoration.sh"
echo "   ./test-text-decoration.sh"
echo "============================================================"
