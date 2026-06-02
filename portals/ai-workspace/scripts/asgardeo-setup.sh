#!/usr/bin/env bash
# =============================================================================
# Asgardeo Setup — AI Workspace
# -----------------------------------------------------------------------------
# 1. Adds all platform API scopes to the Asgardeo API resource
# 2. Assigns the correct scope set to each platform role (admin / developer /
#    publisher / operator / viewer)
#
# Usage:
#   export ASGARDEO_TOKEN=<your-management-api-bearer-token>
#   bash scripts/asgardeo-setup.sh
# =============================================================================
set -euo pipefail

# ── Config ────────────────────────────────────────────────────────────────────
TENANT="${ASGARDEO_TENANT:-thushani}"
BASE="https://api.asgardeo.io/t/${TENANT}"
TOKEN="${ASGARDEO_TOKEN:-}"
RESOURCE_ID="${ASGARDEO_RESOURCE_ID:-b2ebfc5a-97fb-454c-bae0-192bf56916d0}"

if [[ -z "$TOKEN" ]]; then
  echo "ERROR: set ASGARDEO_TOKEN env var to a valid management API bearer token"
  exit 1
fi

CURL=(curl -sf
  -H "Authorization: Bearer ${TOKEN}"
  -H "Content-Type: application/json"
  -H "Accept: application/json"
)

# ── Helpers ───────────────────────────────────────────────────────────────────

# Build a JSON array of scope objects from a bash array of scope strings.
# Each entry: {"name":"api-platform:x:y","displayName":"XY","description":""}
scopes_json() {
  local -n _scopes=$1
  local json="["
  local first=true
  for s in "${_scopes[@]}"; do
    # Convert "api-platform:llm_provider:read" → "LlmProviderRead"
    local display
    display=$(echo "$s" | sed 's/api-platform://g; s/[_:]/ /g' \
      | awk '{for(i=1;i<=NF;i++) $i=toupper(substr($i,1,1)) substr($i,2); print}' \
      | tr -d ' ')
    [[ "$first" == true ]] && first=false || json+=","
    json+="{\"name\":\"${s}\",\"displayName\":\"${display}\",\"description\":\"\"}"
  done
  json+="]"
  echo "$json"
}

# Build a SCIM2 PatchOp body to set permissions on a role.
permissions_patch() {
  local -n _scopes=$1
  local json='{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"add","path":"permissions","value":['
  local first=true
  for s in "${_scopes[@]}"; do
    [[ "$first" == true ]] && first=false || json+=","
    json+="{\"value\":\"${s}\"}"
  done
  json+="]}]}"
  echo "$json"
}

