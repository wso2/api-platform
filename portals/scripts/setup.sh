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
# Shared quickstart setup script for both the AI Workspace and the Developer
# Portal quickstarts (and their standalone distribution zips). Both stacks
# run the same three services — platform-api, ai-workspace, developer-portal —
# behind docker-compose profiles, so this single script provisions everything
# either stack (or both at once) might need:
#
#   - a self-signed TLS certificate shared by all three services
#   - devportal's own encryption/session keys      (APIP_DP_SECURITY_*)
#   - the Platform API's at-rest encryption key, written to resources/keys/encryption.key
#     and read by config.toml via {{ file }} — like the JWT keypair below, never stored
#     as an env var
#   - an RS256 JWT signing keypair for the Platform API, written as PEM files
#     under resources/keys (jwt_private.pem / jwt_public.pem) and read by
#     config.toml via {{ file }} — tokens are signed asymmetrically, so there
#     is no shared HMAC secret to copy between services
#   - an admin username/password (prompted interactively — see below),
#     bcrypt-hashed into APIP_CP_ADMIN_USERNAME / APIP_CP_ADMIN_PASSWORD_HASH
#
# This is a ONE-TIME step. It never runs as part of container startup — every
# service fails closed at startup if a required secret is missing, rather
# than silently generating or accepting a weaker one. Re-running this script
# is safe: by default it only fills in what's missing and never overwrites an
# existing value; pass --force to rotate the TLS cert, JWT signing keypair,
# and admin credentials. --force deliberately does NOT touch the at-rest
# encryption key — see --rotate-encryption-key below.
#
# Usage (run from either portals/ai-workspace or portals/developer-portal, or
# from the root of a standalone distribution zip — same script, same layout):
#   ../scripts/setup.sh          # inside the repo checkout
#   ./scripts/setup.sh     # inside a distribution zip (copied there by `make dist`)
#   docker compose --profile <ai-workspace|developer-portal|all> up -d
#
# Flags:
#   --force                   regenerate TLS cert, JWT signing keypair, and
#                              admin credentials (never the encryption key)
#   --certs-only              generate only the TLS certificate (used by
#                              `make bff-run`)
#   --rotate-encryption-key   DESTRUCTIVE: replace an existing at-rest
#                              encryption key. Anything encrypted under the
#                              old key becomes permanently unreadable.
#
# ADMIN_USERNAME / ADMIN_PASSWORD environment variables skip the interactive
# prompts and pin the credentials (used by CI). If ADMIN_PASSWORD is left
# unset at an interactive terminal, pressing Enter at the prompt generates a
# random one, printed once at the end.
#
# To rotate a single value by hand, delete it from api-platform.env (or
# delete resources/certificates / resources/keys) and re-run this script.

set -euo pipefail

FORCE=false
CERTS_ONLY=false
ROTATE_ENCRYPTION_KEY=false

for arg in "$@"; do
  case "$arg" in
    --force) FORCE=true ;;
    --certs-only) CERTS_ONLY=true ;;
    --rotate-encryption-key) ROTATE_ENCRYPTION_KEY=true ;;
    -h|--help)
      cat <<'EOF'
Usage: ./setup.sh [--force] [--certs-only] [--rotate-encryption-key]

  --force                   regenerate TLS cert, JWT signing keypair, and
                             admin credentials. Never rotates the at-rest
                             encryption key on its own — see
                             --rotate-encryption-key.
  --certs-only              generate only the TLS certificate (used by
                             `make bff-run`)
  --rotate-encryption-key   DESTRUCTIVE: replace resources/keys/encryption.key
                             even if one already exists. Any data encrypted
                             under the old key (e.g. stored secrets) becomes
                             permanently unreadable. Requires interactive
                             confirmation unless ADMIN_USERNAME/ADMIN_PASSWORD
                             are set (CI), in which case passing this flag is
                             itself treated as confirmation.

ADMIN_USERNAME / ADMIN_PASSWORD environment variables skip the interactive
prompts and pin the credentials (used by CI).
EOF
      exit 0
      ;;
    *) echo "[setup] ERROR: unknown option: $arg (try --help)" >&2; exit 2 ;;
  esac
done

THIS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

log() { echo "[setup] $*"; }
fail() { echo "[setup] ERROR: $*" >&2; exit 1; }

