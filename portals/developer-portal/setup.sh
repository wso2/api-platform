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

# Quick-start setup for the Developer Portal.
#
# Generates the secrets required to run the portal locally (via Docker Compose,
# or natively with `npm start`):
#   - devportal's own encryption/session keys      (APIP_DP_SECURITY_*)
#   - the Platform API's at-rest encryption key     (APIP_CP_ENCRYPTION_KEY)
#   - a shared JWT signing key for the Platform API (APIP_CP_AUTH_JWT_SECRET_KEY,
#     written a second time as APIP_DP_PLATFORMAPI_JWTSECRET since devportal's
#     config.toml references it under its own name)
#   - a random admin password, bcrypt-hashed into AUTH_FILE_BASED_USERS
#   - a self-signed TLS certificate for devportal
#
# This is a ONE-TIME step. It never runs as part of container startup — both
# services fail closed at startup if a required secret is missing, rather than
# silently generating or accepting a weaker one. Re-running this script is
# safe: it only fills in what's missing and never overwrites an existing value.
#
# Usage:
#   ./setup.sh              (from the project root — local dev)
#   ./scripts/setup.sh      (from the standalone distribution zip)
#   docker compose up
#
# To rotate a value, delete it from api-platform.env (or delete
# resources/certificates for the TLS cert) and re-run this script.

set -euo pipefail

THIS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# This same script is copied verbatim into the distribution zip's scripts/
# directory (see Makefile's dist target), so it can't assume its own directory
# is the project root — detect which layout is in play by locating
# docker-compose.yaml, which is always a direct sibling of the real root.
if [ -f "$THIS_DIR/docker-compose.yaml" ]; then
    ROOT_DIR="$THIS_DIR"
elif [ -f "$THIS_DIR/../docker-compose.yaml" ]; then
    ROOT_DIR="$(cd "$THIS_DIR/.." && pwd)"
else
    echo "[setup] ERROR: could not find docker-compose.yaml next to this script or its parent directory." >&2
    echo "[setup]        Run this as ./setup.sh from the project root, or ./scripts/setup.sh from the distribution zip." >&2
    exit 1
fi
cd "$ROOT_DIR"

ENV_FILE="$ROOT_DIR/api-platform.env"
DEVPORTAL_CERT_DIR="$ROOT_DIR/resources/certificates"
PLATFORM_API_CONFIG="$ROOT_DIR/configs/config-platform-api.toml"

# Bind-mounted into a container running as a non-root UID: 644 (not 600) so the
# container user can read a file owned by the host user. Local single-user
# quick-start tradeoff — matches the perms the old auto-generated cert used.
CERT_FILE_MODE=644

ADMIN_USERNAME="admin"
GENERATED_PASSWORD=""

log() { echo "[setup] $*"; }
fail() { echo "[setup] ERROR: $*" >&2; exit 1; }

command -v openssl >/dev/null 2>&1 || fail "openssl is required but not found on PATH."
command -v docker  >/dev/null 2>&1 || fail "docker is required but not found on PATH (used to hash the admin password)."

touch "$ENV_FILE"

# Sets KEY=VALUE in api-platform.env, but only if KEY isn't already present
# (idempotent — never overwrites a value the user or a previous run already set).
set_env_var() {
    local key="$1" value="$2"
    if grep -q "^${key}=" "$ENV_FILE" 2>/dev/null; then
        log "  - ${key} already set in api-platform.env, leaving as-is"
    else
        printf '%s=%s\n' "$key" "$value" >> "$ENV_FILE"
        log "  - ${key} generated"
    fi
}

get_env_var() {
    grep "^${1}=" "$ENV_FILE" 2>/dev/null | head -1 | cut -d= -f2-
}

log "Generating devportal secrets into api-platform.env ..."
set_env_var "APIP_DP_SECURITY_ENCRYPTIONKEY" "$(openssl rand -hex 32)"
set_env_var "APIP_DP_SECURITY_SESSIONSECRET" "$(openssl rand -hex 32)"

log "Generating Platform API encryption key into api-platform.env ..."
set_env_var "APIP_CP_ENCRYPTION_KEY" "$(openssl rand -hex 32)"

