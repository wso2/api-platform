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
CLIENT_ID="${THUNDER_CLIENT_ID:-api-platform-system-client}" # yq9TyDfqn-8C3beXEF09GQ
CLIENT_SECRET="${THUNDER_CLIENT_SECRET:-api-platform-system-client-secret}" # uX9he-XZQP13Z7QA1fJTbPhd0YFgE_XkhNOHgSWbVDw
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
# 1. Resource server
# ===========================================================================
log_info "Creating 'api-platform' resource server..."
RS_ID=$(create_or_get_rs "API Platform" "api-platform" "$RS_IDENTIFIER" "WSO2 API Platform permissions" "$DEFAULT_OU_ID")
log_info "Resource server ready (id: $RS_ID)"

# ===========================================================================
# 2. Level-1 resources
# ===========================================================================
log_info "Creating level-1 resources..."
R_ORG=$(create_or_get_resource          "$RS_ID" "Organization"            "org"                   "Organization management")
R_PROJECT=$(create_or_get_resource      "$RS_ID" "Project"                 "project"               "Project management")
R_API=$(create_or_get_resource          "$RS_ID" "API"                     "api"                   "API management")
R_APP=$(create_or_get_resource          "$RS_ID" "Application"             "application"           "Application management")
R_GW=$(create_or_get_resource           "$RS_ID" "Gateway"                 "gateway"               "Gateway management")
R_DP=$(create_or_get_resource           "$RS_ID" "DevPortal"               "devportal"             "Developer portal management")
R_DEPLOY=$(create_or_get_resource       "$RS_ID" "Deployment"              "deployment"            "API deployment management")
R_GIT=$(create_or_get_resource          "$RS_ID" "Git"                     "git"                   "Git repository access")
R_LLMT=$(create_or_get_resource         "$RS_ID" "LLM Provider Template"   "llm-provider-template" "LLM provider template management")
R_LLM=$(create_or_get_resource          "$RS_ID" "LLM Provider"            "llm-provider"          "LLM provider management")
R_PROXY=$(create_or_get_resource        "$RS_ID" "LLM Proxy"               "llm-proxy"             "LLM proxy management")
R_MCP=$(create_or_get_resource          "$RS_ID" "MCP Proxy"               "mcp-proxy"             "MCP proxy management")
R_WEBSUB=$(create_or_get_resource       "$RS_ID" "WebSub API"              "websub-api"            "WebSub API management")
R_WEBBROKER=$(create_or_get_resource    "$RS_ID" "WebBroker API"           "webbroker-api"         "WebBroker API management")
R_SUB=$(create_or_get_resource          "$RS_ID" "Subscription"            "subscription"          "API subscription management")
R_SUBPLAN=$(create_or_get_resource      "$RS_ID" "Subscription Plan"       "subscription-plan"     "Subscription plan management")
R_POLICY=$(create_or_get_resource       "$RS_ID" "Custom Policy"           "custom-policy"         "Custom policy management")
log_info "Level-1 resources created."

# ===========================================================================
# 3. Level-2 sub-resources
# ===========================================================================
log_info "Creating level-2 sub-resources..."
R_GW_TOKEN=$(create_or_get_resource     "$RS_ID" "Gateway Token"           "token"   "Gateway token management"           "$R_GW")
R_GW_APIKEY=$(create_or_get_resource    "$RS_ID" "Gateway API Key"         "api-key" "Gateway-level API key management"   "$R_GW")
R_LLM_KEY=$(create_or_get_resource      "$RS_ID" "LLM Provider API Key"    "api-key" "LLM provider API key management"    "$R_LLM")
R_PROXY_KEY=$(create_or_get_resource    "$RS_ID" "LLM Proxy API Key"       "api-key" "LLM proxy API key management"       "$R_PROXY")
R_API_KEY=$(create_or_get_resource      "$RS_ID" "API Key"                 "api-key" "API-level API key management"       "$R_API")
R_APP_KEY=$(create_or_get_resource      "$RS_ID" "Application API Key"     "api-key" "Application API key management"     "$R_APP")
log_info "Level-2 sub-resources created."

# ===========================================================================
# 4. Actions
# ===========================================================================
log_info "Creating actions..."

