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
# Provisions, in order:
#   - a self-signed TLS certificate for devportal
#   - devportal's own encryption/session keys      (APIP_DP_SECURITY_*)
#   - the Platform API's at-rest encryption key     (APIP_CP_ENCRYPTION_KEY)
#   - an RS256 JWT signing keypair for the Platform API, written as PEM files
#     under resources/keys (jwt_private.pem / jwt_public.pem) and read by
#     config.toml via {{ file }} — tokens are signed asymmetrically, so there is
#     no shared HMAC secret to copy between services
#   - an admin username/password (prompted interactively — see below), bcrypt-hashed
#     into APIP_CP_ADMIN_USERNAME / APIP_CP_ADMIN_PASSWORD_HASH
#
# This is a ONE-TIME step. It never runs as part of container startup — both
# services fail closed at startup if a required secret is missing, rather than
# silently generating or accepting a weaker one. Re-running this script is
# safe: it only fills in what's missing and never overwrites an existing value.
#
# Usage (from the project root, or the standalone distribution zip — same
# layout in both):
#   ./scripts/setup.sh
#   docker compose up
#
# ADMIN_USERNAME / ADMIN_PASSWORD environment variables skip the interactive
# prompts and pin the credentials (used by CI). If ADMIN_PASSWORD is left
# unset at an interactive terminal, pressing Enter at the prompt generates
# a random one, printed once at the end.
#
# To rotate a value, delete it from api-platform.env (or delete
# resources/certificates for the TLS cert) and re-run this script.

set -euo pipefail

THIS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# This same script is copied verbatim into the distribution zip's scripts/
# directory (see Makefile's dist target) — both layouts put this script one
# level below docker-compose.yaml, so ROOT_DIR is always the parent directory.
# The direct-sibling check is kept as a fallback in case this file is ever
# copied elsewhere.
if [ -f "$THIS_DIR/../docker-compose.yaml" ]; then
    ROOT_DIR="$(cd "$THIS_DIR/.." && pwd)"
elif [ -f "$THIS_DIR/docker-compose.yaml" ]; then
    ROOT_DIR="$THIS_DIR"
else
    echo "[setup] ERROR: could not find docker-compose.yaml next to this script or its parent directory." >&2
    echo "[setup]        Run this as ./scripts/setup.sh from the project root or the distribution zip." >&2
    exit 1
fi
cd "$ROOT_DIR"

ENV_FILE="$ROOT_DIR/api-platform.env"
DEVPORTAL_CERT_DIR="$ROOT_DIR/resources/certificates"
# RS256 JWT keypair (PEM). Mounted into the platform-api container at
# /etc/platform-api/keys and read by config.toml via {{ file }}.
JWT_KEY_DIR="$ROOT_DIR/resources/keys"

# Bind-mounted into a container running as a non-root UID: 644 (not 600) so the
# container user can read a file owned by the host user. Local single-user
# quick-start tradeoff — matches the perms the old auto-generated cert used.
CERT_FILE_MODE=644

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

log "Provisioning TLS certificate ..."
mkdir -p "$DEVPORTAL_CERT_DIR"
# Shared by both containers (mounted read-only into each at its own /etc/<service>/tls
# path) — platform-api's HTTPSListener hardcodes the filenames cert.pem/key.pem within
# its cert_dir, so this pair must use those exact names regardless of what devportal's
# own (fully configurable) APIP_DP_TLS_CERTFILE/KEYFILE point at.
if [ -f "$DEVPORTAL_CERT_DIR/cert.pem" ] && [ -f "$DEVPORTAL_CERT_DIR/key.pem" ]; then
    log "  - $DEVPORTAL_CERT_DIR already has a certificate, leaving as-is"
else
    openssl req -x509 -newkey rsa:4096 \
        -keyout "$DEVPORTAL_CERT_DIR/key.pem" \
        -out    "$DEVPORTAL_CERT_DIR/cert.pem" \
        -days 36500 -nodes \
        -subj "/C=US/ST=California/L=San Francisco/O=WSO2/OU=Developer Portal/CN=localhost" \
        -addext "subjectAltName=DNS:localhost,DNS:*.localhost,DNS:platform-api,DNS:devportal,IP:127.0.0.1" \
        2>/dev/null
    chmod "$CERT_FILE_MODE" "$DEVPORTAL_CERT_DIR/key.pem" "$DEVPORTAL_CERT_DIR/cert.pem"
    log "  - self-signed certificate generated at $DEVPORTAL_CERT_DIR"
fi

log "Generating devportal secrets into api-platform.env ..."
set_env_var "APIP_DP_SECURITY_ENCRYPTIONKEY" "$(openssl rand -hex 32)"
set_env_var "APIP_DP_SECURITY_SESSIONSECRET" "$(openssl rand -hex 32)"