# This script is invoked from a caller's working directory that has its own
# docker-compose.yaml one level below it (portals/ai-workspace, or
# portals/developer-portal, or the root of a distribution zip via
# scripts/setup.sh) — never from this script's own directory. Look at $PWD
# first, then fall back to a sibling/parent of this file, so the same file
# works whether it's called as `../scripts/setup.sh` (dev checkout) or
# `./scripts/setup.sh` (distribution zip layout).
if [[ -f "$PWD/docker-compose.yaml" ]]; then
    ROOT_DIR="$PWD"
elif [[ -f "$THIS_DIR/docker-compose.yaml" ]]; then
    ROOT_DIR="$THIS_DIR"
elif [[ -f "$THIS_DIR/../docker-compose.yaml" ]]; then
    ROOT_DIR="$(cd "$THIS_DIR/.." && pwd)"
else
    fail "could not find docker-compose.yaml in the current directory, next to this script, or its parent directory. Run this from portals/ai-workspace, portals/developer-portal, or a distribution zip's root."
fi
cd "$ROOT_DIR"

ENV_FILE="$ROOT_DIR/api-platform.env"
CERTS_DIR="$ROOT_DIR/resources/certificates"
# RS256 JWT keypair (PEM). Mounted into the platform-api container at
# /etc/platform-api/keys and read by config.toml via {{ file }}.
KEYS_DIR="$ROOT_DIR/resources/keys"

# Every service image runs as this fixed non-root UID (see platform-api,
# ai-workspace, and developer-portal Dockerfiles).
CONTAINER_UID=10001

# Non-sensitive files (public cert, public key) — safe to leave world-readable
# so the bind-mounted container UID above can read them.
FILE_MODE=644

# Sensitive files (private keys, the at-rest encryption key) are locked to
# 600 (owner-only, unreadable to other host users) and, where an ACL
# mechanism actually works, get an explicit entry granting read access to the
# fixed container UID instead of opening the file to every host user via 644.
# `setfacl` (Linux `acl` package) supports targeting a bare numeric UID
# directly. macOS's `chmod +a` cannot target a numeric UID that has no
# resolvable local account (the container's UID 10001 never exists on the
# host), so there is no working ACL path there — fall back to the previous
# world-readable 644 rather than silently leaving the container unable to
# read a file it needs at startup.
restrict_secret_file() {
    local file="$1"
    if command -v setfacl >/dev/null 2>&1; then
        chmod 600 "$file"
        if setfacl -m "u:${CONTAINER_UID}:r" "$file" 2>/dev/null; then
            return
        fi
        log "  - WARNING: setfacl failed on $file; falling back to world-readable so the container (UID ${CONTAINER_UID}) can still read it"
    fi
    chmod "$FILE_MODE" "$file"
}

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

# Sets KEY=VALUE in api-platform.env. Idempotent by default — never
# overwrites a value the user or a previous run already set — unless --force
# was passed, in which case any existing line for KEY is replaced.
set_env_var() {
    local key="$1" value="$2"
    if [[ "$FORCE" == false ]] && grep -q "^${key}=" "$ENV_FILE" 2>/dev/null; then
        log "  - ${key} already set in api-platform.env, leaving as-is"
        return
    fi
    if grep -q "^${key}=" "$ENV_FILE" 2>/dev/null; then
        # macOS/BSD sed and GNU sed both accept -i.bak with an explicit suffix.
        sed -i.bak "/^${key}=/d" "$ENV_FILE" && rm -f "$ENV_FILE.bak"
    fi
    printf '%s=%s\n' "$key" "$value" >> "$ENV_FILE"
    log "  - ${key} generated"
}

log "Provisioning TLS certificate ..."
mkdir -p "$CERTS_DIR"
# Shared by every service (mounted read-only into each at its own
# /etc/<service>/tls path) — SAN covers every hostname/container name either
# compose file uses, plus host.docker.internal for host-to-container access.
if [[ "$FORCE" == false && -f "$CERTS_DIR/cert.pem" && -f "$CERTS_DIR/key.pem" ]]; then
    log "  - $CERTS_DIR already has a certificate, leaving as-is"
