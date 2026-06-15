#!/bin/bash
set -euo pipefail

BASE_URL="${DEVPORTAL_URL:-https://localhost:3000}"
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

# Resolve ORG_ID — use env var if set, otherwise discover by handle
if [ -z "${ORG_ID:-}" ]; then
    ORG_ID=$(curl -sk -u "$CREDENTIALS" "$BASE_URL/organizations" | \
        jq -r --arg h "$ORG_HANDLE" '.[] | select(.orgHandle == $h) | .orgID // empty')
    if [ -z "$ORG_ID" ]; then
        echo "Error: organization with handle '$ORG_HANDLE' not found. Ensure the server has started with DP_DEFAULTORGNAME set."
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
        -u "$CREDENTIALS" \
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
    local definition="$api_dir/openapi.yaml"

    if [ ! -f "$api_yaml" ]; then
        echo "  skipping $api_name: no api.yaml found"
        return
    fi

    echo "Seeding: $api_name"

    local curl_args=(-sk -X POST \
        "$BASE_URL/o/$ORG_ID/devportal/v1/apis" \
        -u "$CREDENTIALS" \
        -F "api=@$api_yaml;type=application/yaml")

    if [ -f "$definition" ]; then
        curl_args+=(-F "apiDefinition=@$definition;type=application/yaml")
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
