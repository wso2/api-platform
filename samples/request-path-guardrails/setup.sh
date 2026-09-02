#!/usr/bin/env bash
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
# v1.2.0 is the latest released AI Gateway version as of this writing (the
# next unreleased version's docs live under docs-api-platform's "next" tree —
# this sample intentionally targets the latest *released* version instead).
DIST_VERSION="1.2.0"
DIST_NAME="wso2apip-ai-gateway-${DIST_VERSION}"
DIST_ZIP="${DIST_NAME}.zip"
DIST_URL="https://github.com/wso2/api-platform/releases/download/ai-gateway/v${DIST_VERSION}/${DIST_ZIP}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[[ -f "${SCRIPT_DIR}/.env" ]] && source "${SCRIPT_DIR}/.env"

# These match the host ports the downloaded distribution's own
# docker-compose.yaml hardcodes for the gateway-controller (9090, 9094) and
# gateway-runtime (8443) containers — the URLs built below only reach the
# gateway if they agree with those values.
MGMT_PORT=9090
HEALTH_PORT=9094
TRAFFIC_PORT=8443
MAX_RETRIES="${MAX_RETRIES:-30}"
RETRY_INTERVAL=5
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-guardrails-demo-admin-pw}"

GATEWAY_MGMT_URL="http://localhost:${MGMT_PORT}/api/management/v1"
GATEWAY_HEALTH_URL="http://localhost:${HEALTH_PORT}/api/admin/v1/health"
AUTH_HEADER="Authorization: Basic $(printf %s "${ADMIN_USERNAME}:${ADMIN_PASSWORD}" | base64 | tr -d '\r\n')"

PROVIDER_YAML="${SCRIPT_DIR}/llm-provider.yaml"
PROXY_YAML="${SCRIPT_DIR}/llm-proxy.yaml"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
# Defined here, before the EMBEDDING_PROVIDER validation block below, because
# that block calls error() on its very first validation failure — bash doesn't
# hoist function definitions, so error() must already exist by the time any
# code path can reach it.
info()    { echo "[INFO]  $*"; }
success() { echo "[OK]    $*"; }
error()   { echo "[ERROR] $*" >&2; exit 1; }

# Optional: point semantic-prompt-guard at a REAL embedding provider
# (OPENAI, MISTRAL, or AZURE_OPENAI) instead of the WireMock mock — set
# EMBEDDING_PROVIDER plus its companions to opt in. See README "Optional
# Configuration" for the exact commands, the interactive-prompt alternative
# to EMBEDDING_PROVIDER_API_KEY, and the false-positive/threshold caveat a
# real provider uncovers. The key itself is never written to disk — passed
# to the containers purely via Docker Compose environment passthrough
# (Step 5b below) — so never put it in .env either.
prompt_for_embedding_api_key() {
  local prompt_label="$1"
  if [[ -z "${EMBEDDING_PROVIDER_API_KEY:-}" ]]; then
    # `|| true`: `read` returning non-zero (closed stdin — non-interactive
    # shell, CI) would otherwise abort the script here with no error message
    # at all under `set -euo pipefail`. The `[[ -n ... ]] || error` check
    # after each call site is what actually reports a missing key.
    read -rsp "${prompt_label}" EMBEDDING_PROVIDER_API_KEY || true
    echo
  fi
  # Unconditional, outside the `if`: any value this variable holds — from
  # `read`, a shell export, or even an unsupported `.env` source — is
  # invisible to the `docker compose up` subshell later without an explicit
  # `export` here. No security cost: the value already lives in a plain
  # shell variable either way; this only controls whether the child process
  # that needs it can see it.
  export EMBEDDING_PROVIDER_API_KEY
}