# Organization (admin-only for write; read implied by token presence)
create_action "$RS_ID" "$R_ORG"        "Create"  "create"  "Create an organization"
create_action "$RS_ID" "$R_ORG"        "Read"    "read"    "View organization details"
create_action "$RS_ID" "$R_ORG"        "Update"  "update"  "Update organization settings"
create_action "$RS_ID" "$R_ORG"        "Delete"  "delete"  "Delete an organization"

# Project (admin + developer)
create_action "$RS_ID" "$R_PROJECT"    "Create"  "create"  "Create a project"
create_action "$RS_ID" "$R_PROJECT"    "Read"    "read"    "View project details"
create_action "$RS_ID" "$R_PROJECT"    "Update"  "update"  "Update a project"
create_action "$RS_ID" "$R_PROJECT"    "Delete"  "delete"  "Delete a project"

# API (admin + developer)
create_action "$RS_ID" "$R_API"        "Create"       "create"       "Create an API"
create_action "$RS_ID" "$R_API"        "Read"         "read"         "View API details"
create_action "$RS_ID" "$R_API"        "Update"       "update"       "Update an API"
create_action "$RS_ID" "$R_API"        "Delete"       "delete"       "Delete an API"
create_action "$RS_ID" "$R_API"        "Import"       "import"       "Import an API from OpenAPI spec or project"
create_action "$RS_ID" "$R_API"        "Add Gateway"  "add-gateway"  "Associate an API with a gateway"
create_action "$RS_ID" "$R_API"        "Publish"      "publish"      "Publish an API to a developer portal"
create_action "$RS_ID" "$R_API"        "Unpublish"    "unpublish"    "Unpublish an API from a developer portal"
create_action "$RS_ID" "$R_API_KEY"    "Manage"       "manage"       "Create, update, list, and delete API-level API keys"

# Application (admin + developer)
create_action "$RS_ID" "$R_APP"        "Create"      "create"      "Create an application"
create_action "$RS_ID" "$R_APP"        "Read"        "read"        "View application details"
create_action "$RS_ID" "$R_APP"        "Update"      "update"      "Update an application"
create_action "$RS_ID" "$R_APP"        "Delete"      "delete"      "Delete an application"
create_action "$RS_ID" "$R_APP_KEY"    "Manage"      "manage"      "Add and remove application API keys"

# Gateway (admin-only for write)
create_action "$RS_ID" "$R_GW"         "Create"  "create"  "Register a gateway"
create_action "$RS_ID" "$R_GW"         "Read"    "read"    "View gateway details and status"
create_action "$RS_ID" "$R_GW"         "Update"  "update"  "Update gateway configuration"
create_action "$RS_ID" "$R_GW"         "Delete"  "delete"  "Delete a gateway"
create_action "$RS_ID" "$R_GW_TOKEN"   "Manage"  "manage"  "Rotate and revoke gateway tokens"
create_action "$RS_ID" "$R_GW_APIKEY"  "Manage"  "manage"  "Create and delete gateway-level API keys"

# DevPortal (admin-only)
create_action "$RS_ID" "$R_DP"         "Create"      "create"      "Create a developer portal"
create_action "$RS_ID" "$R_DP"         "Read"        "read"        "View developer portal details"
create_action "$RS_ID" "$R_DP"         "Update"      "update"      "Update developer portal settings"
create_action "$RS_ID" "$R_DP"         "Delete"      "delete"      "Delete a developer portal"
create_action "$RS_ID" "$R_DP"         "Activate"    "activate"    "Activate a developer portal"
create_action "$RS_ID" "$R_DP"         "Deactivate"  "deactivate"  "Deactivate a developer portal"
create_action "$RS_ID" "$R_DP"         "Set Default" "set-default" "Set a developer portal as the default"

# Deployment (admin + developer)
create_action "$RS_ID" "$R_DEPLOY"     "Deploy"    "deploy"    "Deploy an API to a gateway"
create_action "$RS_ID" "$R_DEPLOY"     "Undeploy"  "undeploy"  "Undeploy an API from a gateway"
create_action "$RS_ID" "$R_DEPLOY"     "Restore"   "restore"   "Restore a previous deployment"
create_action "$RS_ID" "$R_DEPLOY"     "Delete"    "delete"    "Delete a deployment record"

