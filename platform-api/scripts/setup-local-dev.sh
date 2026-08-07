#!/usr/bin/env bash
# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License. You may obtain a copy of the
# License at http://www.apache.org/licenses/LICENSE-2.0
# --------------------------------------------------------------------
#
# One-time secret provisioning for running Platform API standalone via
# `go run` (no Docker, no docker-compose, no dependency on any other
# portal's quickstart) — pairs with config/config.local.toml, which reads
# everything this script generates via plain relative paths.
#
# Unlike portals/scripts/setup.sh (the shared ai-workspace/api-portal
# quickstart, which needs a docker-compose.yaml next to it to find its root),
# this script lives inside platform-api itself and only ever provisions
# platform-api's own secrets — nothing API Portal- or AI Workspace-specific.
#
# Generates, all under platform-api/ and all gitignored:
#   - resources/certificates/{cert,key}.pem   — self-signed TLS cert for
#     [server.https] (config.local.toml enables HTTPS by default)
#   - resources/keys/{jwt_private,jwt_public}.pem — RS256 JWT signing keypair
#     for [auth.jwt]; tokens are signed asymmetrically, so there is no shared
#     HMAC secret to manage
#   - resources/keys/encryption.key — at-rest encryption key for
#     [security].encryption_key (subscription tokens etc.)
#   - local-dev.env — APIP_CP_ADMIN_USERNAME / APIP_CP_ADMIN_PASSWORD_HASH
#     (bcrypt), read by config.local.toml's [[auth.file.users]] entry
#
# This is a ONE-TIME step. Platform API fails closed at startup if a
# required secret is missing rather than silently generating or accepting a
# weaker one. Re-running this script is safe: by default it only fills in
# what's missing and never overwrites an existing value; pass --force to
# rotate the TLS cert, JWT signing keypair, and admin credentials. --force
# deliberately does NOT touch the at-rest encryption key — see
# --rotate-encryption-key below.
#
# Usage (run from anywhere; resolves its own location):
#   ./scripts/setup-local-dev.sh
#   source scripts/local-dev.env.sh   # see the end of this script — exports
#                                      # APIP_CP_ENCRYPTION_KEY for `go run`
#
# Flags:
#   --force                   regenerate TLS cert, JWT signing keypair, and
#                              admin credentials (never the encryption key)
#   --rotate-encryption-key   DESTRUCTIVE: replace an existing at-rest
#                              encryption key. Anything encrypted under the
#                              old key becomes permanently unreadable.
#
# ADMIN_USERNAME / ADMIN_PASSWORD environment variables skip the interactive
# prompts and pin the credentials (used by CI). If ADMIN_PASSWORD is left
# unset at an interactive terminal, pressing Enter at the prompt generates a
# random one, printed once at the end.
#
# To rotate a single value by hand, delete it from local-dev.env (or delete
# resources/certificates / resources/keys) and re-run this script.

set -euo pipefail

FORCE=false
ROTATE_ENCRYPTION_KEY=false

for arg in "$@"; do
  case "$arg" in
    --force) FORCE=true ;;
    --rotate-encryption-key) ROTATE_ENCRYPTION_KEY=true ;;
    -h|--help)
      cat <<'EOF'
Usage: ./scripts/setup-local-dev.sh [--force] [--rotate-encryption-key]

  --force                   regenerate TLS cert, JWT signing keypair, and
                             admin credentials. Never rotates the at-rest
                             encryption key on its own — see
                             --rotate-encryption-key.
  --rotate-encryption-key   DESTRUCTIVE: replace resources/keys/encryption.key
                             even if it already exists. Any data encrypted
                             under the old key becomes permanently unreadable.
                             Requires interactive confirmation unless
                             ADMIN_USERNAME/ADMIN_PASSWORD are set (CI), in
                             which case passing this flag is itself treated
                             as confirmation.

ADMIN_USERNAME / ADMIN_PASSWORD environment variables skip the interactive
prompts and pin the credentials (used by CI).
EOF
      exit 0
      ;;
    *) echo "[setup-local-dev] ERROR: unknown option: $arg (try --help)" >&2; exit 2 ;;
  esac
done

# platform-api/ — this script's parent directory, however it was invoked.
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

log() { echo "[setup-local-dev] $*"; }
fail() { echo "[setup-local-dev] ERROR: $*" >&2; exit 1; }

ENV_FILE="$ROOT_DIR/local-dev.env"
CERTS_DIR="$ROOT_DIR/resources/certificates"
KEYS_DIR="$ROOT_DIR/resources/keys"

# Sensitive files (private keys, the at-rest encryption key) are locked to
# 600 (owner-only) — read directly off disk by a locally-run `go run`
# process under the developer's own user, so there is no separate container
# UID to grant read access to the way portals/scripts/setup.sh must for its
# docker-compose stack.
FILE_MODE=644

command -v openssl >/dev/null 2>&1 || fail "openssl is required but not found on PATH."

