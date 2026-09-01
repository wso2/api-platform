#!/usr/bin/env bash
# Shared helpers sourced by setup.sh / publish.sh / discover.sh / teardown.sh

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
BLUE=$'\033[0;34m'
BOLD=$'\033[1m'
NC=$'\033[0m'

PLATFORM_API_URL="${PLATFORM_API_URL:-https://localhost:9243}"
API_PORTAL_URL="${API_PORTAL_URL:-https://localhost:9543}"
# The whole API Portal app (UI pages, Management API, MCP Registry API) is
# mounted under this fixed /api-portal prefix -- see api-portal's own
# src/utils/constants.js (PORTAL_BASE_PATH). Only /health sits outside it.
API_PORTAL_BASE_URL="${API_PORTAL_URL}/api-portal"
ORG_HANDLE="${ORG_HANDLE:-default}"
VIEW_NAME="${VIEW_NAME:-default}"
# The self-signed TLS cert setup.sh generates for Platform API + API Portal.
# Passed to curl via --cacert instead of -k/--insecure, so requests verify
# against this specific cert (which covers "localhost") rather than skipping
# certificate validation altogether.
CA_BUNDLE="${SCRIPT_DIR}/resources/certificates/cert.pem"

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

# bcrypt isn't in openssl; use htpasswd when available, else a throwaway
# httpd container, so docker isn't a hard requirement on hosts that already
# have apache2-utils/httpd-tools installed.
bcrypt_hash() {
  local username="$1" password="$2"
  if command -v htpasswd >/dev/null 2>&1; then
    printf '%s' "$password" | htpasswd -niB -C 12 "$username" | cut -d: -f2 | tr -d '\r\n'
  elif command -v docker >/dev/null 2>&1; then
    docker run --rm httpd:2.4-alpine htpasswd -nbBC 12 "$username" "$password" | cut -d: -f2 | tr -d '\r\n'
  else
    log_err "need either htpasswd (apache2-utils / httpd-tools) or docker to bcrypt-hash the admin password."
    exit 1
  fi
}

# wait_for_health <name> <curl-args...> -- polls until the curl call succeeds
# (exit 0) or times out after ~60s.
wait_for_health() {
  local name="$1"; shift
  for _ in $(seq 1 30); do
    if curl -sS -o /dev/null "$@" 2>/dev/null; then
      return 0
    fi
    sleep 2
  done
  log_err "$name did not become healthy in time."
  return 1
}

# Logs in against the Platform API's file-based auth and prints the JWT on
# stdout. Reads ADMIN_USERNAME / ADMIN_PASSWORD from .env (written by setup.sh).
platform_api_login() {
  local username="$1" password="$2"
  local response
  response=$(curl -sS --cacert "$CA_BUNDLE" -X POST "${PLATFORM_API_URL}/api/portal/v0.9/auth/login" \
    -d "username=${username}" -d "password=${password}")
  local token
  token=$(printf '%s' "$response" | jq -r '.token // empty')
  if [[ -z "$token" ]]; then
    log_err "Login failed against ${PLATFORM_API_URL}: ${response}"
    return 1
  fi
  printf '%s' "$token"
}
