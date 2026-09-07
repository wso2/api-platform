#!/usr/bin/env bash
# Shared helpers sourced by setup.sh / demo.sh / teardown.sh

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
BLUE=$'\033[0;34m'
BOLD=$'\033[1m'
DIM=$'\033[2m'
NC=$'\033[0m'

CONTAINER_NAME="mcp-poison-evil-server"
EVIL_SERVER_PORT="${EVIL_SERVER_PORT:-8089}"
EVIL_SERVER_URL="${EVIL_SERVER_URL:-http://localhost:${EVIL_SERVER_PORT}/mcp}"

log_header() { printf "\n%s%s== %s ==%s\n" "$BOLD" "$BLUE" "$1" "$NC"; }
log_info()   { printf "%s%s%s %s\n" "$BLUE" "➜" "$NC" "$1"; }
log_ok()     { printf "%s%s%s %s\n" "$GREEN" "✔" "$NC" "$1"; }
log_warn()   { printf "%s%s%s %s\n" "$YELLOW" "⚠" "$NC" "$1"; }
log_err()    { printf "%s%s%s %s\n" "$RED" "✘" "$NC" "$1" >&2; }

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log_err "Required command '$1' not found. Please install it and re-run."
    exit 1
  fi
}

# mcp_request <url> <json-body> [bearer-token]
# Minimal JSON-RPC-over-HTTP POST helper. Not a spec-complete MCP transport
# (no session negotiation) -- it's just enough to demonstrate tools/list
# filtering for this demo.
mcp_request() {
  local url="$1" body="$2" token="${3:-}"
  local -a headers=(-H "Content-Type: application/json" -H "Accept: application/json, text/event-stream")
  local cfg="" out status meta http_code content_type

  if [[ -n "$token" ]]; then
    # Pass the bearer token via a 0600 curl config file instead of -H, so it
    # never appears in `ps`/`/proc/<pid>/cmdline` output while the request is
    # in flight. Removed again right after the request completes.
    cfg=$(mktemp) || { log_err "mcp_request: failed to create temp file for auth header"; return 1; }
    chmod 600 "$cfg"
    printf 'header = "Authorization: Bearer %s"\n' "$token" > "$cfg"
    headers+=(--config "$cfg")
  fi

  out=$(mktemp)
  meta=$(curl -sS --max-time 10 -X POST "$url" "${headers[@]}" -d "$body" \
    -o "$out" -w '%{http_code} %{content_type}')
  status=$?
  [[ -n "$cfg" ]] && rm -f "$cfg"
  if [[ $status -ne 0 ]]; then
    rm -f "$out"
    return $status
  fi

  http_code="${meta%% *}"
  content_type="${meta#* }"

  # This helper is not a spec-complete MCP transport (see header comment) --
  # it only ever speaks plain application/json responses, so a non-2xx status
  # or an SSE/other content type is rejected here rather than handed to jq,
  # which would otherwise silently parse as "no tools" further downstream.
  if [[ "$http_code" -lt 200 || "$http_code" -ge 300 ]]; then
    log_err "mcp_request: $url returned HTTP $http_code"
    rm -f "$out"
    return 22
  fi
  # A notification (e.g. notifications/initialized) legitimately gets back an
  # empty 202 with no Content-Type -- only enforce the content-type check when
  # there's an actual body a caller might hand to jq.
  if [[ -s "$out" && "$content_type" != application/json* ]]; then
    log_err "mcp_request: $url returned unsupported content type '${content_type:-<none>}' (expected application/json)"
    rm -f "$out"
    return 22
  fi

  cat "$out"
  rm -f "$out"
}
