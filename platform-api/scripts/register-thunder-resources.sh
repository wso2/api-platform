#!/bin/bash
# Registers the API Platform resource server and all permissions in Thunder.
# Runs against the Thunder endpoint using system client credentials.
#
# ─────────────────────────────────────────────────────────────────────────────
# PREREQUISITES — complete these steps in Thunder before running this script
# ─────────────────────────────────────────────────────────────────────────────
#
# Step 1 — Create a system application in the Thunder console
#   a. Log in to the Thunder admin console.
#   b. Navigate to Applications → Create Application.
#   c. Create a Machine-to-Machine (client credentials) application.
#   d. Note the generated Client ID and Client Secret — these become the
#      THUNDER_CLIENT_ID and THUNDER_CLIENT_SECRET env vars below.
#   e. Go to Roles → Administrator and assign this application to that role.
#      (The system token fetch in this script requires administrator-level access.)
#
# Step 2 — Create roles in the API Platform console
#   After this script has registered the resource server and permissions,
#   create the following roles in the API Platform admin console and assign
#   the Thunder application (from Step 1) to each role so that it can obtain
#   scoped tokens on behalf of platform operations:
#     • admin     — full platform access
#     • developer — create/manage APIs, deployments, and subscriptions
#     • viewer    — read-only access across all resources
#   Assign the Thunder application created in Step 1 to each of these roles.
#
# ─────────────────────────────────────────────────────────────────────────────
#
# Usage:
#   ./register-thunder-resources.sh
#   THUNDER_URL=https://localhost:8090 ./register-thunder-resources.sh
#
# Environment variables:
#   THUNDER_URL           Thunder base URL (default: https://localhost:8090)
#   THUNDER_CLIENT_ID     OAuth2 client ID (default: api-platform-system-client)
#   THUNDER_CLIENT_SECRET OAuth2 client secret
#   RS_IDENTIFIER         Resource server identifier / audience (default: https://localhost:9243)
#   ADMIN_ROLE_ID         Role ID to assign admin permissions (prompted if unset)
#   DEVELOPER_ROLE_ID     Role ID to assign developer permissions (prompted if unset)
#   VIEWER_ROLE_ID        Role ID to assign viewer permissions (prompted if unset)

set -e

THUNDER_URL="${THUNDER_URL:-https://localhost:8090}"
CLIENT_ID="${THUNDER_CLIENT_ID:-api-platform-system-client}"
CLIENT_SECRET="${THUNDER_CLIENT_SECRET:-api-platform-system-client-secret}"
RS_IDENTIFIER="${RS_IDENTIFIER:-https://localhost:9243}"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
log_info()    { echo -e "${BLUE}[INFO]${NC} $1" >&2; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} ✓ $1" >&2; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} ⚠ $1" >&2; }
log_error()   { echo -e "${RED}[ERROR]${NC} ✗ $1" >&2; }