# Git (admin + developer)
create_action "$RS_ID" "$R_GIT"        "Read"    "read"    "Browse repository branches and fetch content"

# LLM Provider Template (admin + developer)
create_action "$RS_ID" "$R_LLMT"       "Create"  "create"  "Create an LLM provider template"
create_action "$RS_ID" "$R_LLMT"       "Read"    "read"    "View LLM provider templates"
create_action "$RS_ID" "$R_LLMT"       "Update"  "update"  "Update an LLM provider template"
create_action "$RS_ID" "$R_LLMT"       "Delete"  "delete"  "Delete an LLM provider template"

# LLM Provider (admin + developer)
create_action "$RS_ID" "$R_LLM"        "Create"  "create"  "Create an LLM provider"
create_action "$RS_ID" "$R_LLM"        "Read"    "read"    "View LLM providers and deployments"
create_action "$RS_ID" "$R_LLM"        "Update"  "update"  "Update an LLM provider"
create_action "$RS_ID" "$R_LLM"        "Delete"  "delete"  "Delete an LLM provider"
create_action "$RS_ID" "$R_LLM"        "Deploy"  "deploy"  "Deploy, undeploy, and restore an LLM provider"
create_action "$RS_ID" "$R_LLM_KEY"    "Manage"  "manage"  "Create, update, and delete LLM provider API keys"

# LLM Proxy (admin + developer)
create_action "$RS_ID" "$R_PROXY"      "Create"  "create"  "Create an LLM proxy"
create_action "$RS_ID" "$R_PROXY"      "Read"    "read"    "View LLM proxies and deployments"
create_action "$RS_ID" "$R_PROXY"      "Update"  "update"  "Update an LLM proxy"
create_action "$RS_ID" "$R_PROXY"      "Delete"  "delete"  "Delete an LLM proxy"
create_action "$RS_ID" "$R_PROXY"      "Deploy"  "deploy"  "Deploy, undeploy, and restore an LLM proxy"
create_action "$RS_ID" "$R_PROXY_KEY"  "Manage"  "manage"  "Create, update, and delete LLM proxy API keys"

# MCP Proxy (admin + developer)
create_action "$RS_ID" "$R_MCP"        "Create"     "create"      "Create an MCP proxy"
create_action "$RS_ID" "$R_MCP"        "Read"       "read"        "View MCP proxies"
create_action "$RS_ID" "$R_MCP"        "Update"     "update"      "Update an MCP proxy"
create_action "$RS_ID" "$R_MCP"        "Delete"     "delete"      "Delete an MCP proxy"
create_action "$RS_ID" "$R_MCP"        "Deploy"     "deploy"      "Deploy, undeploy, and restore an MCP proxy"
create_action "$RS_ID" "$R_MCP"        "Fetch Info" "fetch-info"  "Fetch MCP server info"

# WebSub API (admin + developer)
create_action "$RS_ID" "$R_WEBSUB"     "Create"    "create"    "Create a WebSub API"
create_action "$RS_ID" "$R_WEBSUB"     "Read"      "read"      "View WebSub APIs"
create_action "$RS_ID" "$R_WEBSUB"     "Update"    "update"    "Update a WebSub API"
create_action "$RS_ID" "$R_WEBSUB"     "Delete"    "delete"    "Delete a WebSub API"
create_action "$RS_ID" "$R_WEBSUB"     "Deploy"    "deploy"    "Deploy, undeploy, and restore a WebSub API"
create_action "$RS_ID" "$R_WEBSUB"     "Publish"   "publish"   "Publish a WebSub API to a developer portal"
create_action "$RS_ID" "$R_WEBSUB"     "Unpublish" "unpublish" "Unpublish a WebSub API from a developer portal"

# WebBroker API (admin + developer)
create_action "$RS_ID" "$R_WEBBROKER"  "Create"    "create"    "Create a WebBroker API"
create_action "$RS_ID" "$R_WEBBROKER"  "Read"      "read"      "View WebBroker APIs"
create_action "$RS_ID" "$R_WEBBROKER"  "Update"    "update"    "Update a WebBroker API"
create_action "$RS_ID" "$R_WEBBROKER"  "Delete"    "delete"    "Delete a WebBroker API"
create_action "$RS_ID" "$R_WEBBROKER"  "Deploy"    "deploy"    "Deploy, undeploy, and restore a WebBroker API"
create_action "$RS_ID" "$R_WEBBROKER"  "Publish"   "publish"   "Publish a WebBroker API to a developer portal"
create_action "$RS_ID" "$R_WEBBROKER"  "Unpublish" "unpublish" "Unpublish a WebBroker API from a developer portal"

