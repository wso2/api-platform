#!/bin/bash
set -euo pipefail

BASE_URL="${DEVPORTAL_URL:-https://localhost:3000}"
ORG_ID="${ORG_ID:-1ba42a09-45c0-40f8-a1bf-e4aa7cde1575}"
CREDENTIALS="${DEVPORTAL_CREDENTIALS:-admin:admin}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
APIS_DIR="$SCRIPT_DIR/../samples/apis"

seed_api() {
    local api_dir="$1"
    local api_name
    api_name=$(basename "$api_dir")

    local api_yaml="$api_dir/api.yaml"
    local definition="$api_dir/definition.yml"

    if [ ! -f "$api_yaml" ]; then
        echo "  skipping $api_name: no api.yaml found"
        return
    fi

    echo "Seeding: $api_name"

    local curl_args=(-sk -X POST \
        "$BASE_URL/devportal/organizations/$ORG_ID/apis" \
        -u "$CREDENTIALS" \
        -F "api=@$api_yaml;type=application/yaml")

    if [ -f "$definition" ]; then
        curl_args+=(-F "apiDefinition=@$definition;type=application/yaml")
    fi

    local response http_code body
    response=$(curl "${curl_args[@]}" -w "\n%{http_code}")
    http_code=$(echo "$response" | tail -1)
    body=$(echo "$response" | sed '$d')

    if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ]; then
        echo "  OK ($http_code)"
    else
        echo "  FAILED ($http_code): $body"
    fi
}

for api_dir in "$APIS_DIR"/*/; do
    [ -d "$api_dir" ] && seed_api "$api_dir"
done
