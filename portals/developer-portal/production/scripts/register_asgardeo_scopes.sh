#!/usr/bin/env bash
# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License. You may obtain a copy of the
# License at http://www.apache.org/licenses/LICENSE-2.0
# --------------------------------------------------------------------

# register_asgardeo_scopes.sh
#
# Registers the API Portal API resource and all dp:* scopes in Asgardeo
# so that the API Portal traditional web application can request them.
#
# Prerequisites:
#   1. Create a system OIDC application in Asgardeo.
#   2. Add "API Resource Management API" and "Application Management API" to it.
#   3. Export the system app's client ID and secret as env vars below.
#
# Usage:
#   ./register_asgardeo_scopes.sh
#
# Override defaults via env vars:
#   ASGARDEO_TENANT              Asgardeo tenant/root-org name
#   ASGARDEO_CLIENT_ID           System application client ID
#   ASGARDEO_CLIENT_SECRET       System application client secret
#   ASGARDEO_RESOURCE_IDENTIFIER API resource identifier (usually the API Portal base URL)
#   ASGARDEO_RESOURCE_NAME       Display name for the API resource

set -euo pipefail

TENANT="${ASGARDEO_TENANT:-}"
CLIENT_ID="${ASGARDEO_CLIENT_ID:-}"
CLIENT_SECRET="${ASGARDEO_CLIENT_SECRET:-}"
RESOURCE_IDENTIFIER="${ASGARDEO_RESOURCE_IDENTIFIER:-https://localhost:9543}"
RESOURCE_NAME="${ASGARDEO_RESOURCE_NAME:-API Portal Resources}"

if [[ -z "$TENANT" || -z "$CLIENT_ID" || -z "$CLIENT_SECRET" ]]; then
  echo "Error: ASGARDEO_TENANT, ASGARDEO_CLIENT_ID, and ASGARDEO_CLIENT_SECRET must be set." >&2
  exit 1
fi

TOKEN_EP="https://api.asgardeo.io/t/${TENANT}/oauth2/token"
BASE_URL="https://api.asgardeo.io/t/${TENANT}/api/server/v1/api-resources"

# ── Fetch management token via client credentials ─────────────────────────────

MGMT_SCOPES="internal_api_resource_create internal_api_resource_delete internal_api_resource_update internal_api_resource_view internal_application_business_api_update internal_application_internal_api_update internal_application_mgt_client_secret_create internal_application_mgt_client_secret_view internal_application_mgt_create internal_application_mgt_delete internal_application_mgt_update internal_application_mgt_view internal_org_api_resource_view internal_role_mgt_update internal_role_mgt_view"

echo "Fetching access token for tenant '${TENANT}'..."

token_resp=$(curl -s -w "\n%{http_code}" \
  -X POST "${TOKEN_EP}" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -u "${CLIENT_ID}:${CLIENT_SECRET}" \
  --data-urlencode "grant_type=client_credentials" \
  --data-urlencode "scope=${MGMT_SCOPES}")

token_status=$(echo "$token_resp" | tail -n1)
token_body=$(echo "$token_resp" | sed '$d')

if [[ "$token_status" != "200" ]]; then
  echo "Error: token request failed (HTTP ${token_status}): ${token_body}" >&2
  exit 1
fi