log "Generating shared Platform API JWT signing key into api-platform.env ..."
# Written under both names it needs to reach: APIP_CP_AUTH_JWT_SECRET_KEY for the
# platform-api container's own config-platform-api.toml reference,
# APIP_DP_PLATFORMAPI_JWTSECRET for the devportal container's config.toml
# reference — same value, two names, since each config.toml reads a variable
# only under its own exact name.
if grep -q "^APIP_CP_AUTH_JWT_SECRET_KEY=" "$ENV_FILE" 2>/dev/null; then
    log "  - APIP_CP_AUTH_JWT_SECRET_KEY already set in api-platform.env, leaving as-is"
else
    printf 'APIP_CP_AUTH_JWT_SECRET_KEY=%s\n' "$(openssl rand -hex 32)" >> "$ENV_FILE"
    log "  - APIP_CP_AUTH_JWT_SECRET_KEY generated"
fi
JWT_SECRET_KEY="$(get_env_var APIP_CP_AUTH_JWT_SECRET_KEY)"
set_env_var "APIP_DP_PLATFORMAPI_JWTSECRET" "$JWT_SECRET_KEY"

log "Provisioning devportal TLS certificate ..."
mkdir -p "$DEVPORTAL_CERT_DIR"
if [ -f "$DEVPORTAL_CERT_DIR/server.crt" ] && [ -f "$DEVPORTAL_CERT_DIR/server.key" ]; then
    log "  - $DEVPORTAL_CERT_DIR already has a certificate, leaving as-is"
else
    openssl req -x509 -newkey rsa:4096 \
        -keyout "$DEVPORTAL_CERT_DIR/server.key" \
        -out    "$DEVPORTAL_CERT_DIR/server.crt" \
        -days 36500 -nodes \
        -subj "/C=US/ST=California/L=San Francisco/O=WSO2/OU=Developer Portal/CN=localhost" \
        -addext "subjectAltName=DNS:localhost,DNS:*.localhost,IP:127.0.0.1" \
        2>/dev/null
    chmod "$CERT_FILE_MODE" "$DEVPORTAL_CERT_DIR/server.key" "$DEVPORTAL_CERT_DIR/server.crt"
    log "  - self-signed certificate generated at $DEVPORTAL_CERT_DIR"
fi

log "Provisioning configs/config-platform-api.toml ..."
if [ -f "$PLATFORM_API_CONFIG" ]; then
    log "  - $PLATFORM_API_CONFIG already exists, leaving as-is"
else
    mkdir -p "$ROOT_DIR/configs"
    # Wired to api-platform.env via {{ env "..." }} tokens (never hardcode a
    # secret here) — this is the file docker-compose.yaml bind-mounts into the
    # platform-api container. It's gitignored: for a static, no-dependencies
    # starting point instead (e.g. running platform-api directly, without
    # ./setup.sh), copy configs/config-platform-api-template.toml instead.
    cat > "$PLATFORM_API_CONFIG" <<'EOF'
# Platform API configuration for the Developer Portal.
# Generated by ./setup.sh — every secret below is a {{ env "..." }} token
# resolved from api-platform.env. Not tracked in git (see .gitignore).

log_level = "INFO"   # DEBUG | INFO | WARN | ERROR
port      = "9243"

encryption_key = '{{ env "APIP_CP_ENCRYPTION_KEY" }}'

[database]
driver = "sqlite3"
path   = "/app/data/platform-api-devportal.db"

[auth.jwt]
enabled    = true
issuer     = "platform-api"
secret_key = '{{ env "APIP_CP_AUTH_JWT_SECRET_KEY" }}'

[auth.idp]
enabled = false

[auth.file_based]
enabled = true
users   = '{{ env "AUTH_FILE_BASED_USERS" }}'

[auth.file_based.organization]
id           = "default"
display_name = "Default"
region       = "us"

[tls]
cert_dir = "/app/data/certs"

[default_devportal]
enabled = false
EOF
    log "  - $PLATFORM_API_CONFIG generated"
fi