USE_REAL_EMBEDDING_PROVIDER=false
if [[ -n "${EMBEDDING_PROVIDER:-}" ]]; then
  USE_REAL_EMBEDDING_PROVIDER=true
  case "${EMBEDDING_PROVIDER}" in
    OPENAI|MISTRAL|AZURE_OPENAI) ;;
    *) error "EMBEDDING_PROVIDER must be one of OPENAI, MISTRAL, AZURE_OPENAI (got '${EMBEDDING_PROVIDER}') — these are the only three semantic-prompt-guard supports." ;;
  esac
  [[ -n "${EMBEDDING_PROVIDER_ENDPOINT:-}" ]] || error "EMBEDDING_PROVIDER_ENDPOINT is required when EMBEDDING_PROVIDER is set."
  # Rejects a value containing a double-quote, backslash, or newline: these
  # get interpolated into a TOML basic string literal below (`"%s"`) with no
  # escaping. A quote or newline would break out of the literal and inject
  # arbitrary keys into the merged config.toml; a backslash is TOML's own
  # escape character, so an unrecognized sequence is a hard parse error and a
  # recognized one (e.g. `\t`) silently mutates the value instead.
  [[ "${EMBEDDING_PROVIDER_ENDPOINT}" != *[\"\\$'\n']* ]] || error "EMBEDDING_PROVIDER_ENDPOINT must not contain a double-quote, backslash, or newline."
  # Human-cased label for the prompt only (validation above still checks the
  # raw, all-caps EMBEDDING_PROVIDER value) — cosmetic, so the prompt reads
  # "Enter your OpenAI API key:" instead of surfacing the raw env-var value.
  case "${EMBEDDING_PROVIDER}" in
    OPENAI) EMBEDDING_PROVIDER_LABEL="OpenAI" ;;
    MISTRAL) EMBEDDING_PROVIDER_LABEL="Mistral" ;;
    AZURE_OPENAI) EMBEDDING_PROVIDER_LABEL="Azure OpenAI" ;;
  esac
  prompt_for_embedding_api_key "Enter your ${EMBEDDING_PROVIDER_LABEL} embedding-provider API key: "
  [[ -n "${EMBEDDING_PROVIDER_API_KEY:-}" ]] || error "EMBEDDING_PROVIDER_API_KEY is required when EMBEDDING_PROVIDER is set."
  # Per the official semantic-prompt-guard docs, embeddingModel is required for
  # OPENAI/MISTRAL but not AZURE_OPENAI (whose deployment name lives in the
  # endpoint URL instead).
  if [[ "${EMBEDDING_PROVIDER}" != "AZURE_OPENAI" && -z "${EMBEDDING_PROVIDER_MODEL:-}" ]]; then
    error "EMBEDDING_PROVIDER_MODEL is required for EMBEDDING_PROVIDER=${EMBEDDING_PROVIDER} (only AZURE_OPENAI can omit it)."
  fi
  # Same TOML-injection guard as EMBEDDING_PROVIDER_ENDPOINT above.
  [[ "${EMBEDDING_PROVIDER_MODEL:-}" != *[\"\\$'\n']* ]] || error "EMBEDDING_PROVIDER_MODEL must not contain a double-quote, backslash, or newline."
  # EMBEDDING_PROVIDER_DIMENSION is written unquoted (a bare TOML integer, not
  # a string) — TOML forbids leading zeros on integers, so "0" and anything
  # non-numeric must be rejected too, not just non-digit characters.
  [[ -z "${EMBEDDING_PROVIDER_DIMENSION:-}" || "${EMBEDDING_PROVIDER_DIMENSION}" =~ ^[1-9][0-9]*$ ]] || error "EMBEDDING_PROVIDER_DIMENSION must be a positive integer with no leading zeros (got '${EMBEDDING_PROVIDER_DIMENSION}')."
elif [[ -f "${SCRIPT_DIR}/additional-config.toml" ]] && grep -q 'env "EMBEDDING_PROVIDER_API_KEY"' "${SCRIPT_DIR}/additional-config.toml"; then
  # Covers hand-editing additional-config.toml directly (README "Optional
  # Configuration") instead of the env-var shortcut — checked before Step 1
  # so a missing key fails fast, not after downloading/provisioning/starting.
  prompt_for_embedding_api_key "additional-config.toml references a real embedding provider — enter the EMBEDDING_PROVIDER_API_KEY value: "
  [[ -n "${EMBEDDING_PROVIDER_API_KEY:-}" ]] || error "additional-config.toml references \${EMBEDDING_PROVIDER_API_KEY} but no value was provided. Export it (never in .env) and re-run: EMBEDDING_PROVIDER_API_KEY=your-real-key ./setup.sh"
elif [[ -n "${EMBEDDING_PROVIDER_API_KEY:-}${EMBEDDING_PROVIDER_MODEL:-}${EMBEDDING_PROVIDER_ENDPOINT:-}${EMBEDDING_PROVIDER_DIMENSION:-}" ]]; then
  # Any one of these set alone (EMBEDDING_PROVIDER unset, and the static
  # additional-config.toml doesn't reference it either) would otherwise
  # silently run in mock mode with the value entirely ignored — no error, no
  # warning. Fail fast: this only makes sense as a mistake or a stale
  # leftover env var.
  error "EMBEDDING_PROVIDER_API_KEY/EMBEDDING_PROVIDER_MODEL/EMBEDDING_PROVIDER_ENDPOINT/EMBEDDING_PROVIDER_DIMENSION is set but EMBEDDING_PROVIDER is not — it would be silently ignored (the default additional-config.toml doesn't reference it). Either set EMBEDDING_PROVIDER too, or unset it to use the WireMock mock."
fi

wait_for_health() {
  local url="$1"
  info "Waiting for gateway to be healthy at ${url} ..."
  for i in $(seq 1 "${MAX_RETRIES}"); do
    if curl -sf "${url}" > /dev/null 2>&1; then
      success "Gateway is healthy."
      return 0
    fi
    echo "  attempt ${i}/${MAX_RETRIES} — retrying in ${RETRY_INTERVAL}s ..."
    sleep "${RETRY_INTERVAL}"
  done
  error "Gateway did not become healthy after $((MAX_RETRIES * RETRY_INTERVAL))s. Check: (cd ${DIST_NAME} && docker compose logs)"
}

# ---------------------------------------------------------------------------
# Step 1 — Download distribution
# ---------------------------------------------------------------------------
info "Downloading ${DIST_ZIP} ..."
if [[ -f "${SCRIPT_DIR}/${DIST_ZIP}" ]]; then
  info "Archive already exists, skipping download."
else
  (cd "${SCRIPT_DIR}" && curl -fsSL --progress-bar "${DIST_URL}" -o "${DIST_ZIP}")
  success "Downloaded ${DIST_ZIP}."
fi

# ---------------------------------------------------------------------------
# Step 2 — Unzip
# ---------------------------------------------------------------------------
if [[ -d "${SCRIPT_DIR}/${DIST_NAME}" ]]; then
  info "Distribution directory '${DIST_NAME}' already exists, skipping unzip."
else
  info "Unzipping ${DIST_ZIP} ..."
  (cd "${SCRIPT_DIR}" && unzip -q "${DIST_ZIP}")
  success "Extracted to ${DIST_NAME}/."
fi

# ---------------------------------------------------------------------------
# Step 3 — One-time gateway provisioning (TLS cert, AES-256 key, admin credential)
# ---------------------------------------------------------------------------
# v1.2.0 ships no default credential and fails closed without running this
# first — it generates the listener TLS cert, the at-rest encryption key, and
# api-platform.env. Passing ADMIN_USERNAME/ADMIN_PASSWORD makes it
# non-interactive and deterministic (matches the AUTH_HEADER above); omit
# them and the script prompts + prints a random password once instead.
info "Running one-time gateway provisioning (scripts/setup.sh) ..."
(cd "${SCRIPT_DIR}/${DIST_NAME}" && ADMIN_USERNAME="${ADMIN_USERNAME}" ADMIN_PASSWORD="${ADMIN_PASSWORD}" ./scripts/setup.sh)
success "Gateway provisioned."

# ---------------------------------------------------------------------------
# Step 4 — Start the mock LLM backend (WireMock)
# ---------------------------------------------------------------------------
info "Starting WireMock mock LLM backend ..."
docker rm -f mock-llm-openai > /dev/null 2>&1 || true
docker run -d --name mock-llm-openai \
  -p 8082:8080 \
  -v "${SCRIPT_DIR}/wiremock/mappings:/home/wiremock/mappings" \
  wiremock/wiremock:3.3.1 > /dev/null
success "Mock LLM backend started (WireMock on host port 8082, serving /v1/chat/completions and /v1/embeddings)."

# ---------------------------------------------------------------------------
# Step 5 — Merge additional-config.toml into the gateway config
# ---------------------------------------------------------------------------
GATEWAY_CONFIG="${SCRIPT_DIR}/${DIST_NAME}/configs/config.toml"
[[ -f "${GATEWAY_CONFIG}" ]] || error "Gateway config.toml not found at ${GATEWAY_CONFIG}"

if [[ "${USE_REAL_EMBEDDING_PROVIDER}" == true ]]; then
  # Generated on the fly from EMBEDDING_PROVIDER*/env vars — never a static
  # file, so no real value (only the api-key env-var *name*) ever touches
  # disk here. Cleaned up right after use.
  ADDITIONAL_CONFIG=$(mktemp)
  CONFIG_LABEL="generated real-embedding-provider config (EMBEDDING_PROVIDER=${EMBEDDING_PROVIDER})"
  {
    printf 'embedding_provider = "%s"\n' "${EMBEDDING_PROVIDER}"
    printf 'embedding_provider_endpoint = "%s"\n' "${EMBEDDING_PROVIDER_ENDPOINT}"
    [[ -n "${EMBEDDING_PROVIDER_MODEL:-}" ]] && printf 'embedding_provider_model = "%s"\n' "${EMBEDDING_PROVIDER_MODEL}"
    [[ -n "${EMBEDDING_PROVIDER_DIMENSION:-}" ]] && printf 'embedding_provider_dimension = %s\n' "${EMBEDDING_PROVIDER_DIMENSION}"
    printf "embedding_provider_api_key = '{{ env \"EMBEDDING_PROVIDER_API_KEY\" \"\" }}'\n"
  } > "${ADDITIONAL_CONFIG}"
else
  ADDITIONAL_CONFIG="${SCRIPT_DIR}/additional-config.toml"
  CONFIG_LABEL="additional-config.toml"
  [[ -f "${ADDITIONAL_CONFIG}" ]] || error "additional-config.toml not found at ${ADDITIONAL_CONFIG}"
fi

# Use the first key in additional-config.toml as a sentinel for idempotency.
# `|| true` + explicit check: additional-config.toml is documented as
# user-hand-editable — if it's ever emptied or made all-comments, `grep -m1`
# finds nothing and exits 1, which under `set -e` would otherwise abort the
# script right here with a raw, unhelpful bash error instead of the `error()`
# message every other failure path in this script goes through.
#
# This sentinel is always the literal key name "embedding_provider" (the
# first line written by both the mock and generated-config branches above),
# so the presence check below detects only whether ANY embedding-provider
# config was merged, not whether it matches the CURRENT run's values.
# Switching EMBEDDING_PROVIDER between two runs without a teardown in
# between leaves the gateway's config.toml on the earlier run's values — see
# README "Setup" for the corresponding caveat on the idempotency claim.
SENTINEL=$(grep -m1 '^\s*[a-zA-Z]' "${ADDITIONAL_CONFIG}" | cut -d'=' -f1 | tr -d ' ') || true
[[ -n "${SENTINEL}" ]] || error "${CONFIG_LABEL} has no top-level key to merge — it must contain at least one bare 'key = value' line."
if grep -q "^${SENTINEL}" "${GATEWAY_CONFIG}"; then
  info "${CONFIG_LABEL} already merged into ${GATEWAY_CONFIG}, skipping."
else
  info "Merging ${CONFIG_LABEL} into ${GATEWAY_CONFIG} ..."
  # Prepend, not append: additional-config.toml holds bare top-level keys, and
  # config.toml opens with a [section] header on its very first line — TOML
  # keys bind to whichever section precedes them, so anything appended after
  # a header would be silently absorbed into that section instead of staying
  # global.
  TMP_MERGED=$(mktemp)
  { cat "${ADDITIONAL_CONFIG}"; echo ""; cat "${GATEWAY_CONFIG}"; } > "${TMP_MERGED}"
  mv "${TMP_MERGED}" "${GATEWAY_CONFIG}"
  success "Config merged."
fi
[[ "${USE_REAL_EMBEDDING_PROVIDER}" == true ]] && rm -f "${ADDITIONAL_CONFIG}"

# ---------------------------------------------------------------------------
# Step 5b — Optional: wire EMBEDDING_PROVIDER_API_KEY passthrough into
# docker-compose.yaml
# ---------------------------------------------------------------------------
# semantic-prompt-guard's embedding call is made by gateway-runtime, not
# gateway-controller — but both containers mount the same config.toml, so
# both need EMBEDDING_PROVIDER_API_KEY visible or the `{{ env ... }}`
# template can't resolve on either side. This adds a bare
# `- EMBEDDING_PROVIDER_API_KEY` entry (name only) to both services'
# `environment:` list; Docker Compose passes through whatever this script's
# own shell environment holds at `docker compose up` time — the value never
# gets written into docker-compose.yaml or any other file.
#
# Gated on whether the MERGED config.toml references the template, not on
# EMBEDDING_PROVIDER — so this also works for a hand-edited
# additional-config.toml using the same template, not just the env-var path.
NEEDS_EMBEDDING_KEY_PASSTHROUGH=false
grep -q 'env "EMBEDDING_PROVIDER_API_KEY"' "${GATEWAY_CONFIG}" && NEEDS_EMBEDDING_KEY_PASSTHROUGH=true

if [[ "${NEEDS_EMBEDDING_KEY_PASSTHROUGH}" == true ]]; then
  [[ -n "${EMBEDDING_PROVIDER_API_KEY:-}" ]] || error "configs/config.toml references \${EMBEDDING_PROVIDER_API_KEY} but that variable is not set in your shell. Export it (never in .env) and re-run: EMBEDDING_PROVIDER_API_KEY=your-real-key ./setup.sh"
  COMPOSE_FILE="${SCRIPT_DIR}/${DIST_NAME}/docker-compose.yaml"
  [[ -f "${COMPOSE_FILE}" ]] || error "docker-compose.yaml not found at ${COMPOSE_FILE}"
  # Idempotency check requires BOTH services already patched (count == 2), not
  # just "present somewhere" — a prior partial patch (e.g. from an interrupted
  # run) would otherwise be mistaken for "already done" and never get completed.
  if [[ "$(grep -c "EMBEDDING_PROVIDER_API_KEY" "${COMPOSE_FILE}" || true)" -eq 2 ]]; then
    info "docker-compose.yaml already wired for EMBEDDING_PROVIDER_API_KEY passthrough, skipping."
  else
    info "Wiring EMBEDDING_PROVIDER_API_KEY passthrough into docker-compose.yaml (variable name only — the key itself is never written to this or any file) ..."
    command -v python3 > /dev/null 2>&1 || error "python3 is required to patch docker-compose.yaml for a real embedding provider — install it and re-run."
    python3 - "${COMPOSE_FILE}" <<'PYEOF'
import re
import sys

path = sys.argv[1]
with open(path) as f:
    lines = f.readlines()

# Both gateway-controller and gateway-runtime have an identical `env_file: /
# format: raw` block, so an anchor like "format: raw" alone matches inside
# BOTH services — this tracks which top-level service block we're currently
# in (a line matching "  <name>:" at 2-space indent starts a new service) so
# each patch only ever applies once, in the right block.
service_header = re.compile(r"^  ([a-zA-Z0-9_-]+):\s*$")

# First pass: detect which services already have the passthrough (e.g. a
# prior interrupted/partial run patched only one of the two) — patching is
# per-service idempotent, re-inserting into an already-patched service would
# otherwise produce a duplicate `environment:` key on any repair re-run.
already_patched = set()
current_service = None
for line in lines:
    m = service_header.match(line.rstrip("\n"))
    if m:
        current_service = m.group(1)
    if current_service and "EMBEDDING_PROVIDER_API_KEY" in line:
        already_patched.add(current_service)

# Second pass: patch only services that still need it.
patched_services = set(already_patched)
current_service = None
out = []
for line in lines:
    m = service_header.match(line.rstrip("\n"))
    if m:
        current_service = m.group(1)
    out.append(line)
    stripped = line.rstrip("\n")

    # gateway-controller has no `environment:` block yet — add one right
    # after its `env_file:` block (identified by the `format: raw` line).
    if current_service == "gateway-controller" and current_service not in patched_services and stripped == "        format: raw":
        out.append("    environment:\n")
        out.append("      - EMBEDDING_PROVIDER_API_KEY\n")
        patched_services.add(current_service)

    # gateway-runtime already has an `environment:` block — append to it.
    elif current_service == "gateway-runtime" and current_service not in patched_services and stripped == "      - GATEWAY_CONTROLLER_HOST=gateway-controller":
        out.append("      - EMBEDDING_PROVIDER_API_KEY\n")
        patched_services.add(current_service)

with open(path, "w") as f:
    f.writelines(out)

# Printed for the bash caller to verify — both services must be patched, not
# just "at least one", or the real embedding call (made by gateway-runtime)
# could still never see the key even though the grep-based check would
# otherwise report success on a single, partial match.
print(",".join(sorted(patched_services)))
PYEOF
    PATCH_COUNT=$(grep -c "EMBEDDING_PROVIDER_API_KEY" "${COMPOSE_FILE}" || true)
    # The count above is a plain substring match, not YAML-aware — it can't
    # tell a correct patch from one that inserted a duplicate `environment:`
    # key (e.g. if a future distribution's gateway-controller block already
    # had one). `docker compose config` actually parses the file against
    # Compose's schema, catching structural corruption the count alone would
    # silently accept.
    if ! (cd "${SCRIPT_DIR}/${DIST_NAME}" && docker compose config > /dev/null); then
      error "docker-compose.yaml failed to parse after wiring EMBEDDING_PROVIDER_API_KEY passthrough — the patch likely produced invalid YAML (e.g. a duplicate key). Check ${COMPOSE_FILE} manually."
    fi
    if [[ "${PATCH_COUNT}" -eq 2 ]]; then
      success "docker-compose.yaml wired for EMBEDDING_PROVIDER_API_KEY passthrough (gateway-controller and gateway-runtime)."
    elif [[ "${PATCH_COUNT}" -eq 1 ]]; then
      error "Only partially wired EMBEDDING_PROVIDER_API_KEY passthrough into docker-compose.yaml (1 of 2 services matched) — the v${DIST_VERSION} distribution's file layout may have changed for one service. Check ${COMPOSE_FILE} manually; semantic-prompt-guard's real embedding call is made by gateway-runtime, so a controller-only patch would silently never deliver the key."
    else
      error "Failed to wire EMBEDDING_PROVIDER_API_KEY passthrough into docker-compose.yaml — the v${DIST_VERSION} distribution's file layout may have changed. Check ${COMPOSE_FILE} manually."
    fi
  fi
fi

# ---------------------------------------------------------------------------
# Step 6 — Start the WSO2 AI Gateway stack
# ---------------------------------------------------------------------------
info "Starting Docker Compose stack in ${DIST_NAME}/ ..."
(cd "${SCRIPT_DIR}/${DIST_NAME}" && docker compose up -d)
success "Docker Compose stack started."

# ---------------------------------------------------------------------------
# Step 7 — Health check
# ---------------------------------------------------------------------------
wait_for_health "${GATEWAY_HEALTH_URL}"

# ---------------------------------------------------------------------------
# Step 8 — Connect the mock LLM backend to the gateway's Docker network
# ---------------------------------------------------------------------------
info "Connecting mock LLM backend to the gateway network ..."
# Read the network directly off the running gateway-controller container
# rather than grepping `docker network ls` by name — a host with leftover
# networks from other/earlier gateway stacks can have more than one
# "*gateway-network*" match, and grepping would pick an arbitrary one.
# `|| true` on both: bare command substitutions would otherwise abort the
# script under `set -e` with no diagnostic on a genuine docker failure (e.g.
# a race where the container disappears) — the checks below report that
# through error() instead.
GATEWAY_CONTROLLER_ID=$(cd "${SCRIPT_DIR}/${DIST_NAME}" && docker compose ps -q gateway-controller) || true
GATEWAY_NETWORK=""
if [[ -n "${GATEWAY_CONTROLLER_ID}" ]]; then
  # Newline-separated + `head -n1`: if the container is attached to more than
  # one network (e.g. leftover networks from a prior un-`--clean`ed run), this
  # deterministically picks one instead of gluing every network name into a
  # single unusable garbled string.
  DOCKER_INSPECT_ERROR=$(mktemp)
  GATEWAY_NETWORK=$(docker inspect "${GATEWAY_CONTROLLER_ID}" --format '{{range $net, $cfg := .NetworkSettings.Networks}}{{$net}}{{"\n"}}{{end}}' 2>"${DOCKER_INSPECT_ERROR}" | head -n1) || true
  if [[ -z "${GATEWAY_NETWORK}" && -s "${DOCKER_INSPECT_ERROR}" ]]; then
    inspect_error_output=$(cat "${DOCKER_INSPECT_ERROR}")
    rm -f "${DOCKER_INSPECT_ERROR}"
    error "docker inspect failed while resolving the gateway's network: ${inspect_error_output}"
  fi
  rm -f "${DOCKER_INSPECT_ERROR}"
fi
if [[ -n "${GATEWAY_NETWORK}" ]]; then
  # Check the exit code explicitly — `|| true` alone would swallow a genuine
  # failure and still print a false "Connected" success message, leaving
  # mock-llm-openai unreachable with no indication why. mock-llm-openai is
  # always freshly removed and recreated in Step 4 above, so it never already
  # holds a network attachment by the time this runs.
  if CONNECT_OUTPUT=$(docker network connect "${GATEWAY_NETWORK}" mock-llm-openai 2>&1); then
    success "Connected mock-llm-openai to network: ${GATEWAY_NETWORK}"
  else
    error "Failed to connect mock-llm-openai to network ${GATEWAY_NETWORK}: ${CONNECT_OUTPUT}"
  fi
else
  error "Could not detect the gateway's Docker network — mock routing will not work. Check: docker network ls"
fi

# ---------------------------------------------------------------------------
# Step 9 — Deploy the LLM provider
# ---------------------------------------------------------------------------
info "Deploying LLM provider from ${PROVIDER_YAML} ..."
[[ -f "${PROVIDER_YAML}" ]] || error "llm-provider.yaml not found at ${PROVIDER_YAML}"

# `|| echo -e "\n000"` fallback: under `set -euo pipefail`, a bare
# `VAR=$(curl ...)` assignment aborts the script here if curl itself fails
# (connection refused/reset) rather than returning an HTTP status — see the
# same pattern in teardown.sh. Body is captured too, not just status, so a
# re-run against an already-configured environment is recognized as a no-op
# rather than a hard failure.
RESPONSE=$(curl -s -w "\n%{http_code}" \
  -X POST "${GATEWAY_MGMT_URL}/llm-providers" \
  -H "Content-Type: application/yaml" \
  -H "${AUTH_HEADER}" \
  --data-binary "@${PROVIDER_YAML}" || echo -e "\n000")
HTTP_STATUS=$(echo "${RESPONSE}" | tail -1)
RESPONSE_BODY=$(echo "${RESPONSE}" | sed '$d')

if [[ "${HTTP_STATUS}" =~ ^2 ]]; then
  success "LLM provider deployed (HTTP ${HTTP_STATUS})."
elif echo "${RESPONSE_BODY}" | grep -qi "already exists"; then
  info "LLM provider already deployed, skipping (HTTP ${HTTP_STATUS})."
else
  error "Failed to deploy LLM provider (HTTP ${HTTP_STATUS}): ${RESPONSE_BODY}"
fi

# Poll the provider back via GET, rather than a fixed sleep, before the proxy
# deploy references it by id — a fixed sleep has no real bound and can still
# race a slower management-API backend. `metadata.name` is extracted with
# plain text tools (no python3 dependency) since the file's structure is
# fixed and simple; falls back to a short fixed wait if the name can't be
# read, rather than skipping the check entirely.
PROVIDER_NAME=$(grep -A1 '^metadata:' "${PROVIDER_YAML}" | sed -n 's/^[[:space:]]*name:[[:space:]]*//p')
if [[ -n "${PROVIDER_NAME}" ]]; then
  for i in $(seq 1 10); do
    PROVIDER_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -H "${AUTH_HEADER}" "${GATEWAY_MGMT_URL}/llm-providers/${PROVIDER_NAME}" || echo "000")
    [[ "${PROVIDER_STATUS}" == "200" ]] && break
    sleep 1
  done
else
  sleep 3
fi

# ---------------------------------------------------------------------------
# Step 10 — Deploy the LLM proxy (four guardrail policies chained on /chat/completions)
# ---------------------------------------------------------------------------
info "Deploying LLM proxy from ${PROXY_YAML} ..."
[[ -f "${PROXY_YAML}" ]] || error "llm-proxy.yaml not found at ${PROXY_YAML}"

# See the fallback/idempotency comment above the LLM-provider deploy call — same reasons.
RESPONSE=$(curl -s -w "\n%{http_code}" \
  -X POST "${GATEWAY_MGMT_URL}/llm-proxies" \
  -H "Content-Type: application/yaml" \
  -H "${AUTH_HEADER}" \
  --data-binary "@${PROXY_YAML}" || echo -e "\n000")
HTTP_STATUS=$(echo "${RESPONSE}" | tail -1)
RESPONSE_BODY=$(echo "${RESPONSE}" | sed '$d')

if [[ "${HTTP_STATUS}" =~ ^2 ]]; then
  success "LLM proxy deployed (HTTP ${HTTP_STATUS})."
elif echo "${RESPONSE_BODY}" | grep -qi "already exists"; then
  info "LLM proxy already deployed, skipping (HTTP ${HTTP_STATUS})."
else
  error "Failed to deploy LLM proxy (HTTP ${HTTP_STATUS}): ${RESPONSE_BODY}"
fi

# ---------------------------------------------------------------------------
# Step 11 — Wait for the route to become live
# ---------------------------------------------------------------------------
info "Waiting for the traffic route to become live ..."
TRAFFIC_URL="https://localhost:${TRAFFIC_PORT}/api/llm/chat/completions"
PROBE_PAYLOAD='{"model":"gpt-4o-mini","messages":[{"role":"user","content":"route probe — Pythagorean theorem"}]}'
for i in $(seq 1 "${MAX_RETRIES}"); do
  STATUS=$(curl -sk -o /dev/null -w "%{http_code}" -X POST "${TRAFFIC_URL}" \
    -H "Content-Type: application/json" -d "${PROBE_PAYLOAD}" || true)
  # 000 = connection failed (Envoy/controller not reachable yet); 404 = Envoy
  # is up but this route isn't registered yet (route propagation via xDS
  # lags a moment behind the "deployed" response from the management API).
  # Anything else (200/422/500/...) means the route matched and the request
  # reached the policy chain — that's what "live" means here.
  if [[ -n "${STATUS}" && "${STATUS}" != "000" && "${STATUS}" != "404" ]]; then
    success "Route is live (HTTP ${STATUS})."
    break
  fi
  if [[ "${i}" -eq "${MAX_RETRIES}" ]]; then
    error "Route did not become live after $((MAX_RETRIES * RETRY_INTERVAL))s. Check: (cd ${DIST_NAME} && docker compose logs)"
  fi
  echo "  attempt ${i}/${MAX_RETRIES} — retrying in ${RETRY_INTERVAL}s ..."
  sleep "${RETRY_INTERVAL}"
done

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
echo ""
echo "============================================================"
echo " Setup complete!"
echo "  Gateway health  : ${GATEWAY_HEALTH_URL}"
echo "  Management API  : ${GATEWAY_MGMT_URL}"
echo "  Guardrails proxy: ${TRAFFIC_URL} (self-signed TLS — curl needs -k)"
# Reflects what actually ended up in config.toml, not just which input path
# was used. `|| echo "unknown"` fallback: a hand-edited additional-config.toml
# could use different quoting/spacing than this grep expects; under
# `set -euo pipefail` an unmatched grep would otherwise abort the script here
# — after "Setup complete!" has printed but before the "Run the tests" footer
# — over a purely cosmetic detail.
ACTIVE_EMBEDDING_PROVIDER=$(grep -m1 '^embedding_provider = ' "${GATEWAY_CONFIG}" | sed -E 's/^embedding_provider = "(.*)"/\1/') || ACTIVE_EMBEDDING_PROVIDER="unknown"
[[ -z "${ACTIVE_EMBEDDING_PROVIDER}" ]] && ACTIVE_EMBEDDING_PROVIDER="unknown"
if [[ "${NEEDS_EMBEDDING_KEY_PASSTHROUGH}" == true ]]; then
  echo "  Embedding provider: ${ACTIVE_EMBEDDING_PROVIDER} (real API) — see README for the false-positive/threshold caveat"
else
  echo "  Embedding provider: ${ACTIVE_EMBEDDING_PROVIDER} mock (WireMock) — see README to point this at a real provider instead"
fi
echo ""
echo " Run the tests:"
echo "   ./test-content-length-guard.sh"
echo "   ./test-word-count-guard.sh"
echo "   ./test-prompt-injection-guard.sh"
echo "   ./test-semantic-guard.sh"
echo "   ./test-combined-attack.sh"
echo "============================================================"
