#!/bin/bash

##############################################################################
# STS Kickstart Script
#
# This script creates an organization, user, and application in Thunder
# Run this AFTER starting the STS Docker container
#
# Usage:
#   ./kickstart.sh [inputs_yaml]
#
# Defaults:
#   inputs_yaml: inputs.yaml
##############################################################################

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
INPUTS_FILE="${1:-inputs.yaml}"
OUTPUT_FILE="registration.yaml"
THUNDER_URL="https://localhost:8090"
MAX_RETRIES=30
RETRY_DELAY=2

##############################################################################
# Helper Functions
##############################################################################

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Wait for Thunder to be ready
wait_for_thunder() {
    log_info "Waiting for Thunder to be ready..."
    local retries=0

    while [ $retries -lt $MAX_RETRIES ]; do
        if curl -k -s -f "$THUNDER_URL/health/liveness" > /dev/null 2>&1; then
            log_success "Thunder is ready!"
            return 0
        fi

        retries=$((retries + 1))
        echo -n "."
        sleep $RETRY_DELAY
    done

    echo ""
    log_error "Thunder failed to start after $((MAX_RETRIES * RETRY_DELAY)) seconds"
    log_error "Make sure the STS container is running:"
    log_error "  docker run -d -p 8090:8090 -p 9090:9090 wso2/api-platform-sts:latest"
    return 1
}

# Parse YAML (basic parser using grep/sed - works without yq)
parse_yaml() {
    local file=$1
    local key=$2
    local default=$3

    if [ ! -f "$file" ]; then
        echo "$default"
        return
    fi

    # Simple YAML parser - handles basic key: value format
    local value=$(grep "^[[:space:]]*$key:" "$file" | sed 's/.*:[[:space:]]*//' | tr -d '"' | tr -d "'")

    if [ -z "$value" ]; then
        echo "$default"
    else
        echo "$value"
    fi
}

# Parse YAML list items
parse_yaml_list() {
    local file=$1
    local parent_key=$2

    if [ ! -f "$file" ]; then
        return
    fi

    grep -A 100 "^$parent_key:" "$file" | grep "^[[:space:]]*-" | sed 's/^[[:space:]]*-[[:space:]]*//' | tr -d '"' | tr -d "'"
}

##############################################################################
# Main Script
##############################################################################

echo ""
echo "=========================================="
echo "  STS Kickstart Script"
echo "=========================================="
echo ""

# Check if jq is available (optional, but helpful)
if ! command -v jq &> /dev/null; then
    log_warning "jq is not installed. JSON parsing will be basic."
    log_warning "Install jq for better output: sudo apt-get install jq"
fi

# Wait for Thunder to be ready
wait_for_thunder || exit 1

echo ""
log_info "Loading configuration from $INPUTS_FILE..."

# Load configuration with defaults
ORG_NAME=$(parse_yaml "$INPUTS_FILE" "name" "Default Organization")
ORG_HANDLE=$(parse_yaml "$INPUTS_FILE" "handle" "default-org")
ORG_DESC=$(parse_yaml "$INPUTS_FILE" "description" "Default Organization")

USER_USERNAME=$(parse_yaml "$INPUTS_FILE" "username" "admin")
USER_PASSWORD=$(parse_yaml "$INPUTS_FILE" "password" "Admin@123")
USER_EMAIL=$(parse_yaml "$INPUTS_FILE" "email" "admin@example.com")
USER_FIRSTNAME=$(parse_yaml "$INPUTS_FILE" "firstName" "Admin")
USER_LASTNAME=$(parse_yaml "$INPUTS_FILE" "lastName" "User")
USER_TYPE=$(parse_yaml "$INPUTS_FILE" "type" "superhuman")

APP_NAME=$(parse_yaml "$INPUTS_FILE" "application_name" "Management Portal")
APP_DESC=$(parse_yaml "$INPUTS_FILE" "application_description" "Management Portal Application")
APP_CLIENT_ID=$(parse_yaml "$INPUTS_FILE" "client_id" "management-portal-client")
APP_CLIENT_SECRET=$(parse_yaml "$INPUTS_FILE" "client_secret" "$(openssl rand -hex 32)")