# Subscription (admin + developer)
create_action "$RS_ID" "$R_SUB"        "Create"  "create"  "Create a subscription"
create_action "$RS_ID" "$R_SUB"        "Read"    "read"    "View subscriptions"
create_action "$RS_ID" "$R_SUB"        "Update"  "update"  "Update a subscription"
create_action "$RS_ID" "$R_SUB"        "Delete"  "delete"  "Delete a subscription"

# Subscription Plan (admin-only)
create_action "$RS_ID" "$R_SUBPLAN"    "Create"  "create"  "Create a subscription plan"
create_action "$RS_ID" "$R_SUBPLAN"    "Read"    "read"    "View subscription plans"
create_action "$RS_ID" "$R_SUBPLAN"    "Update"  "update"  "Update a subscription plan"
create_action "$RS_ID" "$R_SUBPLAN"    "Delete"  "delete"  "Delete a subscription plan"

# Custom Policy (admin-only)
create_action "$RS_ID" "$R_POLICY"     "Read"    "read"    "View custom policies"
create_action "$RS_ID" "$R_POLICY"     "Sync"    "sync"    "Sync a custom policy to gateways"
create_action "$RS_ID" "$R_POLICY"     "Delete"  "delete"  "Delete a custom policy"

log_success "All actions registered."

# ===========================================================================
# 5. Assign permissions to roles
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
  "api-platform:api:create","api-platform:api:read","api-platform:api:update","api-platform:api:delete",
  "api-platform:api:import","api-platform:api:add-gateway","api-platform:api:publish","api-platform:api:unpublish",
  "api-platform:api:api-key:manage",
  "api-platform:application:create","api-platform:application:read","api-platform:application:update","api-platform:application:delete",
  "api-platform:application:api-key:manage",
  "api-platform:gateway:create","api-platform:gateway:read","api-platform:gateway:update","api-platform:gateway:delete",
  "api-platform:gateway:token:manage","api-platform:gateway:api-key:manage",
  "api-platform:devportal:create","api-platform:devportal:read","api-platform:devportal:update","api-platform:devportal:delete",
  "api-platform:devportal:activate","api-platform:devportal:deactivate","api-platform:devportal:set-default",
  "api-platform:deployment:deploy","api-platform:deployment:undeploy","api-platform:deployment:restore","api-platform:deployment:delete",
  "api-platform:git:read",
  "api-platform:llm-provider-template:create","api-platform:llm-provider-template:read","api-platform:llm-provider-template:update","api-platform:llm-provider-template:delete",
  "api-platform:llm-provider:create","api-platform:llm-provider:read","api-platform:llm-provider:update","api-platform:llm-provider:delete",
  "api-platform:llm-provider:deploy","api-platform:llm-provider:api-key:manage",
  "api-platform:llm-proxy:create","api-platform:llm-proxy:read","api-platform:llm-proxy:update","api-platform:llm-proxy:delete",
  "api-platform:llm-proxy:deploy","api-platform:llm-proxy:api-key:manage",
  "api-platform:mcp-proxy:create","api-platform:mcp-proxy:read","api-platform:mcp-proxy:update","api-platform:mcp-proxy:delete",
  "api-platform:mcp-proxy:deploy","api-platform:mcp-proxy:fetch-info",
  "api-platform:websub-api:create","api-platform:websub-api:read","api-platform:websub-api:update","api-platform:websub-api:delete",
  "api-platform:websub-api:deploy","api-platform:websub-api:publish","api-platform:websub-api:unpublish",
  "api-platform:webbroker-api:create","api-platform:webbroker-api:read","api-platform:webbroker-api:update","api-platform:webbroker-api:delete",
  "api-platform:webbroker-api:deploy","api-platform:webbroker-api:publish","api-platform:webbroker-api:unpublish",
  "api-platform:subscription:create","api-platform:subscription:read","api-platform:subscription:update","api-platform:subscription:delete",
  "api-platform:subscription-plan:create","api-platform:subscription-plan:read","api-platform:subscription-plan:update","api-platform:subscription-plan:delete",
  "api-platform:custom-policy:read","api-platform:custom-policy:sync","api-platform:custom-policy:delete"
]'

