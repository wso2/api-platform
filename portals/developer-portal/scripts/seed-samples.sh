#!/bin/bash

# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

# Deploys the bundled sample APIs and MCP servers into a running Developer
# Portal, entirely through its public REST API — no in-process seeding logic
# ships in the application itself.
#
# Prerequisites: ./scripts/setup.sh has been run and `docker compose up` is running.
#
# Usage (from the project root, or the standalone distribution zip — same
# layout in both):
#   ./scripts/seed-samples.sh
#
# ADMIN_USERNAME / ADMIN_PASSWORD environment variables skip the interactive
# credential prompt (used by CI). DEVPORTAL_URL / PLATFORM_API_URL override
# the default local URLs.
#
# Safe to re-run: entries that already exist (matched by name + version) are
# skipped, not duplicated.

set -euo pipefail

THIS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Same layout-detection approach as setup.sh — this script is copied verbatim
# into the distribution zip's scripts/ directory (see Makefile's dist target),
# and both layouts put it one level below docker-compose.yaml.
if [ -f "$THIS_DIR/../docker-compose.yaml" ]; then
    ROOT_DIR="$(cd "$THIS_DIR/.." && pwd)"
elif [ -f "$THIS_DIR/docker-compose.yaml" ]; then
    ROOT_DIR="$THIS_DIR"
else
    echo "[seed-samples] ERROR: could not find docker-compose.yaml next to this script or its parent directory." >&2
    echo "[seed-samples]        Run this as ./scripts/seed-samples.sh from the project root or the distribution zip." >&2
    exit 1
fi

# The distribution zip ships samples under resources/samples/; the source
# repo keeps them at samples/ (see Makefile's dist target for the copy).
if [ -d "$ROOT_DIR/resources/samples" ]; then
    SAMPLES_DIR="$ROOT_DIR/resources/samples"
elif [ -d "$ROOT_DIR/samples" ]; then
    SAMPLES_DIR="$ROOT_DIR/samples"
else
    echo "[seed-samples] ERROR: no samples directory found (looked for resources/samples and samples next to $ROOT_DIR)." >&2
    exit 1
fi

DEVPORTAL_URL="${DEVPORTAL_URL:-https://localhost:3000}"
PLATFORM_API_URL="${PLATFORM_API_URL:-https://localhost:9243}"

log() { echo "[seed-samples] $*"; }
fail() { echo "[seed-samples] ERROR: $*" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || fail "curl is required but not found on PATH."
command -v jq   >/dev/null 2>&1 || fail "jq is required but not found on PATH."
command -v zip  >/dev/null 2>&1 || fail "zip is required but not found on PATH."

if [ -z "${ADMIN_USERNAME:-}" ] && [ -t 0 ]; then
    read -r -p "Devportal admin username: " ADMIN_USERNAME
fi
[ -n "${ADMIN_USERNAME:-}" ] || fail "an admin username is required (set ADMIN_USERNAME or run interactively)."

if [ -z "${ADMIN_PASSWORD:-}" ] && [ -t 0 ]; then
    read -r -s -p "Devportal admin password: " ADMIN_PASSWORD
    echo
fi
[ -n "${ADMIN_PASSWORD:-}" ] || fail "an admin password is required (set ADMIN_PASSWORD or run interactively)."

log "Logging in to Platform API at $PLATFORM_API_URL ..."
TOKEN=$(curl -sk -X POST "$PLATFORM_API_URL/api/portal/v0.9/auth/login" \
    -d "username=$ADMIN_USERNAME&password=$ADMIN_PASSWORD" | jq -r '.token // empty')
[ -n "$TOKEN" ] || fail "failed to obtain a token — check the credentials and that Platform API is reachable at $PLATFORM_API_URL."
AUTH_HEADER="Authorization: Bearer $TOKEN"

# Uploads sample_dir/docs/ as the content ZIP for an already-created API/MCP server.
seed_docs() {
    local sample_dir="$1" resource_path="$2"
    [ -d "$sample_dir/docs" ] || return 0

    local tmp_zip
    tmp_zip="$(mktemp /tmp/devportal-docs-XXXXXX)"
    rm -f "$tmp_zip"
    tmp_zip="${tmp_zip}.zip"
    # Wrapped in a top-level folder (named after the sample) rather than zipping
    # docs/ bare at the root — the server unwraps a single top-level directory
    # entry looking for web/ or docs/ inside it; zipping docs/ alone as that one
    # entry gets unwrapped too, so it then looks for (and fails to find) docs/docs.
    (cd "$sample_dir/.." && zip -qr "$tmp_zip" "$(basename "$sample_dir")/docs/")

    local http_code
    http_code=$(curl -sk -o /dev/null -w "%{http_code}" -X POST \
        "$DEVPORTAL_URL$resource_path/assets" \
        -H "$AUTH_HEADER" \
        -F "content=@$tmp_zip;type=application/zip")
    rm -f "$tmp_zip"

    if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ]; then
        log "  docs OK ($http_code)"
    else
        log "  docs FAILED ($http_code)"
    fi
}

# Creates one API or MCP server entry from a sample directory (api.yaml + optional
# definition.* + optional docs/), via the given collection endpoint.
seed_entry() {
    local sample_dir="$1" endpoint="$2"
    local name; name="$(basename "$sample_dir")"
    local api_yaml="$sample_dir/api.yaml"
    if [ ! -f "$api_yaml" ]; then
        log "skipping $name: no api.yaml"
        return
    fi

    local definition
    definition=$(compgen -G "$sample_dir/definition.*" 2>/dev/null | head -1 || true)

    log "Seeding: $name"
    local curl_args=(-sk -X POST "$DEVPORTAL_URL/api/v0.9/$endpoint" \
        -H "$AUTH_HEADER" \
        -F "metadata=@$api_yaml;type=application/yaml")
    if [ -n "$definition" ]; then
        curl_args+=(-F "definition=@$definition;type=application/octet-stream")
    fi

    local response http_code body id
    response=$(curl "${curl_args[@]}" -w "\n%{http_code}")
    http_code=$(echo "$response" | tail -1)
    body=$(echo "$response" | sed '$d')

    if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ]; then
        id=$(echo "$body" | jq -r '.id // empty')
        log "  OK ($http_code) — id: $id"
        [ -n "$id" ] && seed_docs "$sample_dir" "/api/v0.9/$endpoint/$id"
    elif [ "$http_code" -eq 409 ]; then
        log "  already exists, skipping"
    else
        log "  FAILED ($http_code): $body"
    fi
}

if [ -d "$SAMPLES_DIR/apis" ]; then
    log "Seeding sample APIs from $SAMPLES_DIR/apis ..."
    for dir in "$SAMPLES_DIR"/apis/*/; do
        [ -d "$dir" ] && seed_entry "${dir%/}" "apis"
    done
fi

if [ -d "$SAMPLES_DIR/mcps" ]; then
    log "Seeding sample MCP servers from $SAMPLES_DIR/mcps ..."
    for dir in "$SAMPLES_DIR"/mcps/*/; do
        [ -d "$dir" ] && seed_entry "${dir%/}" "mcp-servers"
    done
fi

log "Done."