else
    openssl req -x509 -newkey rsa:2048 -sha256 -days 3650 -nodes \
        -keyout "$CERTS_DIR/key.pem" -out "$CERTS_DIR/cert.pem" \
        -subj "/O=WSO2 API Platform/CN=localhost" \
        -addext "subjectAltName=DNS:localhost,DNS:*.localhost,DNS:platform-api,DNS:ai-workspace,DNS:developer-portal,DNS:devportal,DNS:host.docker.internal,IP:127.0.0.1" \
        >/dev/null 2>&1
    chmod "$FILE_MODE" "$CERTS_DIR/cert.pem"
    restrict_secret_file "$CERTS_DIR/key.pem"
    log "  - self-signed certificate generated at $CERTS_DIR"
fi

if [[ "$CERTS_ONLY" == true ]]; then
    exit 0
fi

touch "$ENV_FILE"
# api-platform.env is read directly by the Docker CLI/daemon via env_file: —
# never bind-mounted into a container — so it only needs host-level
# protection, not container-UID read access via restrict_secret_file.
chmod 600 "$ENV_FILE"

log "Generating devportal secrets into api-platform.env ..."
set_env_var "APIP_DP_SECURITY_ENCRYPTION_KEY" "$(openssl rand -hex 32)"
set_env_var "APIP_DP_SECURITY_SESSION_SECRET" "$(openssl rand -hex 32)"

log "Provisioning Platform API JWT signing keypair (RS256) ..."
# Tokens are signed asymmetrically (RS256), not with a shared HMAC secret. The
# Platform API mints login tokens with the RSA private key and verifies every
# token with the matching public key. A PEM key is multi-line and does not
# survive an env file (one KEY=VALUE per line), so — like the TLS cert above —
# the keypair is written to files and read by config.toml via {{ file }}:
#   config.toml -> public_key/private_key = '{{ file "/etc/platform-api/keys/jwt_*.pem" }}'
# resources/keys is mounted into the platform-api container at
# /etc/platform-api/keys (see docker-compose.yaml), which is on the Platform
# API's {{ file }} allowlist.
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
    restrict_secret_file "$KEYS_DIR/jwt_private.pem"
    log "  - RS256 JWT keypair generated at $KEYS_DIR"
fi

log "Provisioning Platform API at-rest encryption key ..."
# Written to a file (not api-platform.env) and read by config.toml via
# {{ file "/etc/platform-api/keys/encryption.key" }} — the same mounted keys
# dir as the JWT keypair above. 32-byte key as 64 hex chars. Preserve an
# existing key across reruns — regenerating it makes previously-encrypted
# data permanently unreadable — so this key is rotated ONLY via the explicit,
# separate --rotate-encryption-key flag, never by the generic --force (which
# rotates the TLS cert / JWT keypair / admin credentials, none of which risk
# data loss the way rotating this key does).
if [[ -f "$KEYS_DIR/encryption.key" && "$ROTATE_ENCRYPTION_KEY" == true ]]; then
    if [[ -t 0 && -z "${ADMIN_USERNAME:-}" && -z "${ADMIN_PASSWORD:-}" ]]; then
        echo
        echo "  WARNING: rotating $KEYS_DIR/encryption.key will make any data"
        echo "  encrypted under the current key permanently unreadable."
        read -r -p "  Type 'rotate' to proceed: " CONFIRM_ROTATE
        [[ "$CONFIRM_ROTATE" == "rotate" ]] || fail "encryption key rotation not confirmed; aborting."
    else
        log "  - non-interactive/CI invocation: --rotate-encryption-key passed, treating that as confirmation"
    fi
    openssl rand -hex 32 > "$KEYS_DIR/encryption.key"
    restrict_secret_file "$KEYS_DIR/encryption.key"
    log "  - at-rest encryption key ROTATED at $KEYS_DIR/encryption.key"
elif [[ -f "$KEYS_DIR/encryption.key" ]]; then
    log "  - $KEYS_DIR/encryption.key already exists, leaving as-is (pass --rotate-encryption-key to replace it)"
else
    mkdir -p "$KEYS_DIR"
    openssl rand -hex 32 > "$KEYS_DIR/encryption.key"
    restrict_secret_file "$KEYS_DIR/encryption.key"
    log "  - at-rest encryption key generated at $KEYS_DIR/encryption.key"
fi

