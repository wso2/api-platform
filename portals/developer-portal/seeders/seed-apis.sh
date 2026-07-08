#!/bin/bash
set -euo pipefail

BASE_URL="${DEVPORTAL_URL:-https://localhost:3000}"
PLATFORM_API_URL="${PLATFORM_API_URL:-https://localhost:9243}"
CREDENTIALS="${DEVPORTAL_CREDENTIALS:-admin:admin}"
ORG_HANDLE="${ORG_HANDLE:-default}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
APIS_DIR="$SCRIPT_DIR/../samples/apis"

for cmd in jq zip; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "Error: $cmd is required but not installed"
        exit 1
    fi
done

# Acquire bearer token from Platform API
PLATFORM_USER="${CREDENTIALS%%:*}"
PLATFORM_PASS="${CREDENTIALS#*:}"

TOKEN=$(curl -sk -X POST "$PLATFORM_API_URL/api/portal/v0.9/auth/login" \
    -d "username=$PLATFORM_USER&password=$PLATFORM_PASS" | jq -r '.token // empty')
if [ -z "$TOKEN" ]; then
    echo "Error: failed to obtain token from $PLATFORM_API_URL — check credentials and that Platform API is running"
    exit 1
fi

AUTH_HEADER="Authorization: Bearer $TOKEN"

# Resolve ORG_ID — use env var if set, otherwise discover by handle
if [ -z "${ORG_ID:-}" ]; then
    ORG_ID=$(curl -sk -H "$AUTH_HEADER" "$BASE_URL/organizations" | \
        jq -r --arg h "$ORG_HANDLE" '.[] | select(.orgHandle == $h) | .orgId // empty')
    if [ -z "$ORG_ID" ]; then
        echo "Error: organization with handle '$ORG_HANDLE' not found. Ensure the server has started with APIP_DP_ORGANIZATION_DEFAULTNAME set."
        exit 1
    fi
    echo "Resolved ORG_ID: $ORG_ID"
fi

seed_docs() {
    local api_dir="$1"
    local api_id="$2"
    local docs_dir="$api_dir/docs"

    local tmp_zip
    tmp_zip=$(mktemp /tmp/api-docs-XXXXXX)
    rm -f "$tmp_zip"
    tmp_zip="${tmp_zip}.zip"

    (cd "$(dirname "${api_dir%/}")" && zip -qr "$tmp_zip" "$(basename "${api_dir%/}")/docs/")

    local response http_code body
    response=$(curl -sk -X POST \
        "$BASE_URL/o/$ORG_ID/devportal/v1/apis/$api_id/content" \
        -H "$AUTH_HEADER" \
        -F "apiContent=@$tmp_zip;type=application/zip" \
        -w "\n%{http_code}")
    http_code=$(echo "$response" | tail -1)
    body=$(echo "$response" | sed '$d')

    rm -f "$tmp_zip"

    if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ]; then
        echo "  docs OK ($http_code)"
    else
        echo "  docs FAILED ($http_code): $body"
    fi
}

seed_api() {
    local api_dir="$1"
    local api_name
    api_name=$(basename "$api_dir")

    local api_yaml="$api_dir/api.yaml"

    if [ ! -f "$api_yaml" ]; then
        echo "  skipping $api_name: no api.yaml found"
        return
    fi

    echo "Seeding: $api_name"

    # Find definition.* file (definition.yaml, definition.yml, definition.graphql, etc.)
    local definition
    definition=$(compgen -G "$api_dir/definition.*" 2>/dev/null | head -1)

    local curl_args=(-sk -X POST \
        "$BASE_URL/o/$ORG_ID/devportal/v1/apis" \
        -H "$AUTH_HEADER" \
        -F "api=@$api_yaml;type=application/yaml")

    if [ -n "$definition" ]; then
        local ext="${definition##*.}"
        if [ "$ext" = "graphql" ]; then
            curl_args+=(-F "schemaDefinition=@$definition;type=application/octet-stream")
        else
            curl_args+=(-F "apiDefinition=@$definition;type=application/octet-stream")
        fi
    fi

    local response http_code body api_id
    response=$(curl "${curl_args[@]}" -w "\n%{http_code}")
    http_code=$(echo "$response" | tail -1)
    body=$(echo "$response" | sed '$d')

    if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ]; then
        api_id=$(echo "$body" | jq -r '.apiID')
        echo "  OK ($http_code) — apiID: $api_id"

        if [ -d "$api_dir/docs" ] && [ "$api_id" != "null" ]; then
            seed_docs "$api_dir" "$api_id"
        fi
    elif [ "$http_code" -eq 409 ]; then
        echo "  already exists, skipping"
    else
        echo "  FAILED ($http_code): $body"
    fi
}

for api_dir in "$APIS_DIR"/*/; do
    [ -d "$api_dir" ] && seed_api "$api_dir"
done
