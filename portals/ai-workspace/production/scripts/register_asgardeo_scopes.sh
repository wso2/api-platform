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
# Fetches a token via client credentials then creates an Asgardeo API resource
# with all platform scopes (as defined in openapi.yaml securitySchemes) in a single POST request.

# 1. Create a OICD Application and name it as system application.
# 2. Add API Resource Management API  and Application Management API API resource to the system app created.
#
# Usage:
#   ./register_asgardeo_scopes.sh
#
# Override defaults via env vars:
#   ASGARDEO_TENANT              (default: apiplatformtesting)
#   ASGARDEO_CLIENT_ID           (default: lBfbZTLzjV0a9S2G6RarxBk4VyAa)
#   ASGARDEO_CLIENT_SECRET       (default: 85gQ5VlfIGCUb5LoyLD0DAx17hT6QNMwjGPnOOx5HMca)
#   ASGARDEO_RESOURCE_IDENTIFIER (default: https://localhost:9243)
#   ASGARDEO_RESOURCE_NAME       (default: API Platform Resources)

set -euo pipefail

TENANT="${ASGARDEO_TENANT:-abcorgdefault}"
CLIENT_ID="${ASGARDEO_CLIENT_ID:-keJXeZPxHWfZAoYExc4C9xNZLp8a}"
CLIENT_SECRET="${ASGARDEO_CLIENT_SECRET:-Aqx9ZlTOWCW4kTzM_vnOoJnMGH0I2VDNby3M2BfBpQEa}"
RESOURCE_IDENTIFIER="${ASGARDEO_RESOURCE_IDENTIFIER:-https://localhost:9243}"
RESOURCE_NAME="${ASGARDEO_RESOURCE_NAME:-API Platform Resources}"
TOKEN_EP="https://api.asgardeo.io/t/${TENANT}/oauth2/token"
BASE_URL="https://api.asgardeo.io/t/${TENANT}/api/server/v1/api-resources"

# ── Fetch token via client credentials ────────────────────────────────────────

MGMT_SCOPES="internal_api_resource_create internal_api_resource_delete internal_api_resource_update internal_api_resource_view internal_application_business_api_update internal_application_internal_api_update internal_application_mgt_client_secret_create internal_application_mgt_client_secret_view internal_application_mgt_create internal_application_mgt_delete internal_application_mgt_update internal_application_mgt_view internal_org_api_resource_view internal_role_mgt_update internal_role_mgt_view"

TOKEN_CURL_CMD="curl -s -w \"\n%{http_code}\" -X POST \"${TOKEN_EP}\" \
  -H \"Content-Type: application/x-www-form-urlencoded\" \
  -u \"${CLIENT_ID}:${CLIENT_SECRET}\" \
  --data-urlencode \"grant_type=client_credentials\" \
  --data-urlencode \"scope=${MGMT_SCOPES}\""

echo "Fetching access token..."
echo "Command: ${TOKEN_CURL_CMD}"
echo ""

token_resp=$(curl -s -w "\n%{http_code}" \
  -X POST "${TOKEN_EP}" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -u "${CLIENT_ID}:${CLIENT_SECRET}" \
  --data-urlencode "grant_type=client_credentials" \
  --data-urlencode "scope=${MGMT_SCOPES}")

# macOS-compatible: strip last line (status code) to get body
token_status=$(echo "$token_resp" | tail -n1)
token_body=$(echo "$token_resp" | sed '$d')

echo "Response (HTTP ${token_status}):"
echo "${token_body}"
echo ""

if [[ "$token_status" != "200" ]]; then
  echo "Error: token request failed (HTTP ${token_status})" >&2
  exit 1
fi

TOKEN=$(echo "$token_body" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)

if [[ -z "$TOKEN" ]]; then
  echo "Error: could not parse access_token from response" >&2
  exit 1
fi

echo "Token obtained successfully."
echo ""

# ── Scope list ────────────────────────────────────────────────────────────────