# Developer: no org/gateway write, no devportal management, no custom-policy sync/delete
DEVELOPER_PERMISSIONS='[
  "api-platform:project:create","api-platform:project:read","api-platform:project:update","api-platform:project:delete",
  "api-platform:api:create","api-platform:api:read","api-platform:api:update","api-platform:api:delete",
  "api-platform:api:import","api-platform:api:add-gateway","api-platform:api:publish","api-platform:api:unpublish",
  "api-platform:api:api-key:manage",
  "api-platform:application:create","api-platform:application:read","api-platform:application:update","api-platform:application:delete",
  "api-platform:application:api-key:manage",
  "api-platform:gateway:read",
  "api-platform:deployment:deploy","api-platform:deployment:undeploy","api-platform:deployment:restore","api-platform:deployment:delete",
  "api-platform:git:read",
  "api-platform:llm-provider-template:create","api-platform:llm-provider-template:read","api-platform:llm-provider-template:update","api-platform:llm-provider-template:delete",
  "api-platform:llm-provider:create","api-platform:llm-provider:read","api-platform:llm-provider:update","api-platform:llm-provider:delete",
  "api-platform:llm-provider:deploy","api-platform:llm-provider:api-key:manage",
  "api-platform:llm-proxy:create","api-platform:llm-proxy:read","api-platform:llm-proxy:update","api-platform:llm-proxy:delete",
  "api-platform:llm-proxy:deploy","api-platform:llm-proxy:api-key:manage",
  "api-platform:mcp-proxy:create","api-platform:mcp-proxy:read","api-platform:mcp-proxy:update","api-platform:mcp-proxy:delete",
  "api-platform:mcp-proxy:deploy","api-platform:mcp-proxy:fetch-info",
  "api-platform:websub-api:create","api-platform:websub-api:read","api-platform:websub-api:update","api-platform:websub-api:delete",
  "api-platform:websub-api:deploy","api-platform:websub-api:publish","api-platform:websub-api:unpublish",
  "api-platform:webbroker-api:create","api-platform:webbroker-api:read","api-platform:webbroker-api:update","api-platform:webbroker-api:delete",
  "api-platform:webbroker-api:deploy","api-platform:webbroker-api:publish","api-platform:webbroker-api:unpublish",
  "api-platform:subscription:create","api-platform:subscription:read","api-platform:subscription:update","api-platform:subscription:delete",
  "api-platform:subscription-plan:read",
  "api-platform:custom-policy:read"
]'

# Viewer: read-only access across all resources
VIEWER_PERMISSIONS='[
  "api-platform:org:read",
  "api-platform:project:read",
  "api-platform:api:read",
  "api-platform:application:read",
  "api-platform:gateway:read",
  "api-platform:devportal:read",
  "api-platform:llm-provider-template:read",
  "api-platform:llm-provider:read",
  "api-platform:llm-proxy:read",
  "api-platform:mcp-proxy:read",
  "api-platform:websub-api:read",
  "api-platform:webbroker-api:read",
  "api-platform:subscription:read",
  "api-platform:subscription-plan:read",
  "api-platform:custom-policy:read"
]'

assign_role_permissions "$ADMIN_ROLE_ID"     "admin"     "$ADMIN_PERMISSIONS"
assign_role_permissions "$DEVELOPER_ROLE_ID" "developer" "$DEVELOPER_PERMISSIONS"
assign_role_permissions "$VIEWER_ROLE_ID"    "viewer"    "$VIEWER_PERMISSIONS"

log_success "API Platform resource server registration complete."
log_info ""
log_info "Resource server ID : $RS_ID"
log_info "Identifier (aud)   : $RS_IDENTIFIER"
log_info ""
log_info "To use these scopes, request a token with:"
log_info "  scope=api-platform:gateway:create api-platform:api:read ..."
log_info "  resource=$RS_IDENTIFIER"