# Get redirect URIs (default if not specified)
REDIRECT_URIS=$(parse_yaml_list "$INPUTS_FILE" "redirect_uris")
if [ -z "$REDIRECT_URIS" ]; then
    REDIRECT_URIS="https://localhost:3000/callback"
fi

echo ""
log_info "Configuration:"
log_info "  Organization: $ORG_NAME ($ORG_HANDLE)"
log_info "  User: $USER_USERNAME ($USER_EMAIL)"
log_info "  Application: $APP_NAME"
echo ""

##############################################################################
# Step 1: Create Organization
##############################################################################

log_info "[1/3] Creating organization unit..."

ORG_PAYLOAD=$(cat <<EOF
{
  "name": "$ORG_NAME",
  "description": "$ORG_DESC",
  "handle": "$ORG_HANDLE"
}
EOF
)

ORG_RESPONSE=$(curl -k -s -X POST "$THUNDER_URL/organization-units" \
    -H "Content-Type: application/json" \
    -d "$ORG_PAYLOAD")

if command -v jq &> /dev/null; then
    ORG_ID=$(echo "$ORG_RESPONSE" | jq -r '.id // empty')
else
    ORG_ID=$(echo "$ORG_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | sed 's/"id":"\([^"]*\)"/\1/')
fi

if [ -z "$ORG_ID" ]; then
    log_error "Failed to create organization"
    log_error "Response: $ORG_RESPONSE"
    exit 1
fi

log_success "Organization created with ID: $ORG_ID"

##############################################################################
# Step 2: Create User
##############################################################################

log_info "[2/3] Creating user..."

USER_PAYLOAD=$(cat <<EOF
{
  "organizationUnit": "$ORG_ID",
  "type": "$USER_TYPE",
  "attributes": {
    "username": "$USER_USERNAME",
    "password": "$USER_PASSWORD",
    "email": "$USER_EMAIL",
    "firstName": "$USER_FIRSTNAME",
    "lastName": "$USER_LASTNAME"
  }
}
EOF
)

USER_RESPONSE=$(curl -k -s -X POST "$THUNDER_URL/users" \
    -H "Content-Type: application/json" \
    -d "$USER_PAYLOAD")

if command -v jq &> /dev/null; then
    USER_ID=$(echo "$USER_RESPONSE" | jq -r '.id // empty')
else
    USER_ID=$(echo "$USER_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | sed 's/"id":"\([^"]*\)"/\1/')
fi

if [ -z "$USER_ID" ]; then
    log_error "Failed to create user"
    log_error "Response: $USER_RESPONSE"
    exit 1
fi

log_success "User created with ID: $USER_ID"

##############################################################################
# Step 3: Create Application
##############################################################################

log_info "[3/3] Creating application..."

# Build redirect URIs JSON array
REDIRECT_URIS_JSON="["
first=true
while IFS= read -r uri; do
    if [ "$first" = true ]; then
        REDIRECT_URIS_JSON="${REDIRECT_URIS_JSON}\"$uri\""
        first=false
    else
        REDIRECT_URIS_JSON="${REDIRECT_URIS_JSON}, \"$uri\""
    fi
done <<< "$REDIRECT_URIS"
REDIRECT_URIS_JSON="${REDIRECT_URIS_JSON}]"

APP_PAYLOAD=$(cat <<EOF
{
  "name": "$APP_NAME",
  "description": "$APP_DESC",
  "auth_flow_graph_id": "auth_flow_config_basic",
  "inbound_auth_config": [{
    "type": "oauth2",
    "config": {
      "client_id": "$APP_CLIENT_ID",
      "client_secret": "$APP_CLIENT_SECRET",
      "redirect_uris": $REDIRECT_URIS_JSON,
      "grant_types": ["authorization_code", "refresh_token"],
      "response_types": ["code"],
      "token_endpoint_auth_methods": ["client_secret_basic", "client_secret_post"],
      "pkce_required": false
    }
  }]
}
EOF
)

APP_RESPONSE=$(curl -k -s -X POST "$THUNDER_URL/applications" \
    -H "Content-Type: application/json" \
    -d "$APP_PAYLOAD")

if command -v jq &> /dev/null; then
    APP_ID=$(echo "$APP_RESPONSE" | jq -r '.id // empty')
else
    APP_ID=$(echo "$APP_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | sed 's/"id":"\([^"]*\)"/\1/')
fi

if [ -z "$APP_ID" ]; then
    log_error "Failed to create application"
    log_error "Response: $APP_RESPONSE"
    exit 1
fi

log_success "Application created with ID: $APP_ID"

##############################################################################
# Generate Output YAML
##############################################################################

echo ""
log_info "Generating $OUTPUT_FILE..."

# Get redirect URIs as YAML list
REDIRECT_URIS_YAML=""
while IFS= read -r uri; do
    REDIRECT_URIS_YAML="${REDIRECT_URIS_YAML}    - \"$uri\"\n"
done <<< "$REDIRECT_URIS"

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
FIRST_REDIRECT_URI=$(echo "$REDIRECT_URIS" | head -1)

cat > "$OUTPUT_FILE" <<EOF
# STS Registration Output
# Generated: $TIMESTAMP

organization:
  id: "$ORG_ID"
  name: "$ORG_NAME"
  handle: "$ORG_HANDLE"
  description: "$ORG_DESC"
  created_at: "$TIMESTAMP"

user:
  id: "$USER_ID"
  username: "$USER_USERNAME"
  email: "$USER_EMAIL"
  firstName: "$USER_FIRSTNAME"
  lastName: "$USER_LASTNAME"
  organization_unit: "$ORG_ID"
  created_at: "$TIMESTAMP"

  # Credentials (KEEP SECURE!)
  password: "$USER_PASSWORD"

application:
  id: "$APP_ID"
  name: "$APP_NAME"
  description: "$APP_DESC"
  client_id: "$APP_CLIENT_ID"
  client_secret: "$APP_CLIENT_SECRET"
  auth_flow_graph_id: "auth_flow_config_basic"
  created_at: "$TIMESTAMP"

  redirect_uris:
$(echo -e "$REDIRECT_URIS_YAML")

  grant_types:
    - "authorization_code"
    - "refresh_token"

  response_types:
    - "code"

# OAuth 2.0 Endpoints
oauth_endpoints:
  authorize: "$THUNDER_URL/oauth2/authorize"
  token: "$THUNDER_URL/oauth2/token"
  userinfo: "$THUNDER_URL/oauth2/userinfo"

# Example Authorization URL
example_auth_url: "$THUNDER_URL/oauth2/authorize?response_type=code&client_id=$APP_CLIENT_ID&redirect_uri=$FIRST_REDIRECT_URI&scope=openid&state=random_state_123"

# Quick Test Commands
test_commands:
  # 1. Get authorization code (open in browser)
  authorize: "Open the example_auth_url in a browser and login with user credentials"

  # 2. Exchange code for token
  token: |
    curl -k -X POST $THUNDER_URL/oauth2/token \\
      -u $APP_CLIENT_ID:$APP_CLIENT_SECRET \\
      -d "grant_type=authorization_code" \\
      -d "code=<authorization_code>" \\
      -d "redirect_uri=$FIRST_REDIRECT_URI"
EOF

log_success "Registration details saved to $OUTPUT_FILE"

echo ""
echo "=========================================="
log_success "Kickstart completed successfully!"
echo "=========================================="
echo ""
log_info "Summary:"
log_info "  Organization ID: $ORG_ID"
log_info "  User ID: $USER_ID"
log_info "  Application ID: $APP_ID"
log_info "  Client ID: $APP_CLIENT_ID"
echo ""
log_info "Next steps:"
log_info "  1. Review $OUTPUT_FILE for all details"
log_info "  2. Follow the Authorization Code Grant flow:"
log_info "     - Open the example_auth_url from $OUTPUT_FILE in your browser"
log_info "     - Login with the user credentials from $OUTPUT_FILE"
log_info "     - Use the authorization code to exchange for tokens"
echo ""