# Full-access scopes for the seeded admin user — ap:* (platform-admin) plus every
# dp:*_manage scope so it can manage every Developer Portal resource area.
ADMIN_SCOPES="ap:organization:manage ap:gateway:manage ap:gateway_custom_policy:manage ap:rest_api:manage ap:llm_provider:manage ap:llm_proxy:manage ap:mcp_proxy:manage ap:webbroker_api:manage ap:websub_api:manage ap:application:manage ap:subscription:manage ap:subscription_plan:manage ap:project:manage ap:llm_template:manage ap:devportal:manage ap:git:read ap:api_key:read dp:org_read dp:org_write dp:org_manage dp:org_delete dp:org_content_read dp:org_content_write dp:org_content_manage dp:org_content_delete dp:api_read dp:api_write dp:api_manage dp:api_delete dp:api_content_read dp:api_content_write dp:api_content_manage dp:api_content_delete dp:api_key_read dp:api_key_write dp:api_key_manage dp:api_key_revoke dp:api_flow_read dp:api_flow_write dp:api_flow_manage dp:api_flow_delete dp:api_workflow_read dp:api_workflow_create dp:api_workflow_update dp:api_workflow_delete dp:api_workflow_manage dp:app_read dp:app_write dp:app_manage dp:app_delete dp:app_key_write dp:app_key_manage dp:app_key_revoke dp:app_key_mapping_read dp:app_key_mapping_write dp:app_key_mapping_manage dp:subscription_read dp:subscription_write dp:subscription_manage dp:subscription_delete dp:sub_plan_read dp:sub_plan_write dp:sub_plan_manage dp:sub_plan_delete dp:idp_read dp:idp_write dp:idp_manage dp:idp_delete dp:view_read dp:view_write dp:view_manage dp:view_delete dp:km_read dp:km_write dp:km_manage dp:km_delete dp:label_read dp:label_write dp:label_manage dp:label_delete dp:provider_read dp:provider_write dp:provider_manage dp:provider_delete dp:event_read dp:delivery_manage dp:utility_write dp:utility_manage dp:webhook_subscriber_create dp:webhook_subscriber_read dp:webhook_subscriber_update dp:webhook_subscriber_delete dp:webhook_subscriber_manage dev"

log "Provisioning Platform API admin credentials ..."
if grep -q "^AUTH_FILE_BASED_USERS=" "$ENV_FILE" 2>/dev/null; then
    log "  - AUTH_FILE_BASED_USERS already set in api-platform.env, leaving admin credentials as-is"
else
    GENERATED_PASSWORD="$(openssl rand -base64 24 | tr -dc 'A-Za-z0-9' | cut -c1-20)"
    [ -n "$GENERATED_PASSWORD" ] || fail "failed to generate an admin password."

    # Use a throwaway httpd container for bcrypt hashing (htpasswd -B) rather than
    # requiring apache2-utils to be installed on the host — docker is already a
    # hard requirement for the rest of this workflow.
    ADMIN_HASH="$(docker run --rm httpd:2.4-alpine htpasswd -nbBC 12 "$ADMIN_USERNAME" "$GENERATED_PASSWORD" | cut -d: -f2)"
    [ -n "$ADMIN_HASH" ] || fail "failed to hash the admin password (is docker able to pull httpd:2.4-alpine?)."

    # Docker Compose's env_file: loader applies its own ${VAR} interpolation to
    # file content, so a bcrypt hash's "$2y$12$..." gets silently mangled (each
    # "$xyz" segment treated as an undefined variable reference and blanked out)
    # unless every literal "$" is escaped as "$$" here. This is Compose-file
    # syntax, not JSON syntax — platform-api itself receives the un-escaped hash
    # once Compose substitutes "$$" back to "$" before injecting the container env.
    ADMIN_HASH_ESCAPED="${ADMIN_HASH//\$/\$\$}"

    # Matches platform-api's own FileBasedUser JSON shape exactly (see
    # platform-api/config/config.go: FileBasedUser / fileBasedUsersDecodeHook).
    # configs/config-platform-api.toml references this via
    # users = '{{ env "AUTH_FILE_BASED_USERS" }}' under [auth.file_based].
    AUTH_FILE_BASED_USERS_JSON="$(printf '[{"username":"%s","password_hash":"%s","scopes":"%s"}]' \
        "$ADMIN_USERNAME" "$ADMIN_HASH_ESCAPED" "$ADMIN_SCOPES")"
    printf 'AUTH_FILE_BASED_USERS=%s\n' "$AUTH_FILE_BASED_USERS_JSON" >> "$ENV_FILE"
    log "  - AUTH_FILE_BASED_USERS generated in api-platform.env with a random admin password"
fi

echo
log "Setup complete."
echo
if [ -n "$GENERATED_PASSWORD" ]; then
    echo "  ------------------------------------------------------------------"
    echo "   Admin login:  ${ADMIN_USERNAME} / ${GENERATED_PASSWORD}"
    echo "   This password will not be shown again — copy it now."
    echo "   (It is stored, bcrypt-hashed, in api-platform.env's AUTH_FILE_BASED_USERS)"
    echo "  ------------------------------------------------------------------"
    echo
fi
echo "  Next step:"
echo "    docker compose up"
echo