SCOPES=(
  # organization
  "ap:organization:manage"
  "ap:organization:read"
  "ap:organization:subscription:read"

  # project
  "ap:project:create"
  "ap:project:read"
  "ap:project:update"
  "ap:project:delete"
  "ap:project:manage"

  # rest_api
  "ap:rest_api:create"
  "ap:rest_api:read"
  "ap:rest_api:update"
  "ap:rest_api:delete"
  "ap:rest_api:manage"
  "ap:rest_api:import"
  "ap:rest_api:deployment:create"
  "ap:rest_api:deployment:read"
  "ap:rest_api:deployment:manage"
  "ap:rest_api:deployment:delete"
  "ap:rest_api:deployment:restore"
  "ap:rest_api:deployment:undeploy"
  "ap:rest_api:api_key:create"
  "ap:rest_api:api_key:read"
  "ap:rest_api:api_key:update"
  "ap:rest_api:api_key:delete"
  "ap:rest_api:api_key:manage"
  "ap:rest_api:gateway:create"
  "ap:rest_api:gateway:read"
  "ap:rest_api:gateway:manage"
  "ap:rest_api:publication:create"
  "ap:rest_api:publication:read"
  "ap:rest_api:publication:delete"

  # application
  "ap:application:create"
  "ap:application:read"
  "ap:application:update"
  "ap:application:delete"
  "ap:application:manage"
  "ap:application:api_key:create"
  "ap:application:api_key:read"
  "ap:application:api_key:delete"
  "ap:application:api_key:manage"
  "ap:application:associations:create"
  "ap:application:associations:read"
  "ap:application:associations:delete"
  "ap:application:associations:manage"
  "ap:application:associations:api_key:read"

  # gateway
  "ap:gateway:create"
  "ap:gateway:read"
  "ap:gateway:update"
  "ap:gateway:delete"
  "ap:gateway:manage"
  "ap:gateway:artifacts:read"
  "ap:gateway:manifest:read"
  "ap:gateway:token:create"
  "ap:gateway:token:read"
  "ap:gateway:token:delete"
  "ap:gateway:token:manage"

  # gateway_custom_policy
  "ap:gateway_custom_policy:create"
  "ap:gateway_custom_policy:read"
  "ap:gateway_custom_policy:delete"
  "ap:gateway_custom_policy:manage"

  # devportal
  "ap:devportal:create"
  "ap:devportal:read"
  "ap:devportal:update"
  "ap:devportal:delete"
  "ap:devportal:manage"

  # git
  "ap:git:read"

  # llm_template
  "ap:llm_template:create"
  "ap:llm_template:read"
  "ap:llm_template:update"
  "ap:llm_template:delete"
  "ap:llm_template:manage"

  # llm_provider
  "ap:llm_provider:create"
  "ap:llm_provider:read"
  "ap:llm_provider:update"
  "ap:llm_provider:delete"
  "ap:llm_provider:manage"
  "ap:llm_provider:deployment:create"
  "ap:llm_provider:deployment:read"
  "ap:llm_provider:deployment:manage"
  "ap:llm_provider:deployment:delete"
  "ap:llm_provider:deployment:restore"
  "ap:llm_provider:deployment:undeploy"
  "ap:llm_provider:api_key:create"
  "ap:llm_provider:api_key:read"
  "ap:llm_provider:api_key:delete"
  "ap:llm_provider:api_key:manage"

  # llm_proxy
  "ap:llm_proxy:create"
  "ap:llm_proxy:read"
  "ap:llm_proxy:update"
  "ap:llm_proxy:delete"
  "ap:llm_proxy:manage"
  "ap:llm_proxy:deployment:create"
  "ap:llm_proxy:deployment:read"
  "ap:llm_proxy:deployment:manage"
  "ap:llm_proxy:deployment:delete"
  "ap:llm_proxy:deployment:restore"
  "ap:llm_proxy:deployment:undeploy"
  "ap:llm_proxy:api_key:create"
  "ap:llm_proxy:api_key:read"
  "ap:llm_proxy:api_key:delete"
  "ap:llm_proxy:api_key:manage"

  # mcp_proxy
  "ap:mcp_proxy:create"
  "ap:mcp_proxy:read"
  "ap:mcp_proxy:update"
  "ap:mcp_proxy:delete"
  "ap:mcp_proxy:manage"
  "ap:mcp_proxy:deployment:create"
  "ap:mcp_proxy:deployment:read"
  "ap:mcp_proxy:deployment:manage"
  "ap:mcp_proxy:deployment:delete"
  "ap:mcp_proxy:deployment:restore"
  "ap:mcp_proxy:deployment:undeploy"

  # websub_api
  "ap:websub_api:create"
  "ap:websub_api:read"
  "ap:websub_api:update"
  "ap:websub_api:delete"
  "ap:websub_api:manage"
  "ap:websub_api:deployment:create"
  "ap:websub_api:deployment:read"
  "ap:websub_api:deployment:manage"
  "ap:websub_api:deployment:delete"
  "ap:websub_api:deployment:restore"
  "ap:websub_api:deployment:undeploy"
  "ap:websub_api:api_key:create"
  "ap:websub_api:api_key:update"
  "ap:websub_api:api_key:delete"
  "ap:websub_api:api_key:manage"
  "ap:websub_api:publication:create"
  "ap:websub_api:publication:read"
  "ap:websub_api:publication:delete"

  # webbroker_api
  "ap:webbroker_api:create"
  "ap:webbroker_api:read"
  "ap:webbroker_api:update"
  "ap:webbroker_api:delete"
  "ap:webbroker_api:manage"
  "ap:webbroker_api:deployment:create"
  "ap:webbroker_api:deployment:read"
  "ap:webbroker_api:deployment:manage"
  "ap:webbroker_api:deployment:delete"
  "ap:webbroker_api:deployment:restore"
  "ap:webbroker_api:deployment:undeploy"
  "ap:webbroker_api:api_key:create"
  "ap:webbroker_api:api_key:update"
  "ap:webbroker_api:api_key:delete"
  "ap:webbroker_api:api_key:manage"
  "ap:webbroker_api:publication:create"
  "ap:webbroker_api:publication:read"
  "ap:webbroker_api:publication:delete"

  # subscription
  "ap:subscription:create"
  "ap:subscription:read"
  "ap:subscription:update"
  "ap:subscription:delete"
  "ap:subscription:manage"

  # subscription_plan
  "ap:subscription_plan:create"
  "ap:subscription_plan:read"
  "ap:subscription_plan:update"
  "ap:subscription_plan:delete"
  "ap:subscription_plan:manage"
)

