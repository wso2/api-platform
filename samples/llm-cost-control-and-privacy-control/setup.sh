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
PROXY_YAML="${SCRIPT_DIR}/llm-proxy.yaml"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info()    { echo "[INFO]  $*"; }
success() { echo "[OK]    $*"; }
error()   { echo "[ERROR] $*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# Resolve OpenAI API key — arg > env var > interactive prompt (required)
# ---------------------------------------------------------------------------
API_KEY="${1:-${OPENAI_API_KEY:-}}"
if [[ -z "${API_KEY}" ]]; then
  read -rsp "Enter your OpenAI API key: " API_KEY
  echo
fi
[[ -n "${API_KEY}" ]] || error "OpenAI API key is required."

# ---------------------------------------------------------------------------
# Resolve Mistral API key — env var > interactive prompt (required)
# Used by the semantic-cache policy to generate embeddings.
# ---------------------------------------------------------------------------
MISTRAL_KEY="${MISTRAL_API_KEY:-}"
if [[ -z "${MISTRAL_KEY}" ]]; then
  read -rsp "Enter your Mistral API key: " MISTRAL_KEY
  echo
fi
[[ -n "${MISTRAL_KEY}" ]] || error "Mistral API key is required."

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
# Step 3 — Merge additional-config.toml into gateway config.toml
# ---------------------------------------------------------------------------
ADDITIONAL_CONFIG="${SCRIPT_DIR}/additional-config.toml"
GATEWAY_CONFIG="${DIST_NAME}/configs/config.toml"

[[ -f "${ADDITIONAL_CONFIG}" ]] || error "additional-config.toml not found at ${ADDITIONAL_CONFIG}"
[[ -f "${GATEWAY_CONFIG}" ]]    || error "Gateway config.toml not found at ${GATEWAY_CONFIG}"

# Use the first key in additional-config.toml as a sentinel for idempotency.
SENTINEL=$(grep -m1 '^\s*[a-zA-Z]' "${ADDITIONAL_CONFIG}" | cut -d'=' -f1 | tr -d ' ')
if grep -q "^${SENTINEL}" "${GATEWAY_CONFIG}"; then
  info "Additional config already merged into ${GATEWAY_CONFIG}, skipping."
else
  info "Merging ${ADDITIONAL_CONFIG} into ${GATEWAY_CONFIG} ..."
  # Prepend, not append: additional-config.toml holds bare top-level keys.
  # Appending after a [section] header in config.toml would bind them to that
  # section, so the policy engine never sees them as the globals it expects.
  TMP_ADDITIONAL=$(mktemp)
  TMP_MERGED=$(mktemp)
  sed "s|<MISTRAL_API_KEY>|${MISTRAL_KEY}|g" "${ADDITIONAL_CONFIG}" > "${TMP_ADDITIONAL}"
  { cat "${TMP_ADDITIONAL}"; echo ""; cat "${GATEWAY_CONFIG}"; } > "${TMP_MERGED}"
  mv "${TMP_MERGED}" "${GATEWAY_CONFIG}"
  rm -f "${TMP_ADDITIONAL}"
  success "Config merged."
fi

# ---------------------------------------------------------------------------
# Step 4 — Merge redis service into docker-compose
# ---------------------------------------------------------------------------
COMPOSE_FILE="${DIST_NAME}/docker-compose.yaml"
[[ -f "${COMPOSE_FILE}" ]] || COMPOSE_FILE="${DIST_NAME}/docker-compose.yml"
[[ -f "${COMPOSE_FILE}" ]] || error "docker-compose file not found in ${DIST_NAME}/"

REDIS_SERVICE_YAML="${SCRIPT_DIR}/redis-service.yaml"
[[ -f "${REDIS_SERVICE_YAML}" ]] || error "redis-service.yaml not found at ${REDIS_SERVICE_YAML}"

info "Merging redis service into ${COMPOSE_FILE} ..."
python3 - "${COMPOSE_FILE}" "${REDIS_SERVICE_YAML}" <<'PYEOF'
import sys, yaml

compose_file, redis_file = sys.argv[1], sys.argv[2]

with open(compose_file) as f:
    compose = yaml.safe_load(f)

with open(redis_file) as f:
    redis_service = yaml.safe_load(f)

if 'redis' in compose.get('services', {}):
    print("Redis service already present, skipping merge.")
    sys.exit(0)

compose.setdefault('services', {}).update(redis_service)
compose.setdefault('volumes', {})['redis-data'] = {'driver': 'local'}

with open(compose_file, 'w') as f:
    yaml.dump(compose, f, default_flow_style=False, allow_unicode=True, sort_keys=False)
PYEOF
success "Redis service merged into docker-compose."

# ---------------------------------------------------------------------------
# Step 5 — Start the stack
# ---------------------------------------------------------------------------
info "Starting Docker Compose stack in ${DIST_NAME}/ ..."
(cd "${DIST_NAME}" && docker compose up -d)
success "Docker Compose stack started."

# ---------------------------------------------------------------------------
# Step 6 — Health check
# ---------------------------------------------------------------------------
wait_for_health "${GATEWAY_HEALTH_URL}"

# ---------------------------------------------------------------------------
# Step 7 — Deploy LLM provider
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
# Step 8 — Deploy LLM proxy
# ---------------------------------------------------------------------------
info "Deploying LLM proxy from ${PROXY_YAML} ..."
[[ -f "${PROXY_YAML}" ]] || error "llm-proxy.yaml not found at ${PROXY_YAML}"

HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X POST "${GATEWAY_MGMT_URL}/llm-proxies" \
  -H "Content-Type: application/yaml" \
  -H "${AUTH_HEADER}" \
  --data-binary "@${PROXY_YAML}")

if [[ "${HTTP_STATUS}" =~ ^2 ]]; then
  success "LLM proxy deployed (HTTP ${HTTP_STATUS})."
else
  error "Failed to deploy LLM proxy (HTTP ${HTTP_STATUS})."
fi

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
echo ""
echo "============================================================"
echo " Setup complete!"
echo "  Gateway health : ${GATEWAY_HEALTH_URL}"
echo "  Management API : ${GATEWAY_MGMT_URL}"
echo ""
echo " Usage:"
echo "   ./setup.sh <openai-api-key>                       # pass OpenAI key as argument"
echo "   OPENAI_API_KEY=sk-... MISTRAL_API_KEY=... ./setup.sh   # or via env vars"
echo "   ./setup.sh                                        # or enter interactively"
echo "============================================================"