# ---------------------------------------------------------------------------
# Get a system-scoped token
# ---------------------------------------------------------------------------
log_info "Obtaining system token from $THUNDER_URL ..."
TOKEN_RESPONSE=$(curl -sk -X POST "$THUNDER_URL/oauth2/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -u "$CLIENT_ID:$CLIENT_SECRET" \
  -d "grant_type=client_credentials&scope=system")

TOKEN=$(echo "$TOKEN_RESPONSE" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
if [[ -z "$TOKEN" ]]; then
  log_error "Failed to obtain system token. Response: $TOKEN_RESPONSE"
  exit 1
fi
log_success "System token obtained."

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
api_call() {
  local method="$1" endpoint="$2" data="${3:-}"
  if [[ -z "$data" ]]; then
    curl -sk -w "\n%{http_code}" -X "$method" "$THUNDER_URL$endpoint" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $TOKEN" 2>/dev/null || echo -e "\n000"
  else
    curl -sk -w "\n%{http_code}" -X "$method" "$THUNDER_URL$endpoint" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $TOKEN" \
      -d "$data" 2>/dev/null || echo -e "\n000"
  fi
}

create_or_get_rs() {
  local name="$1" handle="$2" identifier="$3" description="$4" ou_id="$5"
  local payload="{\"name\":\"${name}\",\"handle\":\"${handle}\",\"identifier\":\"${identifier}\",\"description\":\"${description}\",\"ouId\":\"${ou_id}\"}"
  local response http_code body id

  response=$(api_call POST "/resource-servers" "$payload")
  http_code="${response: -3}"; body="${response%???}"

  if [[ "$http_code" == "201" ]] || [[ "$http_code" == "200" ]]; then
    id=$(echo "$body" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    log_success "Resource server '${identifier}' created (id: $id)"
    echo "$id"; return 0
  fi

  if [[ "$http_code" == "409" ]]; then
    log_warning "Resource server '${identifier}' already exists, retrieving ID..."
    response=$(api_call GET "/resource-servers")
    http_code="${response: -3}"; body="${response%???}"
    [[ "$http_code" != "200" ]] && { log_error "Failed to list resource servers (HTTP $http_code)"; exit 1; }
    id=$(echo "$body" | sed 's/},{/}\n{/g' | grep "\"identifier\":\"${identifier}\"" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    [[ -z "$id" ]] && { log_error "Resource server '${identifier}' not found after 409"; exit 1; }
    log_success "Found existing resource server '${identifier}' (id: $id)"
    echo "$id"; return 0
  fi

  log_error "Failed to create resource server '${identifier}' (HTTP $http_code): $body"
  exit 1
}

create_or_get_resource() {
  local rs_id="$1" name="$2" handle="$3" description="$4" parent_id="${5:-}"
  local payload list_url response http_code body id

  if [[ -n "$parent_id" ]]; then
    payload="{\"name\":\"${name}\",\"handle\":\"${handle}\",\"description\":\"${description}\",\"parent\":\"${parent_id}\"}"
    list_url="/resource-servers/${rs_id}/resources?parentId=${parent_id}"
  else
    payload="{\"name\":\"${name}\",\"handle\":\"${handle}\",\"description\":\"${description}\"}"
    list_url="/resource-servers/${rs_id}/resources"
  fi

  response=$(api_call POST "/resource-servers/${rs_id}/resources" "$payload")
  http_code="${response: -3}"; body="${response%???}"

  if [[ "$http_code" == "201" ]] || [[ "$http_code" == "200" ]]; then
    id=$(echo "$body" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    echo "$id"; return 0
  fi

  if [[ "$http_code" == "409" ]]; then
    log_warning "Resource '${handle}' already exists, retrieving ID..."
    response=$(api_call GET "${list_url}")
    http_code="${response: -3}"; body="${response%???}"
    [[ "$http_code" != "200" ]] && { log_error "Failed to list resources (HTTP $http_code)"; exit 1; }
    id=$(echo "$body" | sed 's/},{/}\n{/g' | grep "\"handle\":\"${handle}\"" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
    [[ -z "$id" ]] && { log_error "Resource '${handle}' not found after 409"; exit 1; }
    log_success "Found existing resource '${handle}' (id: $id)"
    echo "$id"; return 0
  fi

  log_error "Failed to create resource '${handle}' (HTTP $http_code): $body"
  exit 1
}

create_action() {
  local rs_id="$1" res_id="$2" name="$3" handle="$4" description="$5"
  local payload="{\"name\":\"${name}\",\"handle\":\"${handle}\",\"description\":\"${description}\"}"
  local response http_code body

  response=$(api_call POST "/resource-servers/${rs_id}/resources/${res_id}/actions" "$payload")
  http_code="${response: -3}"; body="${response%???}"

  if [[ "$http_code" == "201" ]] || [[ "$http_code" == "200" ]] || [[ "$http_code" == "409" ]]; then
    return 0
  fi

  log_error "Failed to create action '${handle}' on resource ${res_id} (HTTP $http_code): $body"
  exit 1
}

# ===========================================================================
# 0. Fetch default OU ID
# ===========================================================================
log_info "Fetching default organization unit ID..."
OU_RESPONSE=$(api_call GET "/organization-units/tree/default")
OU_HTTP="${OU_RESPONSE: -3}"; OU_BODY="${OU_RESPONSE%???}"
if [[ "$OU_HTTP" != "200" ]]; then
  log_error "Failed to fetch default OU (HTTP $OU_HTTP): $OU_BODY"
  exit 1
fi
DEFAULT_OU_ID=$(echo "$OU_BODY" | grep -o '"handle":"default"[^}]*"id":"[^"]*"\|"id":"[^"]*"[^}]*"handle":"default"' | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
if [[ -z "$DEFAULT_OU_ID" ]]; then
  log_error "Could not extract default OU ID from response"
  exit 1
fi
log_success "Default OU ID: $DEFAULT_OU_ID"

# ===========================================================================
# 1. Register org user attributes, OIDC claims, and SCIM2 mappings
# ===========================================================================
# Two user attributes are required for multi-org support:
#
#   organization  (http://wso2.org/claims/organization)
#     The user's currently selected / active organization UUID.
#     Set to the first org on registration; updated per-session via SCIM2
#     when the user switches orgs.
#
#   organizations  (http://wso2.org/claims/organizations)
#     Space-separated list of ALL organization UUIDs the user belongs to.
#     Appended via SCIM2 each time the user creates or joins a new org.
#
# Each attribute needs three registrations:
#   a. Local claim  — the WSO2 claim URI (http://wso2.org/claims/...)
#   b. OIDC claim   — the JWT claim name (organization / organizations)
#   c. SCIM2 mapping — urn:scim:schemas:extension:custom:User:<attr>
# ===========================================================================

register_user_attribute() {
  local claim_uri="$1"      # e.g. http://wso2.org/claims/organization
  local oidc_name="$2"      # e.g. organization
  local display_name="$3"   # e.g. Organization
  local description="$4"

  log_info "Registering user attribute: $oidc_name ..."

  # ── a. Local claim ──────────────────────────────────────────────────────────
  local local_payload
  local_payload=$(cat <<EOF
{
  "claimURI": "${claim_uri}",
  "displayName": "${display_name}",
  "description": "${description}",
  "attributeName": "${oidc_name}",
  "readOnly": false,
  "required": false,
  "supportedByDefault": true
}
EOF
)
  local resp http_code body
  resp=$(api_call POST "/claim-dialects/local/claims" "$local_payload")
  http_code="${resp: -3}"; body="${resp%???}"
  if [[ "$http_code" == "201" ]] || [[ "$http_code" == "409" ]]; then
    log_success "Local claim ${claim_uri} ready"
  else
    log_warning "Local claim ${claim_uri} — HTTP ${http_code}: ${body}"
  fi

  # ── b. OIDC claim mapping ───────────────────────────────────────────────────
  local oidc_payload
  oidc_payload=$(cat <<EOF
{
  "claimURI": "${claim_uri}",
  "oidcClaimURI": "${oidc_name}"
}
EOF
)
  resp=$(api_call POST "/claim-dialects/oidc/claims" "$oidc_payload")
  http_code="${resp: -3}"; body="${resp%???}"
  if [[ "$http_code" == "201" ]] || [[ "$http_code" == "409" ]]; then
    log_success "OIDC claim ${oidc_name} → ${claim_uri} ready"
  else
    log_warning "OIDC claim ${oidc_name} — HTTP ${http_code}: ${body}"
  fi

  # ── c. SCIM2 custom-user extension mapping ──────────────────────────────────
  local scim_attr="urn:scim:schemas:extension:custom:User:${oidc_name}"
  local scim_payload
  scim_payload=$(cat <<EOF
{
  "claimURI": "${claim_uri}",
  "scimClaimURI": "${scim_attr}"
}
EOF
)
  resp=$(api_call POST "/claim-dialects/scim2/claims" "$scim_payload")
  http_code="${resp: -3}"; body="${resp%???}"
  if [[ "$http_code" == "201" ]] || [[ "$http_code" == "409" ]]; then
    log_success "SCIM2 mapping ${scim_attr} ready"
  else
    log_warning "SCIM2 mapping ${scim_attr} — HTTP ${http_code}: ${body}"
  fi
}

register_user_attribute \
  "http://wso2.org/claims/organization" \
  "organization" \
  "Organization" \
  "The user's currently active organization UUID (set at login, updated per org-switch)"

register_user_attribute \
  "http://wso2.org/claims/organizations" \
  "organizations" \
  "Organizations" \
  "Space-separated list of all organization UUIDs the user belongs to"

# ===========================================================================
# 3. Resource server
# ===========================================================================
log_info "Creating 'api-platform' resource server..."
RS_ID=$(create_or_get_rs "API Platform" "api-platform" "$RS_IDENTIFIER" "WSO2 API Platform permissions" "$DEFAULT_OU_ID")
log_info "Resource server ready (id: $RS_ID)"

# ===========================================================================
# 4. Level-1 resources
# ===========================================================================
log_info "Creating level-1 resources..."
R_ORG=$(create_or_get_resource          "$RS_ID" "Organization"            "org"               "Organization management")
R_PROJECT=$(create_or_get_resource      "$RS_ID" "Project"                 "project"           "Project management")
R_API=$(create_or_get_resource          "$RS_ID" "REST API"                "rest_api"          "REST API management")
R_APP=$(create_or_get_resource          "$RS_ID" "Application"             "application"       "Application management")
R_GW=$(create_or_get_resource           "$RS_ID" "Gateway"                 "gateway"           "Gateway management")
R_DP=$(create_or_get_resource           "$RS_ID" "DevPortal"               "devportal"         "Developer portal management")
R_GIT=$(create_or_get_resource          "$RS_ID" "Git"                     "git"               "Git repository access")
R_LLMT=$(create_or_get_resource         "$RS_ID" "LLM Provider Template"   "llm_template"      "LLM provider template management")
R_LLM=$(create_or_get_resource          "$RS_ID" "LLM Provider"            "llm_provider"      "LLM provider management")
R_PROXY=$(create_or_get_resource        "$RS_ID" "LLM Proxy"               "llm_proxy"         "LLM proxy management")
R_MCP=$(create_or_get_resource          "$RS_ID" "MCP Proxy"               "mcp_proxy"         "MCP proxy management")
R_WEBSUB=$(create_or_get_resource       "$RS_ID" "WebSub API"              "websub_api"        "WebSub API management")
R_WEBBROKER=$(create_or_get_resource    "$RS_ID" "WebBroker API"           "webbroker_api"     "WebBroker API management")
R_SUB=$(create_or_get_resource          "$RS_ID" "Subscription"            "subscription"      "API subscription management")
R_SUBPLAN=$(create_or_get_resource      "$RS_ID" "Subscription Plan"       "subscription_plan" "Subscription plan management")
R_POLICY=$(create_or_get_resource       "$RS_ID" "Custom Policy"           "custom_policy"     "Custom policy management")
log_info "Level-1 resources created."

# ===========================================================================
# 5. Level-2 sub-resources
# ===========================================================================
log_info "Creating level-2 sub-resources..."
# REST API sub-resources
R_API_DEPLOY=$(create_or_get_resource   "$RS_ID" "REST API Deployment"     "deployment" "REST API deployment management"        "$R_API")
R_API_GW=$(create_or_get_resource       "$RS_ID" "REST API Gateway"        "gateway"    "REST API gateway association"          "$R_API")
R_API_KEY=$(create_or_get_resource      "$RS_ID" "REST API Key"            "api_key"    "REST API-level API key management"     "$R_API")

# Application sub-resources
R_APP_KEY=$(create_or_get_resource      "$RS_ID" "Application API Key"     "api_key"    "Application API key management"        "$R_APP")

# Gateway sub-resources
R_GW_TOKEN=$(create_or_get_resource     "$RS_ID" "Gateway Token"           "token"      "Gateway token management"              "$R_GW")
R_GW_POLICY=$(create_or_get_resource    "$RS_ID" "Gateway Policy"          "policy"     "Gateway policy management"             "$R_GW")

# LLM Provider sub-resources
R_LLM_DEPLOY=$(create_or_get_resource   "$RS_ID" "LLM Provider Deployment" "deployment" "LLM provider deployment management"    "$R_LLM")
R_LLM_KEY=$(create_or_get_resource      "$RS_ID" "LLM Provider API Key"    "api_key"    "LLM provider API key management"       "$R_LLM")

# LLM Proxy sub-resources
R_PROXY_DEPLOY=$(create_or_get_resource "$RS_ID" "LLM Proxy Deployment"    "deployment" "LLM proxy deployment management"       "$R_PROXY")
R_PROXY_KEY=$(create_or_get_resource    "$RS_ID" "LLM Proxy API Key"       "api_key"    "LLM proxy API key management"          "$R_PROXY")

# MCP Proxy sub-resources
R_MCP_DEPLOY=$(create_or_get_resource   "$RS_ID" "MCP Proxy Deployment"    "deployment" "MCP proxy deployment management"       "$R_MCP")

# WebSub API sub-resources
R_WEBSUB_DEPLOY=$(create_or_get_resource "$RS_ID" "WebSub API Deployment"  "deployment" "WebSub API deployment management"      "$R_WEBSUB")
R_WEBSUB_KEY=$(create_or_get_resource    "$RS_ID" "WebSub API Key"         "api_key"    "WebSub API key management"             "$R_WEBSUB")

# WebBroker API sub-resources
R_WEBBROKER_DEPLOY=$(create_or_get_resource "$RS_ID" "WebBroker API Deployment" "deployment" "WebBroker API deployment management" "$R_WEBBROKER")
R_WEBBROKER_KEY=$(create_or_get_resource    "$RS_ID" "WebBroker API Key"        "api_key"    "WebBroker API key management"        "$R_WEBBROKER")

log_info "Level-2 sub-resources created."

# ===========================================================================
# 6. Actions
# ===========================================================================
log_info "Creating actions..."

# Organization (admin-only for write; read implied by token presence)
create_action "$RS_ID" "$R_ORG"          "Create"   "create"   "Create an organization"
create_action "$RS_ID" "$R_ORG"          "Read"     "read"     "View organization details"
create_action "$RS_ID" "$R_ORG"          "Update"   "update"   "Update organization settings"
create_action "$RS_ID" "$R_ORG"          "Delete"   "delete"   "Delete an organization"

# Project (admin + developer)
create_action "$RS_ID" "$R_PROJECT"      "Create"   "create"   "Create a project"
create_action "$RS_ID" "$R_PROJECT"      "Read"     "read"     "View project details"
create_action "$RS_ID" "$R_PROJECT"      "Update"   "update"   "Update a project"
create_action "$RS_ID" "$R_PROJECT"      "Delete"   "delete"   "Delete a project"

# REST API (admin + developer)
create_action "$RS_ID" "$R_API"          "Create"   "create"   "Create a REST API"
create_action "$RS_ID" "$R_API"          "Read"     "read"     "View REST API details"
create_action "$RS_ID" "$R_API"          "Update"   "update"   "Update a REST API"
create_action "$RS_ID" "$R_API"          "Delete"   "delete"   "Delete a REST API"
create_action "$RS_ID" "$R_API"          "Import"   "import"   "Import a REST API from OpenAPI spec or project"
create_action "$RS_ID" "$R_API"          "Publish"  "publish"  "Publish a REST API to a developer portal"

# REST API → Deployment sub-resource (admin + developer + operator)
create_action "$RS_ID" "$R_API_DEPLOY"   "Create"   "create"   "Deploy a REST API to a gateway"
create_action "$RS_ID" "$R_API_DEPLOY"   "Read"     "read"     "View REST API deployments"
create_action "$RS_ID" "$R_API_DEPLOY"   "Delete"   "delete"   "Delete a REST API deployment record"
create_action "$RS_ID" "$R_API_DEPLOY"   "Undeploy" "undeploy" "Undeploy a REST API from a gateway"
create_action "$RS_ID" "$R_API_DEPLOY"   "Restore"  "restore"  "Restore a previous REST API deployment"

# REST API → Gateway association sub-resource (admin + developer + operator)
create_action "$RS_ID" "$R_API_GW"       "Create"   "create"   "Associate a REST API with a gateway"
create_action "$RS_ID" "$R_API_GW"       "Read"     "read"     "View REST API gateway associations"

# REST API → API Key sub-resource (admin + developer)
create_action "$RS_ID" "$R_API_KEY"      "Create"   "create"   "Create a REST API-level API key"
create_action "$RS_ID" "$R_API_KEY"      "Read"     "read"     "List REST API-level API keys"
create_action "$RS_ID" "$R_API_KEY"      "Update"   "update"   "Update a REST API-level API key"
create_action "$RS_ID" "$R_API_KEY"      "Delete"   "delete"   "Delete a REST API-level API key"

# Application (admin + developer)
create_action "$RS_ID" "$R_APP"          "Create"   "create"   "Create an application"
create_action "$RS_ID" "$R_APP"          "Read"     "read"     "View application details"
create_action "$RS_ID" "$R_APP"          "Update"   "update"   "Update an application"
create_action "$RS_ID" "$R_APP"          "Delete"   "delete"   "Delete an application"

# Application → API Key sub-resource (admin + developer)
create_action "$RS_ID" "$R_APP_KEY"      "Create"   "create"   "Create an application API key"
create_action "$RS_ID" "$R_APP_KEY"      "Read"     "read"     "List application API keys"
create_action "$RS_ID" "$R_APP_KEY"      "Delete"   "delete"   "Delete an application API key"

# Gateway (admin-only for write)
create_action "$RS_ID" "$R_GW"           "Create"   "create"   "Register a gateway"
create_action "$RS_ID" "$R_GW"           "Read"     "read"     "View gateway details and status"
create_action "$RS_ID" "$R_GW"           "Update"   "update"   "Update gateway configuration"
create_action "$RS_ID" "$R_GW"           "Delete"   "delete"   "Delete a gateway"

# Gateway → Token sub-resource (admin-only)
create_action "$RS_ID" "$R_GW_TOKEN"     "Create"   "create"   "Issue a gateway token"
create_action "$RS_ID" "$R_GW_TOKEN"     "Read"     "read"     "List gateway tokens"
create_action "$RS_ID" "$R_GW_TOKEN"     "Delete"   "delete"   "Revoke a gateway token"

# Gateway → Policy sub-resource (admin-only)
create_action "$RS_ID" "$R_GW_POLICY"    "Create"   "create"   "Upload a gateway policy"
create_action "$RS_ID" "$R_GW_POLICY"    "Read"     "read"     "View gateway policies"
create_action "$RS_ID" "$R_GW_POLICY"    "Delete"   "delete"   "Delete a gateway policy"

# DevPortal (admin-only)
create_action "$RS_ID" "$R_DP"           "Create"   "create"   "Create a developer portal"
create_action "$RS_ID" "$R_DP"           "Read"     "read"     "View developer portal details"
create_action "$RS_ID" "$R_DP"           "Update"   "update"   "Update developer portal settings (including activate, deactivate, set-default)"
create_action "$RS_ID" "$R_DP"           "Delete"   "delete"   "Delete a developer portal"

# Git (admin + developer)
create_action "$RS_ID" "$R_GIT"          "Read"     "read"     "Browse repository branches and fetch content"

# LLM Provider Template (admin + developer)
create_action "$RS_ID" "$R_LLMT"         "Create"   "create"   "Create an LLM provider template"
create_action "$RS_ID" "$R_LLMT"         "Read"     "read"     "View LLM provider templates"
create_action "$RS_ID" "$R_LLMT"         "Update"   "update"   "Update an LLM provider template"
create_action "$RS_ID" "$R_LLMT"         "Delete"   "delete"   "Delete an LLM provider template"

# LLM Provider (admin + developer)
create_action "$RS_ID" "$R_LLM"          "Create"   "create"   "Create an LLM provider"
create_action "$RS_ID" "$R_LLM"          "Read"     "read"     "View LLM providers"
create_action "$RS_ID" "$R_LLM"          "Update"   "update"   "Update an LLM provider"
create_action "$RS_ID" "$R_LLM"          "Delete"   "delete"   "Delete an LLM provider"

# LLM Provider → Deployment sub-resource (admin + developer + operator)
create_action "$RS_ID" "$R_LLM_DEPLOY"   "Create"   "create"   "Deploy an LLM provider to a gateway"
create_action "$RS_ID" "$R_LLM_DEPLOY"   "Read"     "read"     "View LLM provider deployments"
create_action "$RS_ID" "$R_LLM_DEPLOY"   "Delete"   "delete"   "Delete an LLM provider deployment record"
create_action "$RS_ID" "$R_LLM_DEPLOY"   "Undeploy" "undeploy" "Undeploy an LLM provider from a gateway"
create_action "$RS_ID" "$R_LLM_DEPLOY"   "Restore"  "restore"  "Restore a previous LLM provider deployment"

# LLM Provider → API Key sub-resource (admin + developer; explicit opt-in, not covered by llm_provider:manage)
create_action "$RS_ID" "$R_LLM_KEY"      "Create"   "create"   "Create an LLM provider API key"
create_action "$RS_ID" "$R_LLM_KEY"      "Read"     "read"     "List LLM provider API keys"
create_action "$RS_ID" "$R_LLM_KEY"      "Delete"   "delete"   "Delete an LLM provider API key"

# LLM Proxy (admin + developer)
create_action "$RS_ID" "$R_PROXY"        "Create"   "create"   "Create an LLM proxy"
create_action "$RS_ID" "$R_PROXY"        "Read"     "read"     "View LLM proxies"
create_action "$RS_ID" "$R_PROXY"        "Update"   "update"   "Update an LLM proxy"
create_action "$RS_ID" "$R_PROXY"        "Delete"   "delete"   "Delete an LLM proxy"

# LLM Proxy → Deployment sub-resource (admin + developer + operator)
create_action "$RS_ID" "$R_PROXY_DEPLOY" "Create"   "create"   "Deploy an LLM proxy to a gateway"
create_action "$RS_ID" "$R_PROXY_DEPLOY" "Read"     "read"     "View LLM proxy deployments"
create_action "$RS_ID" "$R_PROXY_DEPLOY" "Delete"   "delete"   "Delete an LLM proxy deployment record"
create_action "$RS_ID" "$R_PROXY_DEPLOY" "Undeploy" "undeploy" "Undeploy an LLM proxy from a gateway"
create_action "$RS_ID" "$R_PROXY_DEPLOY" "Restore"  "restore"  "Restore a previous LLM proxy deployment"

# LLM Proxy → API Key sub-resource (admin + developer; explicit opt-in)
create_action "$RS_ID" "$R_PROXY_KEY"    "Create"   "create"   "Create an LLM proxy API key"
create_action "$RS_ID" "$R_PROXY_KEY"    "Read"     "read"     "List LLM proxy API keys"
create_action "$RS_ID" "$R_PROXY_KEY"    "Delete"   "delete"   "Delete an LLM proxy API key"

# MCP Proxy (admin + developer)
create_action "$RS_ID" "$R_MCP"          "Create"   "create"   "Create an MCP proxy"
create_action "$RS_ID" "$R_MCP"          "Read"     "read"     "View MCP proxies"
create_action "$RS_ID" "$R_MCP"          "Update"   "update"   "Update an MCP proxy"
create_action "$RS_ID" "$R_MCP"          "Delete"   "delete"   "Delete an MCP proxy"

# MCP Proxy → Deployment sub-resource (admin + developer + operator)
create_action "$RS_ID" "$R_MCP_DEPLOY"   "Create"   "create"   "Deploy an MCP proxy to a gateway"
create_action "$RS_ID" "$R_MCP_DEPLOY"   "Read"     "read"     "View MCP proxy deployments"
create_action "$RS_ID" "$R_MCP_DEPLOY"   "Delete"   "delete"   "Delete an MCP proxy deployment record"
create_action "$RS_ID" "$R_MCP_DEPLOY"   "Undeploy" "undeploy" "Undeploy an MCP proxy from a gateway"
create_action "$RS_ID" "$R_MCP_DEPLOY"   "Restore"  "restore"  "Restore a previous MCP proxy deployment"

# WebSub API (admin + developer)
create_action "$RS_ID" "$R_WEBSUB"       "Create"   "create"   "Create a WebSub API"
create_action "$RS_ID" "$R_WEBSUB"       "Read"     "read"     "View WebSub APIs"
create_action "$RS_ID" "$R_WEBSUB"       "Update"   "update"   "Update a WebSub API"
create_action "$RS_ID" "$R_WEBSUB"       "Delete"   "delete"   "Delete a WebSub API"
create_action "$RS_ID" "$R_WEBSUB"       "Publish"  "publish"  "Publish a WebSub API to a developer portal"

# WebSub API → Deployment sub-resource (admin + developer + operator)
create_action "$RS_ID" "$R_WEBSUB_DEPLOY" "Create"   "create"   "Deploy a WebSub API to a gateway"
create_action "$RS_ID" "$R_WEBSUB_DEPLOY" "Read"     "read"     "View WebSub API deployments"
create_action "$RS_ID" "$R_WEBSUB_DEPLOY" "Delete"   "delete"   "Delete a WebSub API deployment record"
create_action "$RS_ID" "$R_WEBSUB_DEPLOY" "Undeploy" "undeploy" "Undeploy a WebSub API from a gateway"
create_action "$RS_ID" "$R_WEBSUB_DEPLOY" "Restore"  "restore"  "Restore a previous WebSub API deployment"

# WebSub API → API Key sub-resource (admin + developer; explicit opt-in)
create_action "$RS_ID" "$R_WEBSUB_KEY"   "Create"   "create"   "Create a WebSub API key"
create_action "$RS_ID" "$R_WEBSUB_KEY"   "Update"   "update"   "Update a WebSub API key"
create_action "$RS_ID" "$R_WEBSUB_KEY"   "Delete"   "delete"   "Delete a WebSub API key"

# WebBroker API (admin + developer)
create_action "$RS_ID" "$R_WEBBROKER"    "Create"   "create"   "Create a WebBroker API"
create_action "$RS_ID" "$R_WEBBROKER"    "Read"     "read"     "View WebBroker APIs"
create_action "$RS_ID" "$R_WEBBROKER"    "Update"   "update"   "Update a WebBroker API"
create_action "$RS_ID" "$R_WEBBROKER"    "Delete"   "delete"   "Delete a WebBroker API"
create_action "$RS_ID" "$R_WEBBROKER"    "Publish"  "publish"  "Publish a WebBroker API to a developer portal"

# WebBroker API → Deployment sub-resource (admin + developer + operator)
create_action "$RS_ID" "$R_WEBBROKER_DEPLOY" "Create"   "create"   "Deploy a WebBroker API to a gateway"
create_action "$RS_ID" "$R_WEBBROKER_DEPLOY" "Read"     "read"     "View WebBroker API deployments"
create_action "$RS_ID" "$R_WEBBROKER_DEPLOY" "Delete"   "delete"   "Delete a WebBroker API deployment record"
create_action "$RS_ID" "$R_WEBBROKER_DEPLOY" "Undeploy" "undeploy" "Undeploy a WebBroker API from a gateway"
create_action "$RS_ID" "$R_WEBBROKER_DEPLOY" "Restore"  "restore"  "Restore a previous WebBroker API deployment"

# WebBroker API → API Key sub-resource (admin + developer; explicit opt-in)
create_action "$RS_ID" "$R_WEBBROKER_KEY" "Create"  "create"   "Create a WebBroker API key"
create_action "$RS_ID" "$R_WEBBROKER_KEY" "Update"  "update"   "Update a WebBroker API key"
create_action "$RS_ID" "$R_WEBBROKER_KEY" "Delete"  "delete"   "Delete a WebBroker API key"

# Subscription (admin + developer)
create_action "$RS_ID" "$R_SUB"          "Create"   "create"   "Create a subscription"
create_action "$RS_ID" "$R_SUB"          "Read"     "read"     "View subscriptions"
create_action "$RS_ID" "$R_SUB"          "Update"   "update"   "Update a subscription"
create_action "$RS_ID" "$R_SUB"          "Delete"   "delete"   "Delete a subscription"

# Subscription Plan (admin-only)
create_action "$RS_ID" "$R_SUBPLAN"      "Create"   "create"   "Create a subscription plan"
create_action "$RS_ID" "$R_SUBPLAN"      "Read"     "read"     "View subscription plans"
create_action "$RS_ID" "$R_SUBPLAN"      "Update"   "update"   "Update a subscription plan"
create_action "$RS_ID" "$R_SUBPLAN"      "Delete"   "delete"   "Delete a subscription plan"

# Custom Policy (admin-only)
create_action "$RS_ID" "$R_POLICY"       "Read"     "read"     "View custom policies"
create_action "$RS_ID" "$R_POLICY"       "Sync"     "sync"     "Sync a custom policy to gateways"
create_action "$RS_ID" "$R_POLICY"       "Delete"   "delete"   "Delete a custom policy"

log_success "All actions registered."

# ===========================================================================
# 7. Assign permissions to roles
# ===========================================================================
log_info "Assigning permissions to roles..."
log_info "─────────────────────────────────────────────────────────────────────────────"
log_info "ACTION REQUIRED — API Platform console setup"
log_info ""
log_info "Before providing role IDs, ensure you have completed the following in the"
log_info "API Platform admin console:"
log_info ""
log_info "  1. Create an 'admin' role and assign the Thunder system application to it."
log_info "  2. Create a 'developer' role and assign the Thunder system application to it."
log_info "  3. Create a 'viewer' role and assign the Thunder system application to it."
log_info ""
log_info "Once created, copy the role IDs from the console and provide them below."
log_info "─────────────────────────────────────────────────────────────────────────────"
log_info "Provide role IDs via env vars (ADMIN_ROLE_ID, DEVELOPER_ROLE_ID, VIEWER_ROLE_ID)"
log_info "or enter them interactively below. Leave blank to skip a role."

prompt_role_id() {
  local env_var="$1" role_label="$2"
  local val="${!env_var}"
  if [[ -z "$val" ]]; then
    read -r -p "  Role ID for '${role_label}' (blank to skip): " val
  fi
  echo "$val"
}

assign_role_permissions() {
  local role_id="$1" role_name="$2" permissions_json="$3"
  [[ -z "$role_id" ]] && { log_warning "Skipping role '${role_name}' — no ID provided."; return 0; }

  local payload="{\"name\":\"${role_name}\",\"ouId\":\"${DEFAULT_OU_ID}\",\"permissions\":[{\"resourceServerId\":\"${RS_ID}\",\"permissions\":${permissions_json}}]}"
  local response http_code body

  response=$(api_call PUT "/roles/${role_id}" "$payload")
  http_code="${response: -3}"; body="${response%???}"

  if [[ "$http_code" == "200" ]]; then
    log_success "Permissions assigned to role '${role_name}' (id: ${role_id})"
    return 0
  fi

  log_error "Failed to assign permissions to role '${role_name}' (HTTP $http_code): $body"
  return 1
}

ADMIN_ROLE_ID=$(prompt_role_id ADMIN_ROLE_ID "admin")
DEVELOPER_ROLE_ID=$(prompt_role_id DEVELOPER_ROLE_ID "developer")
VIEWER_ROLE_ID=$(prompt_role_id VIEWER_ROLE_ID "viewer")

# Admin: full access to everything
ADMIN_PERMISSIONS='[
  "api-platform:org:create","api-platform:org:read","api-platform:org:update","api-platform:org:delete",
  "api-platform:project:create","api-platform:project:read","api-platform:project:update","api-platform:project:delete",
  "api-platform:rest_api:create","api-platform:rest_api:read","api-platform:rest_api:update","api-platform:rest_api:delete",
  "api-platform:rest_api:import","api-platform:rest_api:publish",
  "api-platform:rest_api:deployment:create","api-platform:rest_api:deployment:read","api-platform:rest_api:deployment:delete",
  "api-platform:rest_api:deployment:undeploy","api-platform:rest_api:deployment:restore",
  "api-platform:rest_api:gateway:create","api-platform:rest_api:gateway:read",
  "api-platform:rest_api:api_key:create","api-platform:rest_api:api_key:read","api-platform:rest_api:api_key:update","api-platform:rest_api:api_key:delete",
  "api-platform:application:create","api-platform:application:read","api-platform:application:update","api-platform:application:delete",
  "api-platform:application:api_key:create","api-platform:application:api_key:read","api-platform:application:api_key:delete",
  "api-platform:gateway:create","api-platform:gateway:read","api-platform:gateway:update","api-platform:gateway:delete",
  "api-platform:gateway:token:create","api-platform:gateway:token:read","api-platform:gateway:token:delete",
  "api-platform:gateway:policy:create","api-platform:gateway:policy:read","api-platform:gateway:policy:delete",
  "api-platform:devportal:create","api-platform:devportal:read","api-platform:devportal:update","api-platform:devportal:delete",
  "api-platform:git:read",
  "api-platform:llm_template:create","api-platform:llm_template:read","api-platform:llm_template:update","api-platform:llm_template:delete",
  "api-platform:llm_provider:create","api-platform:llm_provider:read","api-platform:llm_provider:update","api-platform:llm_provider:delete",
  "api-platform:llm_provider:deployment:create","api-platform:llm_provider:deployment:read","api-platform:llm_provider:deployment:delete",
  "api-platform:llm_provider:deployment:undeploy","api-platform:llm_provider:deployment:restore",
  "api-platform:llm_provider:api_key:create","api-platform:llm_provider:api_key:read","api-platform:llm_provider:api_key:delete",
  "api-platform:llm_proxy:create","api-platform:llm_proxy:read","api-platform:llm_proxy:update","api-platform:llm_proxy:delete",
  "api-platform:llm_proxy:deployment:create","api-platform:llm_proxy:deployment:read","api-platform:llm_proxy:deployment:delete",
  "api-platform:llm_proxy:deployment:undeploy","api-platform:llm_proxy:deployment:restore",
  "api-platform:llm_proxy:api_key:create","api-platform:llm_proxy:api_key:read","api-platform:llm_proxy:api_key:delete",
  "api-platform:mcp_proxy:create","api-platform:mcp_proxy:read","api-platform:mcp_proxy:update","api-platform:mcp_proxy:delete",
  "api-platform:mcp_proxy:deployment:create","api-platform:mcp_proxy:deployment:read","api-platform:mcp_proxy:deployment:delete",
  "api-platform:mcp_proxy:deployment:undeploy","api-platform:mcp_proxy:deployment:restore",
  "api-platform:websub_api:create","api-platform:websub_api:read","api-platform:websub_api:update","api-platform:websub_api:delete",
  "api-platform:websub_api:publish",
  "api-platform:websub_api:deployment:create","api-platform:websub_api:deployment:read","api-platform:websub_api:deployment:delete",
  "api-platform:websub_api:deployment:undeploy","api-platform:websub_api:deployment:restore",
  "api-platform:websub_api:api_key:create","api-platform:websub_api:api_key:update","api-platform:websub_api:api_key:delete",
  "api-platform:webbroker_api:create","api-platform:webbroker_api:read","api-platform:webbroker_api:update","api-platform:webbroker_api:delete",
  "api-platform:webbroker_api:publish",
  "api-platform:webbroker_api:deployment:create","api-platform:webbroker_api:deployment:read","api-platform:webbroker_api:deployment:delete",
  "api-platform:webbroker_api:deployment:undeploy","api-platform:webbroker_api:deployment:restore",
  "api-platform:webbroker_api:api_key:create","api-platform:webbroker_api:api_key:update","api-platform:webbroker_api:api_key:delete",
  "api-platform:subscription:create","api-platform:subscription:read","api-platform:subscription:update","api-platform:subscription:delete",
  "api-platform:subscription_plan:create","api-platform:subscription_plan:read","api-platform:subscription_plan:update","api-platform:subscription_plan:delete",
  "api-platform:custom_policy:read","api-platform:custom_policy:sync","api-platform:custom_policy:delete"
]'

# Developer: no org/gateway write, no devportal management, no subscription plan write, no custom-policy write
DEVELOPER_PERMISSIONS='[
  "api-platform:project:create","api-platform:project:read","api-platform:project:update","api-platform:project:delete",
  "api-platform:rest_api:create","api-platform:rest_api:read","api-platform:rest_api:update","api-platform:rest_api:delete",
  "api-platform:rest_api:import","api-platform:rest_api:publish",
  "api-platform:rest_api:deployment:create","api-platform:rest_api:deployment:read","api-platform:rest_api:deployment:delete",
  "api-platform:rest_api:deployment:undeploy","api-platform:rest_api:deployment:restore",
  "api-platform:rest_api:gateway:create","api-platform:rest_api:gateway:read",
  "api-platform:rest_api:api_key:create","api-platform:rest_api:api_key:read","api-platform:rest_api:api_key:update","api-platform:rest_api:api_key:delete",
  "api-platform:application:create","api-platform:application:read","api-platform:application:update","api-platform:application:delete",
  "api-platform:application:api_key:create","api-platform:application:api_key:read","api-platform:application:api_key:delete",
  "api-platform:gateway:read",
  "api-platform:devportal:read",
  "api-platform:git:read",
  "api-platform:llm_template:create","api-platform:llm_template:read","api-platform:llm_template:update","api-platform:llm_template:delete",
  "api-platform:llm_provider:create","api-platform:llm_provider:read","api-platform:llm_provider:update","api-platform:llm_provider:delete",
  "api-platform:llm_provider:deployment:create","api-platform:llm_provider:deployment:read","api-platform:llm_provider:deployment:delete",
  "api-platform:llm_provider:deployment:undeploy","api-platform:llm_provider:deployment:restore",
  "api-platform:llm_provider:api_key:create","api-platform:llm_provider:api_key:read","api-platform:llm_provider:api_key:delete",
  "api-platform:llm_proxy:create","api-platform:llm_proxy:read","api-platform:llm_proxy:update","api-platform:llm_proxy:delete",
  "api-platform:llm_proxy:deployment:create","api-platform:llm_proxy:deployment:read","api-platform:llm_proxy:deployment:delete",
  "api-platform:llm_proxy:deployment:undeploy","api-platform:llm_proxy:deployment:restore",
  "api-platform:llm_proxy:api_key:create","api-platform:llm_proxy:api_key:read","api-platform:llm_proxy:api_key:delete",
  "api-platform:mcp_proxy:create","api-platform:mcp_proxy:read","api-platform:mcp_proxy:update","api-platform:mcp_proxy:delete",
  "api-platform:mcp_proxy:deployment:create","api-platform:mcp_proxy:deployment:read","api-platform:mcp_proxy:deployment:delete",
  "api-platform:mcp_proxy:deployment:undeploy","api-platform:mcp_proxy:deployment:restore",
  "api-platform:websub_api:create","api-platform:websub_api:read","api-platform:websub_api:update","api-platform:websub_api:delete",
  "api-platform:websub_api:publish",
  "api-platform:websub_api:deployment:create","api-platform:websub_api:deployment:read","api-platform:websub_api:deployment:delete",
  "api-platform:websub_api:deployment:undeploy","api-platform:websub_api:deployment:restore",
  "api-platform:websub_api:api_key:create","api-platform:websub_api:api_key:update","api-platform:websub_api:api_key:delete",
  "api-platform:webbroker_api:create","api-platform:webbroker_api:read","api-platform:webbroker_api:update","api-platform:webbroker_api:delete",
  "api-platform:webbroker_api:publish",
  "api-platform:webbroker_api:deployment:create","api-platform:webbroker_api:deployment:read","api-platform:webbroker_api:deployment:delete",
  "api-platform:webbroker_api:deployment:undeploy","api-platform:webbroker_api:deployment:restore",
  "api-platform:webbroker_api:api_key:create","api-platform:webbroker_api:api_key:update","api-platform:webbroker_api:api_key:delete",
  "api-platform:subscription:create","api-platform:subscription:read","api-platform:subscription:update","api-platform:subscription:delete",
  "api-platform:subscription_plan:read",
  "api-platform:custom_policy:read"
]'

# Viewer: read-only access across all resources
VIEWER_PERMISSIONS='[
  "api-platform:org:read",
  "api-platform:project:read",
  "api-platform:rest_api:read",
  "api-platform:rest_api:deployment:read",
  "api-platform:rest_api:gateway:read",
  "api-platform:application:read",
  "api-platform:application:api_key:read",
  "api-platform:gateway:read",
  "api-platform:gateway:token:read",
  "api-platform:gateway:policy:read",
  "api-platform:devportal:read",
  "api-platform:llm_template:read",
  "api-platform:llm_provider:read","api-platform:llm_provider:deployment:read",
  "api-platform:llm_proxy:read","api-platform:llm_proxy:deployment:read",
  "api-platform:mcp_proxy:read","api-platform:mcp_proxy:deployment:read",
  "api-platform:websub_api:read","api-platform:websub_api:deployment:read",
  "api-platform:webbroker_api:read","api-platform:webbroker_api:deployment:read",
  "api-platform:subscription:read",
  "api-platform:subscription_plan:read",
  "api-platform:custom_policy:read"
]'

assign_role_permissions "$ADMIN_ROLE_ID"     "admin"     "$ADMIN_PERMISSIONS"
assign_role_permissions "$DEVELOPER_ROLE_ID" "developer" "$DEVELOPER_PERMISSIONS"
assign_role_permissions "$VIEWER_ROLE_ID"    "viewer"    "$VIEWER_PERMISSIONS"

# ===========================================================================
# 8. Configure the AI Workspace application to include org claims in tokens
# ===========================================================================
# The organization and organizations claims must be included in the access
# token so the platform API can scope requests and list the user's orgs.
#
# App client ID is read from AI_WORKSPACE_CLIENT_ID env var (optional).
# When unset this step is skipped — configure claim inclusion manually in
# the Thunder console under the application's "API Authorization" settings.
# ===========================================================================
AI_WORKSPACE_CLIENT_ID="${AI_WORKSPACE_CLIENT_ID:-}"

configure_app_claims() {
  local client_id="$1"
  log_info "Configuring org claims for application client_id=${client_id} ..."

  # Fetch the application ID from the client_id
  local resp http_code body app_id
  resp=$(api_call GET "/applications?clientId=${client_id}")
  http_code="${resp: -3}"; body="${resp%???}"

  if [[ "$http_code" != "200" ]]; then
    log_warning "Could not fetch application for client_id=${client_id} (HTTP ${http_code}) — skipping claim config"
    return 0
  fi

  app_id=$(echo "$body" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
  if [[ -z "$app_id" ]]; then
    log_warning "Application with client_id=${client_id} not found — skipping claim config"
    return 0
  fi

  # Add organization and organizations to the requested claims list
  local claims_payload
  claims_payload=$(cat <<EOF
{
  "claimConfiguration": {
    "requestedClaims": [
      { "claim": { "uri": "http://wso2.org/claims/organization" }, "mandatory": false },
      { "claim": { "uri": "http://wso2.org/claims/organizations" }, "mandatory": false }
    ],
    "includeInAccessToken": true
  }
}
EOF
)
  resp=$(api_call PATCH "/applications/${app_id}" "$claims_payload")
  http_code="${resp: -3}"; body="${resp%???}"

  if [[ "$http_code" == "200" ]] || [[ "$http_code" == "204" ]]; then
    log_success "Org claims configured for application ${app_id}"
  else
    log_warning "Could not configure org claims for application ${app_id} (HTTP ${http_code})"
    log_warning "Add 'organization' and 'organizations' claims manually in the Thunder console"
  fi
}

if [[ -n "$AI_WORKSPACE_CLIENT_ID" ]]; then
  configure_app_claims "$AI_WORKSPACE_CLIENT_ID"
else
  log_warning "AI_WORKSPACE_CLIENT_ID not set — skipping automatic claim configuration."
  log_warning "Manually add 'organization' and 'organizations' to the app's access token claims in Thunder."
fi

log_success "API Platform resource server registration complete."
log_info ""
log_info "Resource server ID : $RS_ID"
log_info "Identifier (aud)   : $RS_IDENTIFIER"
log_info ""
log_info "Org claims setup:"
log_info "  organization  → http://wso2.org/claims/organization  (active org UUID per session)"
log_info "  organizations → http://wso2.org/claims/organizations (all org UUIDs, space-separated)"
log_info ""
log_info "To use these scopes, request a token with:"
log_info "  scope=api-platform:gateway:create api-platform:api:read ..."
log_info "  resource=$RS_IDENTIFIER"