# Look up a role ID by display name via SCIM2.
get_role_id() {
  local name="$1"
  local result
  result=$("${CURL[@]}" \
    "${BASE}/scim2/v2/Roles?filter=displayName+eq+${name}" \
    | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
  echo "$result"
}

# ── All scopes (mirrors src/auth/permissions.ts) ──────────────────────────────
ALL_SCOPES=(
  # Projects
  "api-platform:project:read"
  "api-platform:project:create"
  "api-platform:project:update"
  "api-platform:project:delete"
  "api-platform:project:manage"
  # Applications
  "api-platform:application:read"
  "api-platform:application:create"
  "api-platform:application:update"
  "api-platform:application:delete"
  "api-platform:application:manage"
  "api-platform:application:api_key:read"
  "api-platform:application:api_key:manage"
  "api-platform:application:associations:read"
  "api-platform:application:associations:manage"
  # Gateways
  "api-platform:gateway:read"
  "api-platform:gateway:create"
  "api-platform:gateway:update"
  "api-platform:gateway:delete"
  "api-platform:gateway:manage"
  "api-platform:gateway:token:read"
  "api-platform:gateway:token:manage"
  "api-platform:gateway:policy:read"
  "api-platform:gateway:policy:manage"
  "api-platform:gateway:artifacts:read"
  "api-platform:gateway:manifest:read"
  "api-platform:gateway:status:read"
  # LLM Providers
  "api-platform:llm_provider:read"
  "api-platform:llm_provider:create"
  "api-platform:llm_provider:update"
  "api-platform:llm_provider:delete"
  "api-platform:llm_provider:manage"
  "api-platform:llm_provider:key:manage"
  "api-platform:llm_provider:deployment:manage"
  # LLM Proxies
  "api-platform:llm_proxy:read"
  "api-platform:llm_proxy:create"
  "api-platform:llm_proxy:update"
  "api-platform:llm_proxy:delete"
  "api-platform:llm_proxy:manage"
  "api-platform:llm_proxy:key:manage"
  "api-platform:llm_proxy:deployment:manage"
  # LLM Templates
  "api-platform:llm_template:read"
  "api-platform:llm_template:manage"
  # MCP Proxies
  "api-platform:mcp_proxy:read"
  "api-platform:mcp_proxy:create"
  "api-platform:mcp_proxy:update"
  "api-platform:mcp_proxy:delete"
  "api-platform:mcp_proxy:manage"
  "api-platform:mcp_proxy:deployment:manage"
  # DevPortals
  "api-platform:devportal:read"
  "api-platform:devportal:manage"
  # Subscriptions
  "api-platform:subscription:read"
  "api-platform:subscription:manage"
  "api-platform:subscription_plan:read"
  "api-platform:subscription_plan:manage"
  # REST APIs
  "api-platform:rest_api:read"
  "api-platform:rest_api:create"
  "api-platform:rest_api:manage"
  "api-platform:rest_api:publish"
  "api-platform:rest_api:deployment:manage"
  # Git
  "api-platform:git:read"
)

# ── Role → scope sets (mirrors ROLE_SCOPES in permissions.ts) ─────────────────
ADMIN_SCOPES=(
  "api-platform:project:manage"
  "api-platform:application:manage"
  "api-platform:application:api_key:manage"
  "api-platform:application:associations:manage"
  "api-platform:gateway:manage"
  "api-platform:gateway:token:manage"
  "api-platform:gateway:policy:manage"
  "api-platform:llm_provider:manage"
  "api-platform:llm_provider:key:manage"
  "api-platform:llm_provider:deployment:manage"
  "api-platform:llm_proxy:manage"
  "api-platform:llm_proxy:key:manage"
  "api-platform:llm_proxy:deployment:manage"
  "api-platform:llm_template:manage"
  "api-platform:mcp_proxy:manage"
  "api-platform:mcp_proxy:deployment:manage"
  "api-platform:devportal:manage"
  "api-platform:subscription:manage"
  "api-platform:subscription_plan:manage"
  "api-platform:rest_api:manage"
  "api-platform:rest_api:deployment:manage"
  "api-platform:git:read"
)

DEVELOPER_SCOPES=(
  "api-platform:project:manage"
  "api-platform:application:manage"
  "api-platform:application:api_key:manage"
  "api-platform:application:associations:manage"
  "api-platform:llm_provider:manage"
  "api-platform:llm_provider:key:manage"
  "api-platform:llm_provider:deployment:manage"
  "api-platform:llm_proxy:manage"
  "api-platform:llm_proxy:key:manage"
  "api-platform:llm_proxy:deployment:manage"
  "api-platform:llm_template:manage"
  "api-platform:mcp_proxy:manage"
  "api-platform:mcp_proxy:deployment:manage"
  "api-platform:rest_api:manage"
  "api-platform:rest_api:deployment:manage"
  "api-platform:git:read"
)

PUBLISHER_SCOPES=(
  "api-platform:project:read"
  "api-platform:llm_provider:read"
  "api-platform:llm_proxy:read"
  "api-platform:mcp_proxy:read"
  "api-platform:application:read"
  "api-platform:rest_api:read"
  "api-platform:rest_api:publish"
  "api-platform:devportal:manage"
  "api-platform:subscription:read"
)

OPERATOR_SCOPES=(
  "api-platform:project:read"
  "api-platform:gateway:manage"
  "api-platform:gateway:token:manage"
  "api-platform:gateway:policy:read"
  "api-platform:llm_provider:read"
  "api-platform:llm_provider:deployment:manage"
  "api-platform:llm_proxy:read"
  "api-platform:llm_proxy:deployment:manage"
  "api-platform:mcp_proxy:read"
  "api-platform:mcp_proxy:deployment:manage"
  "api-platform:rest_api:read"
  "api-platform:rest_api:deployment:manage"
)

VIEWER_SCOPES=(
  "api-platform:project:read"
  "api-platform:application:read"
  "api-platform:application:api_key:read"
  "api-platform:application:associations:read"
  "api-platform:gateway:read"
  "api-platform:gateway:token:read"
  "api-platform:gateway:policy:read"
  "api-platform:llm_provider:read"
  "api-platform:llm_proxy:read"
  "api-platform:llm_template:read"
  "api-platform:mcp_proxy:read"
  "api-platform:devportal:read"
  "api-platform:subscription:read"
  "api-platform:subscription_plan:read"
  "api-platform:rest_api:read"
)

# ── Step 1: Add all scopes to the API resource ────────────────────────────────
echo "▶ Adding ${#ALL_SCOPES[@]} scopes to API resource ${RESOURCE_ID} ..."

SCOPES_BODY=$(scopes_json ALL_SCOPES)
PATCH_BODY="{\"addedScopes\":${SCOPES_BODY}}"

"${CURL[@]}" -X PATCH \
  "${BASE}/api/server/v1/api-resources/${RESOURCE_ID}" \
  -d "$PATCH_BODY" \
  | grep -q "" && echo "  ✓ Scopes added" || echo "  ✓ Scopes added (or already exist)"

# ── Step 2: Assign scopes to each role ───────────────────────────────────────
declare -A ROLE_SCOPE_MAP=(
  [admin]="ADMIN_SCOPES"
  [developer]="DEVELOPER_SCOPES"
  [publisher]="PUBLISHER_SCOPES"
  [operator]="OPERATOR_SCOPES"
  [viewer]="VIEWER_SCOPES"
)

for ROLE_NAME in admin developer publisher operator viewer; do
  echo "▶ Looking up role: ${ROLE_NAME} ..."
  ROLE_ID=$(get_role_id "$ROLE_NAME")

  if [[ -z "$ROLE_ID" ]]; then
    echo "  ⚠ Role '${ROLE_NAME}' not found — skipping (create it in Asgardeo first)"
    continue
  fi

  echo "  ID: ${ROLE_ID}"
  SCOPE_VAR="${ROLE_SCOPE_MAP[$ROLE_NAME]}"
  PATCH=$( permissions_patch "$SCOPE_VAR" )

  "${CURL[@]}" -X PATCH \
    "${BASE}/scim2/v2/Roles/${ROLE_ID}" \
    -d "$PATCH" \
    | grep -q "" && echo "  ✓ Permissions assigned to '${ROLE_NAME}'"
done

echo ""
echo "Done. Reload the app and log in — scopes will appear in the access token."
