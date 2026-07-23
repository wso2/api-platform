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

# Colors/symbols only when writing to an interactive terminal (respects the
# NO_COLOR convention: https://no-color.org/) — a piped/CI log gets plain
# ASCII instead of ANSI escapes and unicode glyphs.
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
    C_GREEN=$'\033[32m'; C_RED=$'\033[31m'; C_YELLOW=$'\033[33m'
    C_DIM=$'\033[2m'; C_BOLD=$'\033[1m'; C_RESET=$'\033[0m'
    SYM_OK="✓"; SYM_FAIL="✗"; SYM_SKIP="•"
else
    C_GREEN=""; C_RED=""; C_YELLOW=""; C_DIM=""; C_BOLD=""; C_RESET=""
    SYM_OK="OK"; SYM_FAIL="FAIL"; SYM_SKIP="-"
fi

log() { echo "${C_DIM}[seed-samples]${C_RESET} $*"; }
fail() { echo "${C_RED}[seed-samples] ERROR:${C_RESET} $*" >&2; exit 1; }

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

SECONDS=0
API_CREATED=0; API_SKIPPED=0; API_FAILED=0
MCP_CREATED=0; MCP_SKIPPED=0; MCP_FAILED=0

# Uploads sample_dir/docs/ as the content ZIP for an already-created API/MCP server.
# Result is left in DOCS_RESULT (a ready-to-print fragment) and DOCS_FAILED (0/1)
# rather than printed directly, so seed_entry can fold the fragment into that
# sample's single summary line and tally the outcome into the right counter.
seed_docs() {
    local sample_dir="$1" resource_path="$2"
    DOCS_RESULT=""
    DOCS_FAILED=0
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
        DOCS_RESULT="${C_GREEN}docs ${SYM_OK}${C_RESET}"
    else
        DOCS_RESULT="${C_RED}docs ${SYM_FAIL} (${http_code})${C_RESET}"
        DOCS_FAILED=1
    fi
}

# Bumps the API_* or MCP_* counter matching $1 (an endpoint value) by one,
# for the field named by $2 (CREATED/SKIPPED/FAILED). Avoids bash namerefs
# (`local -n`, needs bash 4.3+) so this still runs under macOS's stock
# bash 3.2 — this script has no other bash-version dependency, so it
# shouldn't gain one just for tallying counters.
bump_counter() {
    local endpoint="$1" field="$2"
    if [ "$endpoint" = "mcp-servers" ]; then
        case "$field" in
            CREATED) MCP_CREATED=$((MCP_CREATED + 1)) ;;
            SKIPPED) MCP_SKIPPED=$((MCP_SKIPPED + 1)) ;;
            FAILED)  MCP_FAILED=$((MCP_FAILED + 1)) ;;
        esac
    else
        case "$field" in
            CREATED) API_CREATED=$((API_CREATED + 1)) ;;
            SKIPPED) API_SKIPPED=$((API_SKIPPED + 1)) ;;
            FAILED)  API_FAILED=$((API_FAILED + 1)) ;;
        esac
    fi
}

# Creates one API or MCP server entry from a sample directory (api.yaml + optional
# definition.* + optional docs/), via the given collection endpoint. Prints exactly
# one summary line per sample and tallies the outcome into the *_CREATED/SKIPPED/
# FAILED counters (bucketed by $endpoint) for the closing summary.
seed_entry() {
    local sample_dir="$1" endpoint="$2"
    local name; name="$(basename "$sample_dir")"
    local api_yaml="$sample_dir/api.yaml"

    if [ ! -f "$api_yaml" ]; then
        printf "  ${C_YELLOW}%s${C_RESET} %-28s ${C_DIM}(no api.yaml, skipped)${C_RESET}\n" "$SYM_SKIP" "$name"
        bump_counter "$endpoint" SKIPPED
        return
    fi

    local definition
    definition=$(compgen -G "$sample_dir/definition.*" 2>/dev/null | head -1 || true)

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
        DOCS_RESULT=""
        DOCS_FAILED=0
        [ -n "$id" ] && seed_docs "$sample_dir" "/api/v0.9/$endpoint/$id"
        if [ "$DOCS_FAILED" -eq 1 ]; then
            # Entry itself was created, but its docs upload failed — surface this
            # as a failure (red symbol, FAILED tally) rather than a clean success,
            # so the closing summary's failed count isn't silently undercounted.
            printf "  ${C_RED}%s${C_RESET} %-28s ${C_DIM}(id: %s, %s${C_DIM})${C_RESET}\n" "$SYM_FAIL" "$name" "$id" "$DOCS_RESULT"
            bump_counter "$endpoint" FAILED
        elif [ -n "$DOCS_RESULT" ]; then
            printf "  ${C_GREEN}%s${C_RESET} %-28s ${C_DIM}(id: %s, %s${C_DIM})${C_RESET}\n" "$SYM_OK" "$name" "$id" "$DOCS_RESULT"
            bump_counter "$endpoint" CREATED
        else
            printf "  ${C_GREEN}%s${C_RESET} %-28s ${C_DIM}(id: %s)${C_RESET}\n" "$SYM_OK" "$name" "$id"
            bump_counter "$endpoint" CREATED
        fi
    elif [ "$http_code" -eq 409 ]; then
        printf "  ${C_YELLOW}%s${C_RESET} %-28s ${C_DIM}(already exists)${C_RESET}\n" "$SYM_SKIP" "$name"
        bump_counter "$endpoint" SKIPPED
    else
        local short_err
        short_err=$(echo "$body" | jq -r '.error // .message // empty' 2>/dev/null)
        [ -n "$short_err" ] || short_err="$body"
        printf "  ${C_RED}%s${C_RESET} %-28s ${C_RED}(%s: %s)${C_RESET}\n" "$SYM_FAIL" "$name" "$http_code" "$short_err"
        bump_counter "$endpoint" FAILED
    fi
}

if [ -d "$SAMPLES_DIR/apis" ]; then
    echo
    echo "${C_BOLD}Seeding APIs${C_RESET}"
    for dir in "$SAMPLES_DIR"/apis/*/; do
        [ -d "$dir" ] && seed_entry "${dir%/}" "apis"
    done
fi

if [ -d "$SAMPLES_DIR/mcps" ]; then
    echo
    echo "${C_BOLD}Seeding MCP servers${C_RESET}"
    for dir in "$SAMPLES_DIR"/mcps/*/; do
        [ -d "$dir" ] && seed_entry "${dir%/}" "mcp-servers"
    done
fi

TOTAL_CREATED=$((API_CREATED + MCP_CREATED))
TOTAL_SKIPPED=$((API_SKIPPED + MCP_SKIPPED))
TOTAL_FAILED=$((API_FAILED + MCP_FAILED))

echo
if [ "$TOTAL_FAILED" -gt 0 ]; then
    STATUS_COLOR="$C_RED"
else
    STATUS_COLOR="$C_GREEN"
fi
API_WORD="APIs"; [ "$API_CREATED" -eq 1 ] && API_WORD="API"
MCP_WORD="MCP servers"; [ "$MCP_CREATED" -eq 1 ] && MCP_WORD="MCP server"
echo "${STATUS_COLOR}Done${C_RESET} — ${C_BOLD}${TOTAL_CREATED} seeded${C_RESET} (${API_CREATED} ${API_WORD}, ${MCP_CREATED} ${MCP_WORD}), ${TOTAL_SKIPPED} skipped, ${TOTAL_FAILED} failed in ${SECONDS}s"