log "Provisioning Platform API admin credentials ..."
CREDENTIALS_PROVISIONED=false
HAS_ADMIN_USERNAME=false
HAS_ADMIN_HASH=false
grep -q "^APIP_CP_ADMIN_USERNAME=" "$ENV_FILE" 2>/dev/null && HAS_ADMIN_USERNAME=true
grep -q "^APIP_CP_ADMIN_PASSWORD_HASH=" "$ENV_FILE" 2>/dev/null && HAS_ADMIN_HASH=true

if [[ "$FORCE" == false && "$HAS_ADMIN_USERNAME" == true && "$HAS_ADMIN_HASH" != true ]]; then
    fail "api-platform.env has APIP_CP_ADMIN_USERNAME set but no APIP_CP_ADMIN_PASSWORD_HASH — a username paired with no hash (or a mismatched one) can never authenticate. Either delete APIP_CP_ADMIN_USERNAME from api-platform.env and re-run this script to regenerate both together, or re-run with --force to rotate both credentials at once."
elif [[ "$FORCE" == false && "$HAS_ADMIN_HASH" == true && "$HAS_ADMIN_USERNAME" != true ]]; then
    fail "api-platform.env has APIP_CP_ADMIN_PASSWORD_HASH set but no APIP_CP_ADMIN_USERNAME — a hash with no matching username can never authenticate. Either delete APIP_CP_ADMIN_PASSWORD_HASH from api-platform.env and re-run this script to regenerate both together, or re-run with --force to rotate both credentials at once."
elif [[ "$FORCE" == false && "$HAS_ADMIN_USERNAME" == true && "$HAS_ADMIN_HASH" == true ]]; then
    log "  - APIP_CP_ADMIN_USERNAME already set in api-platform.env, leaving admin credentials as-is"
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

    # Written raw, un-escaped — docker-compose.yaml's env_file: entries use
    # `format: raw`, which passes file content through byte-for-byte with no
    # ${VAR} interpolation, so a literal bcrypt hash ("$2y$12$...") survives
    # into the container as-is. Escaping "$" as "$$" here would corrupt it.
    set_env_var "APIP_CP_ADMIN_USERNAME" "$ADMIN_USERNAME"
    set_env_var "APIP_CP_ADMIN_PASSWORD_HASH" "$ADMIN_HASH"

    CREDENTIALS_PROVISIONED=true
fi

echo
log "Setup complete."
echo
if [[ "$CREDENTIALS_PROVISIONED" == true ]]; then
    echo "  ------------------------------------------------------------------"
    echo "   Admin login:  ${ADMIN_USERNAME} / ${ADMIN_PASSWORD}"
    echo "   This password will not be shown again — copy it now."
    echo "   (It is stored, bcrypt-hashed, in api-platform.env's APIP_CP_ADMIN_PASSWORD_HASH)"
    echo "  ------------------------------------------------------------------"
    echo
fi
echo "  Next step — choose how much to start:"
echo
# Each pack's docker-compose.yaml marks its optional sibling service with a
# distinct, stable comment — used here to print every profile combination for
# whichever pack this script is running in, with the pack's own default
# listed first, rather than a generic placeholder the reader has to decode.
if grep -q '^  # Optional: enable AI Workspace' "$ROOT_DIR/docker-compose.yaml" 2>/dev/null; then
    echo "    docker compose --profile developer-portal up -d                         # Developer Portal + Platform API (default)"
    echo "    docker compose --profile developer-portal --profile ai-workspace up -d  # + AI Workspace"
    echo "    docker compose --profile all up -d                                      # AI Workspace + Developer Portal + Platform API"
    echo "    docker compose --profile platform-api up -d                             # Platform API only"
elif grep -q '^  # Optional: enable the developer portal' "$ROOT_DIR/docker-compose.yaml" 2>/dev/null; then
    echo "    docker compose --profile ai-workspace up -d                             # AI Workspace + Platform API (default)"
    echo "    docker compose --profile ai-workspace --profile developer-portal up -d  # + Developer Portal"
    echo "    docker compose --profile all up -d                                      # AI Workspace + Developer Portal + Platform API"
    echo "    docker compose --profile platform-api up -d                             # Platform API only"
else
    echo "    docker compose --profile <ai-workspace|developer-portal|all|platform-api> up -d"
fi
echo