log "Generating Platform API encryption key into api-platform.env ..."
set_env_var "APIP_CP_ENCRYPTION_KEY" "$(openssl rand -hex 32)"

log "Provisioning Platform API JWT signing keypair (RS256) ..."
# Tokens are signed asymmetrically now (RS256), not with a shared HMAC secret.
# The Platform API mints login tokens with the RSA private key and verifies every
# token with the matching public key. A PEM key is multi-line and does not survive
# an env file (one KEY=VALUE per line), so — like the TLS cert above — the keypair
# is written to files and read by config.toml via {{ file }}:
#   config.toml -> public_key/private_key = '{{ file "/etc/platform-api/keys/jwt_*.pem" }}'
# resources/keys is mounted into the platform-api container at /etc/platform-api/keys
# (see docker-compose.yaml), which is on the Platform API's {{ file }} allowlist.
if [ -f "$JWT_KEY_DIR/jwt_private.pem" ] && [ -f "$JWT_KEY_DIR/jwt_public.pem" ]; then
    log "  - $JWT_KEY_DIR already has a JWT keypair, leaving as-is"
else
    mkdir -p "$JWT_KEY_DIR"
    # PKCS#8 private key + matching SPKI public key — the PEM encodings
    # golang-jwt's ParseRSAPrivateKeyFromPEM / ParseRSAPublicKeyFromPEM accept.
    openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 \
        -out "$JWT_KEY_DIR/jwt_private.pem" 2>/dev/null
    openssl rsa -in "$JWT_KEY_DIR/jwt_private.pem" -pubout \
        -out "$JWT_KEY_DIR/jwt_public.pem" 2>/dev/null
    chmod "$CERT_FILE_MODE" "$JWT_KEY_DIR/jwt_private.pem" "$JWT_KEY_DIR/jwt_public.pem"
    log "  - RS256 JWT keypair generated at $JWT_KEY_DIR"
fi

log "Provisioning Platform API admin credentials ..."
CREDENTIALS_PROVISIONED=false
if grep -q "^APIP_CP_ADMIN_USERNAME=" "$ENV_FILE" 2>/dev/null; then
    log "  - APIP_CP_ADMIN_USERNAME already set in api-platform.env, leaving admin credentials as-is"
else
    GENERATED_PASSWORD="$(openssl rand -base64 24 | tr -dc 'A-Za-z0-9' | cut -c1-20)"
    [ -n "$GENERATED_PASSWORD" ] || fail "failed to generate an admin password."

    if [ -z "${ADMIN_USERNAME:-}" ] && [ -t 0 ]; then
        read -r -p "Admin username [admin]: " ADMIN_USERNAME
    fi
    ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"

    if [ -z "${ADMIN_PASSWORD:-}" ] && [ -t 0 ]; then
        read -r -s -p "Admin password [press Enter to generate one]: " ADMIN_PASSWORD
        echo
    fi
    ADMIN_PASSWORD="${ADMIN_PASSWORD:-$GENERATED_PASSWORD}"

    # Use a throwaway httpd container for bcrypt hashing (htpasswd -B) rather than
    # requiring apache2-utils to be installed on the host — docker is already a
    # hard requirement for the rest of this workflow.
    ADMIN_HASH="$(docker run --rm httpd:2.4-alpine htpasswd -nbBC 12 "$ADMIN_USERNAME" "$ADMIN_PASSWORD" | cut -d: -f2)"
    [ -n "$ADMIN_HASH" ] || fail "failed to hash the admin password (is docker able to pull httpd:2.4-alpine?)."

    # Written raw, un-escaped — docker-compose.yaml's env_file: entries use
    # `format: raw`, which passes file content through byte-for-byte with no
    # ${VAR} interpolation, so a literal bcrypt hash ("$2y$12$...") survives
    # into the container as-is. Escaping "$" as "$$" here would corrupt it.
    # Read by config-platform-api.toml's [[platform_api.auth.file.users]] entry —
    # scopes lives there as a plain literal, not in this env file.
    set_env_var "APIP_CP_ADMIN_USERNAME" "$ADMIN_USERNAME"
    set_env_var "APIP_CP_ADMIN_PASSWORD_HASH" "$ADMIN_HASH"

    CREDENTIALS_PROVISIONED=true
fi

echo
log "Setup complete."
echo
if [ "$CREDENTIALS_PROVISIONED" = true ]; then
    echo "  ------------------------------------------------------------------"
    echo "   Admin login:  ${ADMIN_USERNAME} / ${ADMIN_PASSWORD}"
    echo "   This password will not be shown again — copy it now."
    echo "   (It is stored, bcrypt-hashed, in api-platform.env's APIP_CP_ADMIN_PASSWORD_HASH)"
    echo "  ------------------------------------------------------------------"
    echo
fi
echo "  Next step:"
echo "    docker compose up"
echo