# bcrypt isn't in openssl; use htpasswd when available, else a throwaway
# httpd container, so docker isn't a hard requirement for hosts that already
# have apache2-utils/httpd-tools installed.
bcrypt_hash() {
  local username="$1" password="$2"
  if command -v htpasswd >/dev/null 2>&1; then
    printf '%s' "$password" | htpasswd -niB -C 12 "$username" | cut -d: -f2 | tr -d '\r\n'
  elif command -v docker >/dev/null 2>&1; then
    docker run --rm httpd:2.4-alpine htpasswd -nbBC 12 "$username" "$password" | cut -d: -f2 | tr -d '\r\n'
  else
    fail "need either htpasswd (apache2-utils / httpd-tools) or docker to bcrypt-hash the admin password."
  fi
}

# Sets KEY=VALUE in the given env file. Idempotent by default — never
# overwrites a value the user or a previous run already set — unless --force
# was passed, in which case any existing line for KEY is replaced.
set_env_var() {
    local file="$1" key="$2" value="$3"
    if [[ "$FORCE" == false ]] && grep -q "^${key}=" "$file" 2>/dev/null; then
        log "  - ${key} already set in $(basename "$file"), leaving as-is"
        return
    fi
    if grep -q "^${key}=" "$file" 2>/dev/null; then
        sed -i.bak "/^${key}=/d" "$file" && rm -f "$file.bak"
    fi
    printf '%s=%s\n' "$key" "$value" >> "$file"
    log "  - ${key} set in $(basename "$file")"
}

# One-time confirmation gate for --rotate-encryption-key.
confirm_rotation_once() {
    local key_path="$1"
    if [[ -t 0 && -z "${ADMIN_USERNAME:-}" && -z "${ADMIN_PASSWORD:-}" ]]; then
        echo
        echo "  WARNING: --rotate-encryption-key will make any data encrypted"
        echo "  under the current at-rest encryption key ($key_path)"
        echo "  permanently unreadable."
        read -r -p "  Type 'rotate' to proceed: " CONFIRM_ROTATE
        [[ "$CONFIRM_ROTATE" == "rotate" ]] || fail "encryption key rotation not confirmed; aborting."
    else
        log "  - non-interactive/CI invocation: --rotate-encryption-key passed, treating that as confirmation"
    fi
}

log "Provisioning TLS certificate ..."
mkdir -p "$CERTS_DIR"
if [[ "$FORCE" == false && -f "$CERTS_DIR/cert.pem" && -f "$CERTS_DIR/key.pem" ]]; then
    log "  - $CERTS_DIR already has a certificate, leaving as-is"
else
    openssl req -x509 -newkey rsa:2048 -sha256 -days 3650 -nodes \
        -keyout "$CERTS_DIR/key.pem" -out "$CERTS_DIR/cert.pem" \
        -subj "/O=WSO2 API Platform/CN=localhost" \
        -addext "subjectAltName=DNS:localhost,IP:127.0.0.1" \
        >/dev/null 2>&1
    chmod "$FILE_MODE" "$CERTS_DIR/cert.pem"
    chmod 600 "$CERTS_DIR/key.pem"
    log "  - self-signed certificate generated at $CERTS_DIR"
fi

log "Provisioning JWT signing keypair (RS256) ..."
# Tokens are signed asymmetrically, not with a shared HMAC secret: Platform
# API mints login tokens with the RSA private key and verifies every token
# with the matching public key. config.local.toml reads both PEM files via
# plain os.ReadFile (not the {{ file }} interpolation token), so a relative
# path under this repo checkout works fine.
if [[ "$FORCE" == false && -f "$KEYS_DIR/jwt_private.pem" && -f "$KEYS_DIR/jwt_public.pem" ]]; then
    log "  - $KEYS_DIR already has a JWT keypair, leaving as-is"
else
    mkdir -p "$KEYS_DIR"
    # PKCS#8 private key + matching SPKI public key — the PEM encodings
    # golang-jwt's ParseRSAPrivateKeyFromPEM / ParseRSAPublicKeyFromPEM accept.
    openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 \
        -out "$KEYS_DIR/jwt_private.pem" 2>/dev/null
    openssl rsa -in "$KEYS_DIR/jwt_private.pem" -pubout \
        -out "$KEYS_DIR/jwt_public.pem" 2>/dev/null
    chmod "$FILE_MODE" "$KEYS_DIR/jwt_public.pem"
    chmod 600 "$KEYS_DIR/jwt_private.pem"
    log "  - RS256 JWT keypair generated at $KEYS_DIR"
fi

log "Provisioning at-rest encryption key ..."
# 32-byte key as 64 hex chars, read via {{ env "APIP_CP_ENCRYPTION_KEY" }} in
# config.local.toml — NOT {{ file }}: the config-interpolation file allowlist
# is two fixed absolute directories (/etc/platform-api, /secrets/platform-api,
# see config/config.go's defaultFileSourceAllowlist), so a relative path under
# this repo checkout is rejected there regardless of the value. Preserve an
# existing key across reruns — regenerating it makes previously-encrypted
# data permanently unreadable — so this key is rotated ONLY via the explicit,
# separate --rotate-encryption-key flag, never by the generic --force.
if [[ -f "$KEYS_DIR/encryption.key" && "$ROTATE_ENCRYPTION_KEY" == true ]]; then
    confirm_rotation_once "$KEYS_DIR/encryption.key"
    openssl rand -hex 32 > "$KEYS_DIR/encryption.key"
    chmod 600 "$KEYS_DIR/encryption.key"
    log "  - at-rest encryption key ROTATED at $KEYS_DIR/encryption.key"