TOKEN=$(echo "$token_body" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
if [[ -z "$TOKEN" ]]; then
  echo "Error: could not parse access_token from response" >&2
  exit 1
fi

echo "Token obtained."
echo ""

# ── dp:* scope list ───────────────────────────────────────────────────────────

SCOPES=(
  # organization
  "dp:organization:create"
  "dp:organization:read"
  "dp:organization:update"
  "dp:organization:delete"
  "dp:organization:manage"

  # organization content
  "dp:organization_content:create"
  "dp:organization_content:read"
  "dp:organization_content:update"
  "dp:organization_content:delete"
  "dp:organization_content:manage"

  # views
  "dp:view:create"
  "dp:view:read"
  "dp:view:update"
  "dp:view:delete"
  "dp:view:manage"

  # labels
  "dp:label:create"
  "dp:label:read"
  "dp:label:update"
  "dp:label:delete"
  "dp:label:manage"

  # providers
  "dp:provider:create"
  "dp:provider:read"
  "dp:provider:update"
  "dp:provider:delete"
  "dp:provider:manage"

  # key managers
  "dp:key_manager:create"
  "dp:key_manager:read"
  "dp:key_manager:update"
  "dp:key_manager:delete"
  "dp:key_manager:manage"

  # APIs
  "dp:api:create"
  "dp:api:read"
  "dp:api:update"
  "dp:api:delete"
  "dp:api:manage"

  # API content
  "dp:api_content:create"
  "dp:api_content:read"
  "dp:api_content:update"
  "dp:api_content:delete"
  "dp:api_content:manage"

  # API flows
  "dp:api_flow:create"
  "dp:api_flow:read"
  "dp:api_flow:update"
  "dp:api_flow:delete"
  "dp:api_flow:manage"

  # API keys
  "dp:api_key:create"
  "dp:api_key:read"
  "dp:api_key:update"
  "dp:api_key:revoke"
  "dp:api_key:manage"

  # applications
  "dp:application:create"
  "dp:application:read"
  "dp:application:update"
  "dp:application:delete"
  "dp:application:manage"

  # application keys
  "dp:application_key:create"
  "dp:application_key:update"
  "dp:application_key:revoke"
  "dp:application_key:manage"

  # application key mappings
  "dp:application_key_mapping:create"
  "dp:application_key_mapping:read"
  "dp:application_key_mapping:manage"

  # subscriptions
  "dp:subscription:create"
  "dp:subscription:read"
  "dp:subscription:update"
  "dp:subscription:delete"
  "dp:subscription:manage"

  # subscription plans
  "dp:subscription_plan:create"
  "dp:subscription_plan:read"
  "dp:subscription_plan:update"
  "dp:subscription_plan:delete"
  "dp:subscription_plan:manage"

  # webhook subscribers
  "dp:webhook_subscriber:create"
  "dp:webhook_subscriber:read"
  "dp:webhook_subscriber:update"
  "dp:webhook_subscriber:delete"
  "dp:webhook_subscriber:manage"

  # webhook events
  "dp:event:read"
  "dp:delivery:manage"

  # utilities
  "dp:utility:create"
  "dp:utility:manage"
)

# ── Build and POST the resource ───────────────────────────────────────────────

echo "Creating resource '${RESOURCE_NAME}' with ${#SCOPES[@]} scopes..."
echo "Resource identifier: ${RESOURCE_IDENTIFIER}"
echo ""

scopes_json=""
for scope in "${SCOPES[@]}"; do
  scopes_json="${scopes_json}{\"description\":\"\",\"displayName\":\"${scope}\",\"name\":\"${scope}\"},"
done
scopes_json="[${scopes_json%,}]"

resource_payload="{\"identifier\":\"${RESOURCE_IDENTIFIER}\",\"name\":\"${RESOURCE_NAME}\",\"requiresAuthorization\":true,\"scopes\":${scopes_json}}"

http_status=$(curl -s -o /tmp/dp_resource_resp.json -w "%{http_code}" \
  -X POST "${BASE_URL}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Accept: application/json" \
  -H "Content-Type: application/json" \
  --data-raw "${resource_payload}")

if [[ "$http_status" == "200" || "$http_status" == "201" ]]; then
  echo "Resource created (HTTP ${http_status})"
  cat /tmp/dp_resource_resp.json
  echo ""
  RESOURCE_ID=$(grep -o '"id":"[^"]*"' /tmp/dp_resource_resp.json | head -1 | cut -d'"' -f4)
  echo "Resource ID: ${RESOURCE_ID}"
elif [[ "$http_status" == "409" ]]; then
  echo "Resource already exists (HTTP 409). Fetching existing resource ID..."
  ENCODED_ID=$(python3 -c "import urllib.parse; print(urllib.parse.quote('${RESOURCE_IDENTIFIER}', safe=''))")
  existing_resp=$(curl -s -w "\n%{http_code}" \
    -X GET "${BASE_URL}?identifier=${ENCODED_ID}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Accept: application/json")
  existing_status=$(echo "$existing_resp" | tail -n1)
  existing_body=$(echo "$existing_resp" | sed '$d')
  if [[ "$existing_status" != "200" ]]; then
    echo "Failed to fetch existing resource (HTTP ${existing_status}): ${existing_body}" >&2
    exit 1
  fi
  RESOURCE_ID=$(echo "$existing_body" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
  if [[ -z "$RESOURCE_ID" ]]; then
    echo "Error: could not parse resource ID" >&2
    exit 1
  fi
  echo "Existing Resource ID: ${RESOURCE_ID}"
else
  echo "Resource creation FAILED (HTTP ${http_status})"
  cat /tmp/dp_resource_resp.json
  echo ""
  exit 1
fi

echo ""
echo "Done. Next steps:"
echo "  1. In Asgardeo, open the API Portal web application."
echo "  2. Add the '${RESOURCE_NAME}' API resource (ID: ${RESOURCE_ID})."
echo "  3. Create a role (e.g. dp_admin) and assign the dp:* scopes."
echo "  4. Assign that role to users in your Asgardeo organization."
