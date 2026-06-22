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
# Registers the Developer Portal API resource and all dp:* scopes in Asgardeo
# so that the devportal traditional web application can request them.
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
#   ASGARDEO_RESOURCE_IDENTIFIER API resource identifier (usually the devportal base URL)
#   ASGARDEO_RESOURCE_NAME       Display name for the API resource

set -euo pipefail

TENANT="${ASGARDEO_TENANT:-}"
CLIENT_ID="${ASGARDEO_CLIENT_ID:-}"
CLIENT_SECRET="${ASGARDEO_CLIENT_SECRET:-}"
RESOURCE_IDENTIFIER="${ASGARDEO_RESOURCE_IDENTIFIER:-https://localhost:3000}"
RESOURCE_NAME="${ASGARDEO_RESOURCE_NAME:-Developer Portal Resources}"

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
  "dp:org_read"
  "dp:org_write"
  "dp:org_delete"
  "dp:org_manage"

  # organization content
  "dp:org_content_read"
  "dp:org_content_write"
  "dp:org_content_delete"
  "dp:org_content_manage"

  # views
  "dp:view_read"
  "dp:view_write"
  "dp:view_delete"
  "dp:view_manage"

  # labels
  "dp:label_read"
  "dp:label_write"
  "dp:label_delete"
  "dp:label_manage"

  # providers
  "dp:provider_read"
  "dp:provider_write"
  "dp:provider_delete"
  "dp:provider_manage"

  # key managers
  "dp:km_read"
  "dp:km_write"
  "dp:km_delete"
  "dp:km_manage"

  # APIs
  "dp:api_read"
  "dp:api_write"
  "dp:api_delete"
  "dp:api_manage"

  # API content
  "dp:api_content_read"
  "dp:api_content_write"
  "dp:api_content_delete"
  "dp:api_content_manage"

  # API flows
  "dp:api_flow_read"
  "dp:api_flow_write"
  "dp:api_flow_delete"
  "dp:api_flow_manage"

  # API keys
  "dp:api_key_read"
  "dp:api_key_write"
  "dp:api_key_revoke"
  "dp:api_key_manage"

  # applications
  "dp:app_read"
  "dp:app_write"
  "dp:app_delete"
  "dp:app_manage"

  # application keys
  "dp:app_key_write"
  "dp:app_key_revoke"
  "dp:app_key_manage"

  # application key mappings
  "dp:app_key_mapping_read"
  "dp:app_key_mapping_write"
  "dp:app_key_mapping_manage"

  # subscriptions
  "dp:subscription_read"
  "dp:subscription_write"
  "dp:subscription_delete"
  "dp:subscription_manage"

  # subscription plans
  "dp:sub_plan_read"
  "dp:sub_plan_write"
  "dp:sub_plan_delete"
  "dp:sub_plan_manage"

  # webhook events
  "dp:event_read"
  "dp:delivery_manage"

  # utilities
  "dp:utility_write"
  "dp:utility_manage"
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
echo "  1. In Asgardeo, open the Developer Portal web application."
echo "  2. Add the '${RESOURCE_NAME}' API resource (ID: ${RESOURCE_ID})."
echo "  3. Create a role (e.g. dp_admin) and assign the dp:* scopes."
echo "  4. Assign that role to users in your Asgardeo organization."