elif [[ -f "$KEYS_DIR/encryption.key" ]]; then
    log "  - $KEYS_DIR/encryption.key already exists, leaving as-is (pass --rotate-encryption-key to replace it)"
else
    mkdir -p "$KEYS_DIR"
    openssl rand -hex 32 > "$KEYS_DIR/encryption.key"
    chmod 600 "$KEYS_DIR/encryption.key"
    log "  - at-rest encryption key generated at $KEYS_DIR/encryption.key"
fi

log "Provisioning admin credentials ..."
touch "$ENV_FILE"
chmod 600 "$ENV_FILE"
CREDENTIALS_PROVISIONED=false
HAS_ADMIN_USERNAME=false
HAS_ADMIN_HASH=false
grep -q "^APIP_CP_ADMIN_USERNAME=" "$ENV_FILE" 2>/dev/null && HAS_ADMIN_USERNAME=true
grep -q "^APIP_CP_ADMIN_PASSWORD_HASH=" "$ENV_FILE" 2>/dev/null && HAS_ADMIN_HASH=true

if [[ "$FORCE" == false && "$HAS_ADMIN_USERNAME" == true && "$HAS_ADMIN_HASH" != true ]]; then
    fail "local-dev.env has APIP_CP_ADMIN_USERNAME set but no APIP_CP_ADMIN_PASSWORD_HASH — a username paired with no hash can never authenticate. Either delete APIP_CP_ADMIN_USERNAME from local-dev.env and re-run this script to regenerate both together, or re-run with --force to rotate both credentials at once."
elif [[ "$FORCE" == false && "$HAS_ADMIN_HASH" == true && "$HAS_ADMIN_USERNAME" != true ]]; then
    fail "local-dev.env has APIP_CP_ADMIN_PASSWORD_HASH set but no APIP_CP_ADMIN_USERNAME — a hash with no matching username can never authenticate. Either delete APIP_CP_ADMIN_PASSWORD_HASH from local-dev.env and re-run this script to regenerate both together, or re-run with --force to rotate both credentials at once."
elif [[ "$FORCE" == false && "$HAS_ADMIN_USERNAME" == true && "$HAS_ADMIN_HASH" == true ]]; then
    log "  - APIP_CP_ADMIN_USERNAME already set in local-dev.env, leaving admin credentials as-is"
else
    GENERATED_PASSWORD="$(openssl rand -base64 24 | tr -dc 'A-Za-z0-9' | cut -c1-20)"
    [[ -n "$GENERATED_PASSWORD" ]] || fail "failed to generate an admin password."

    if [[ -z "${ADMIN_USERNAME:-}" && -t 0 ]]; then
        read -r -p "Admin username [admin]: " ADMIN_USERNAME
    fi
    ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"

    if [[ -z "${ADMIN_PASSWORD:-}" && -t 0 ]]; then
        read -r -s -p "Admin password [press Enter to generate one]: " ADMIN_PASSWORD
        echo
    fi
    ADMIN_PASSWORD="${ADMIN_PASSWORD:-$GENERATED_PASSWORD}"

    ADMIN_HASH="$(bcrypt_hash "$ADMIN_USERNAME" "$ADMIN_PASSWORD")"
    [[ -n "$ADMIN_HASH" ]] || fail "failed to hash the admin password (is docker able to pull httpd:2.4-alpine, or is htpasswd installed?)."

    set_env_var "$ENV_FILE" "APIP_CP_ADMIN_USERNAME" "$ADMIN_USERNAME"
    set_env_var "$ENV_FILE" "APIP_CP_ADMIN_PASSWORD_HASH" "$ADMIN_HASH"

    CREDENTIALS_PROVISIONED=true
fi

echo
log "Setup complete."
echo
if [[ "$CREDENTIALS_PROVISIONED" == true ]]; then
    echo "  ------------------------------------------------------------------"
    echo "   Admin login:  ${ADMIN_USERNAME} / ${ADMIN_PASSWORD}"
    echo "   This password will not be shown again — copy it now."
    echo "   (It is stored, bcrypt-hashed, in local-dev.env's APIP_CP_ADMIN_PASSWORD_HASH)"
    echo "  ------------------------------------------------------------------"
    echo
fi
echo "  Next step — run Platform API:"
echo
echo "    export \$(grep -v '^#' local-dev.env | xargs) 2>/dev/null; export APIP_CP_ENCRYPTION_KEY=\$(cat resources/keys/encryption.key)"
echo "    go run ./cmd/main.go -config config/config.local.toml"
echo
echo "  (Do not 'source' local-dev.env directly — its bcrypt hash contains a"
echo "   literal \"\$2y\$12\$...\", which bash reinterprets as positional-parameter"
echo "   expansion and corrupts it. 'export \$(grep ... | xargs)' above is safe"
echo "   because xargs passes the hash through as one opaque argument.)"
echo