# Derive a human-readable display name: "ap:llm_proxy:manage" -> "llm proxy manage"
display_name() {
  local scope="$1"
  local resource action
  resource=$(echo "$scope" | cut -d: -f2)
  action=$(echo "$scope" | cut -d: -f3)
  echo "${resource//_/ } ${action}"
}

echo "Creating resource '${RESOURCE_NAME}' with ${#SCOPES[@]} scopes"
echo "Tenant: ${TENANT}"
echo "---"

# Build scopes JSON array from SCOPES
scopes_json=""
for scope in "${SCOPES[@]}"; do
  name="$(display_name "$scope")"
  scopes_json="${scopes_json}{\"description\":\"\",\"displayName\":\"${scope}\",\"name\":\"${scope}\"},"
done
scopes_json="[${scopes_json%,}]"

resource_payload="{\"identifier\":\"${RESOURCE_IDENTIFIER}\",\"name\":\"${RESOURCE_NAME}\",\"requiresAuthorization\":true,\"scopes\":${scopes_json}}"

http_status=$(curl -s -o /tmp/asgardeo_resource_resp.json -w "%{http_code}" \
  -X POST "${BASE_URL}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Accept: application/json" \
  -H "Content-Type: application/json" \
  --data-raw "${resource_payload}")

if [[ "$http_status" == "200" || "$http_status" == "201" ]]; then
  echo "Resource created OK (HTTP ${http_status})"
  cat /tmp/asgardeo_resource_resp.json
  echo ""
  RESOURCE_ID=$(grep -o '"id":"[^"]*"' /tmp/asgardeo_resource_resp.json | head -1 | cut -d'"' -f4)
  echo "Resource ID: ${RESOURCE_ID}"
elif [[ "$http_status" == "409" ]]; then
  echo "Resource already exists (HTTP 409), fetching existing resource ID..."
  existing_resp=$(curl -s -w "\n%{http_code}" \
    -X GET "${BASE_URL}?identifier=$(python3 -c "import urllib.parse; print(urllib.parse.quote('${RESOURCE_IDENTIFIER}', safe=''))")" \
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
    echo "Error: could not parse resource ID from existing resource response" >&2
    echo "$existing_body" >&2
    exit 1
  fi
  echo "Existing Resource ID: ${RESOURCE_ID}"
else
  echo "Resource creation FAILED (HTTP ${http_status})"
  cat /tmp/asgardeo_resource_resp.json
  echo ""
  exit 1
fi
