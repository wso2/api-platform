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
# run the same three services — platform-api, ai-workspace, api-portal —
# behind docker-compose profiles, so this single script provisions everything
# either stack (or both at once) might need:
#
#   - a self-signed TLS certificate shared by all three services
#   - API Portal's own encryption key and session secret, written to
#     resources/keys/api-portal-encryption.key and api-portal-session-secret and
#     read by config.toml via {{ file }} — never stored as an env var
#   - the Platform API's at-rest encryption key, written to resources/keys/encryption.key
#     and read by config.toml via {{ file }} — like the JWT keypair below, never stored
#     as an env var
#   - an RS256 JWT signing keypair for the Platform API, written as PEM files
#     under resources/keys (jwt_private.pem / jwt_public.pem) and read by
#     config.toml via {{ file }} — tokens are signed asymmetrically, so there
#     is no shared HMAC secret to copy between services
#   - an admin username/password (prompted interactively — see below),
#     bcrypt-hashed into APIP_CP_ADMIN_USERNAME / APIP_CP_ADMIN_PASSWORD_HASH
#   - COMPOSE_PROJECT_NAME in ./.env, unique to this copy of the distribution.
#     Read by the docker compose CLI (not by the containers) to prefix every
#     container, network, and volume, so another extraction of the same zip on
#     this host cannot bind to this stack's volumes — the Platform API's
#     database and the API Portal's data.
#
# This is a ONE-TIME step. It never runs as part of container startup — every
# service fails closed at startup if a required secret is missing, rather
# than silently generating or accepting a weaker one. Re-running this script
# is safe: by default it only fills in what's missing and never overwrites an
# existing value; pass --force to rotate the TLS cert, JWT signing keypair,
# and admin credentials. --force deliberately does NOT touch the at-rest
# encryption key — see --rotate-encryption-key below.
#
# Usage (run from either portals/ai-workspace or portals/api-portal, or
# from the root of a standalone distribution zip — same script, same layout):
#   ../scripts/setup.sh          # inside the repo checkout
#   ./scripts/setup.sh     # inside a distribution zip (copied there by `make dist`)
#   docker compose up -d   # uses this pack's default profiles, set via COMPOSE_PROFILES in ./.env
#   docker compose --profile <ai-workspace|api-portal|all> up -d  # override
#
# Flags:
#   --force                   regenerate TLS cert, JWT signing keypair, and
#                              admin credentials (never the encryption key)
#   --certs-only              generate only the TLS certificate (used by
#                              `make bff-run`)
#   --rotate-encryption-key   DESTRUCTIVE: replace an existing at-rest
#                              encryption key. Anything encrypted under the
#                              old key becomes permanently unreadable.
#   --profiles=<a,b,...>      override the default COMPOSE_PROFILES this
#                              script writes to .env (see DEFAULT_COMPOSE_PROFILES
#                              below)
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

# The comma-separated COMPOSE_PROFILES value this script writes to .env, so
# that a plain `docker compose up -d` (no --profile flag) starts the right
# services for this pack. Each pack's `make dist` target bakes in its own
# value here via sed when it copies this shared script into the distribution
# zip (see the `dist` target in portals/ai-workspace/Makefile and
# portals/api-portal/Makefile). Left at this placeholder, it falls back
# to detect_pack()'s directory-name detection below, which is what happens
# when this script runs straight out of a repo checkout (not a dist zip).
DEFAULT_COMPOSE_PROFILES="__DEFAULT_COMPOSE_PROFILES__"
# Which pack this is ("api-portal" or "ai-workspace") — `make dist`
# bakes this in the same way as DEFAULT_COMPOSE_PROFILES above, so a shipped
# distribution zip never needs to detect it at runtime. Left at this
# placeholder, detect_pack() below falls back to this pack's own directory
# name instead (only reachable from a repo checkout).
DEFAULT_PACK_NAME="__DEFAULT_PACK_NAME__"
# This pack's version, baked in by `make dist` the same way as the two values
# above. Only feeds the generated COMPOSE_PROJECT_NAME. Left at this
# placeholder, the VERSION file in a repo checkout is read instead.
DEFAULT_DIST_VERSION="__DEFAULT_DIST_VERSION__"
PROFILES_OVERRIDE=""

for arg in "$@"; do
  case "$arg" in
    --force) FORCE=true ;;
    --certs-only) CERTS_ONLY=true ;;
    --rotate-encryption-key) ROTATE_ENCRYPTION_KEY=true ;;
    --profiles=*) PROFILES_OVERRIDE="${arg#*=}" ;;
    -h|--help)
      cat <<'EOF'
Usage: ./setup.sh [--force] [--certs-only] [--rotate-encryption-key] [--profiles=<a,b,...>]

  --force                   regenerate TLS cert, JWT signing keypair, and
                             admin credentials. Never rotates the at-rest
                             encryption key on its own — see
                             --rotate-encryption-key.
  --certs-only              generate only the TLS certificate (used by
                             `make bff-run`)
  --rotate-encryption-key   DESTRUCTIVE: replace resources/keys/encryption.key
                             and resources/keys/api-portal-encryption.key even if
                             they already exist. Any data encrypted
                             under the old key(s) (e.g. stored secrets) becomes
                             permanently unreadable. Requires interactive
                             confirmation unless ADMIN_USERNAME/ADMIN_PASSWORD
                             are set (CI), in which case passing this flag is
                             itself treated as confirmation.
  --profiles=<a,b,...>      override the default COMPOSE_PROFILES value this
                             script writes to .env, e.g. --profiles=all or
                             --profiles=platform-api

ADMIN_USERNAME / ADMIN_PASSWORD environment variables skip the interactive
prompts and pin the credentials (used by CI).

Compose project name (data isolation):
  Setup writes COMPOSE_PROJECT_NAME into .env on the first run, unique to this copy of
  the distribution. It prefixes every container, network, and volume, so another
  extraction of this zip on the same host cannot bind to this stack's volumes (the
  Platform API's database, the API Portal's data).
  The name is pinned once and never changes afterwards - not on a rerun, not with
  --force, not with --rotate-encryption-key. A new name would leave the running data
  behind in the old volumes and start the stack with an empty database. Deleting .env
  has that same effect, so leave it in place.
  To choose the name yourself, set COMPOSE_PROJECT_NAME in the environment for the
  FIRST run:
    COMPOSE_PROJECT_NAME=my-workspace ./scripts/setup.sh
  It must match ^[a-z0-9][a-z0-9_-]*$ (no dots, no uppercase - docker compose rejects
  those).
  That is also the upgrade path from a distribution that had no .env: its data lives in
  volumes prefixed with the old extracted-folder name (find it with: docker volume ls),
  so pass that name on the first run to keep it.
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
# portals/api-portal, or the root of a distribution zip via
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
    fail "could not find docker-compose.yaml in the current directory, next to this script, or its parent directory. Run this from portals/ai-workspace, portals/api-portal, or a distribution zip's root."
fi
cd "$ROOT_DIR"

ENV_FILE="$ROOT_DIR/api-platform.env"
# Project-level env file — Docker Compose reads COMPOSE_PROFILES from here
# automatically (unlike api-platform.env above, which is only ever passed to
# containers explicitly via each service's own env_file: entry), so this is
# what makes a plain `docker compose up -d` resolve the right services.
COMPOSE_ENV_FILE="$ROOT_DIR/.env"
# COMPOSE_PROJECT_NAME goes in the same file. The full prefix is
# "$PROJECT_NAME_PREFIX-$PACK", so the two packs never share a project name.
PROJECT_NAME_PREFIX="wso2apip"
PROJECT_NAME=""
CERTS_DIR="$ROOT_DIR/resources/certificates"
# RS256 JWT keypair (PEM). Mounted into the platform-api container at
# /etc/platform-api/keys and read by config.toml via {{ file }}.
KEYS_DIR="$ROOT_DIR/resources/keys"

# Every service image runs as this fixed non-root UID (see platform-api,
# ai-workspace, and api-portal Dockerfiles).
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

# One-time confirmation gate for --rotate-encryption-key, shared by both the
# API Portal and Platform API at-rest encryption keys (rotating either makes
# data encrypted under it permanently unreadable) — prompts at most once per
# run even though both keys are provisioned separately below.
ROTATION_CONFIRMED=false
confirm_rotation_once() {
    local key_path="$1"
    if [[ "$ROTATION_CONFIRMED" == true ]]; then
        return
    fi
    if [[ -t 0 && -z "${ADMIN_USERNAME:-}" && -z "${ADMIN_PASSWORD:-}" ]]; then
        echo
        echo "  WARNING: --rotate-encryption-key will make any data encrypted"
        echo "  under the current at-rest encryption key(s) (starting with"
        echo "  $key_path) permanently unreadable."
        read -r -p "  Type 'rotate' to proceed: " CONFIRM_ROTATE
        [[ "$CONFIRM_ROTATE" == "rotate" ]] || fail "encryption key rotation not confirmed; aborting."
    else
        log "  - non-interactive/CI invocation: --rotate-encryption-key passed, treating that as confirmation"
    fi
    ROTATION_CONFIRMED=true
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
        # macOS/BSD sed and GNU sed both accept -i.bak with an explicit suffix.
        sed -i.bak "/^${key}=/d" "$file" && rm -f "$file.bak"
    fi
    printf '%s=%s\n' "$key" "$value" >> "$file"
    log "  - ${key} set in $(basename "$file")"
}

# Which pack's docker-compose.yaml this is — a single source of truth reused
# both to resolve the default COMPOSE_PROFILES value below and to print the
# right profile combinations at the end of this script. Prefers the value
# `make dist` baked into DEFAULT_PACK_NAME; falls back to this pack's own
# directory name, since both packs ship the same services and differ only in
# which one is their headline component.
detect_pack() {
    if [[ "$DEFAULT_PACK_NAME" != "__DEFAULT_PACK_NAME__" ]]; then
        echo "$DEFAULT_PACK_NAME"
        return
    fi
    local dir
    dir="$(basename "$ROOT_DIR")"
    if [[ "$dir" == "api-portal" ]]; then
        echo "api-portal"
    elif [[ "$dir" == "ai-workspace" ]]; then
        echo "ai-workspace"
    else
        echo "unknown"
    fi
}
PACK="$(detect_pack)"

# --- Compose project name -------------------------------------------------------
# Compose accepts ^[a-z0-9][a-z0-9_-]*$ only — no dots, no uppercase. $2 is an optional
# extra line naming where the rejected value came from.
validate_project_name() {
    [[ "$1" =~ ^[a-z0-9][a-z0-9_-]*$ ]] && return
    echo "[setup] ERROR: invalid project name '$1' — must start with a lowercase letter or digit and contain only [a-z0-9_-]" >&2
    [[ -n "${2:-}" ]] && echo "$2" >&2
    exit 2
}

# This pack's version: the value `make dist` baked in for a distribution zip,
# else the VERSION file a repo checkout has. Neither dist target copies VERSION
# into the zip, which is why the baked placeholder exists at all.
dist_version() {
    if [[ "$DEFAULT_DIST_VERSION" != "__DEFAULT_DIST_VERSION__" ]]; then
        printf '%s' "$DEFAULT_DIST_VERSION"
    elif [[ -f "$ROOT_DIR/VERSION" ]]; then
        tr -d ' \t\r\n' < "$ROOT_DIR/VERSION"
    fi
}

# <prefix>-<pack>-<sanitized version>-<6 hex>, e.g. wso2apip-ai-workspace-1-0-0-rc-a3f19c.
# The version is cosmetic: if it can't be read or sanitizes to nothing, fall back to
# <prefix>-<pack>-<6 hex> rather than failing setup. The suffix alone is what makes
# the name unique.
gen_project_name() {
    local version suffix prefix candidate
    prefix="$PROJECT_NAME_PREFIX-$PACK"
    version="$(dist_version)"
    version="${version%-SNAPSHOT}"
    version="$(printf '%s' "$version" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9_-' '-' | tr -s '-' | sed 's/^-*//; s/-*$//')"
    suffix="$(openssl rand -hex 3)"
    candidate="$prefix-$suffix"
    if [[ -n "$version" && "$prefix-$version-$suffix" =~ ^[a-z0-9][a-z0-9_-]*$ ]]; then
        candidate="$prefix-$version-$suffix"
    fi
    printf '%s' "$candidate"
}

# Reads the value the way compose itself does — it strips whitespace around an unquoted
# value, so "  name  " and "name" are the same pin and must validate the same way.
read_dotenv_project_name() {
    [[ -f "$COMPOSE_ENV_FILE" ]] || return 0
    local line
    line="$(grep -E '^[[:space:]]*COMPOSE_PROJECT_NAME=' "$COMPOSE_ENV_FILE" | tail -n1 || true)"
    [[ -n "$line" ]] || return 0
    line="${line#*=}"          # value only
    line="${line%$'\r'}"       # tolerate a CRLF-written .env
    line="${line//\"/}"        # unquote
    line="${line//\'/}"
    line="${line#"${line%%[![:space:]]*}"}"   # trim leading whitespace
    line="${line%"${line##*[![:space:]]}"}"   # trim trailing whitespace
    printf '%s' "$line"
}

# Key-scoped write: .env already holds COMPOSE_PROFILES (and possibly a dev's own
# keys), so never rewrite the whole file. set_env_var() cannot be used here — it
# replaces the value whenever --force is passed, which is exactly what must never
# happen to this key.
write_dotenv_project_name() {
    local name="$1" tmp
    if [[ ! -f "$COMPOSE_ENV_FILE" ]]; then
        cat > "$COMPOSE_ENV_FILE" <<EOF
# Generated by scripts/setup.sh on $(date -u +"%Y-%m-%dT%H:%M:%SZ").
# Read automatically by "docker compose" from this directory. NOT passed to the containers.
#
# COMPOSE_PROJECT_NAME namespaces every container, network, and volume of this stack.
# It is unique to THIS copy of the distribution, so another extraction of the same zip
# elsewhere on this host cannot bind to this stack's volumes.
#
# Do not change or delete this line after the first start: the running data lives in
# volumes named <project>_platform-api-data etc., and a different name means the stack
# starts with an empty database.
COMPOSE_PROJECT_NAME=$name
EOF
        return
    fi
    if grep -qE '^[[:space:]]*COMPOSE_PROJECT_NAME=' "$COMPOSE_ENV_FILE"; then
        tmp="$(mktemp "${COMPOSE_ENV_FILE}.XXXXXX")"
        awk -v val="$name" '
            /^[[:space:]]*COMPOSE_PROJECT_NAME=/ {
                if (!seen) { print "COMPOSE_PROJECT_NAME=" val; seen = 1 }
                next
            }
            { print }
        ' "$COMPOSE_ENV_FILE" > "$tmp"
        # Copy over the original rather than mv, so the existing file's permissions stay.
        cat "$tmp" > "$COMPOSE_ENV_FILE"
        rm -f "$tmp"
    else
        cat >> "$COMPOSE_ENV_FILE" <<EOF

# Added by scripts/setup.sh on $(date -u +"%Y-%m-%dT%H:%M:%SZ").
# COMPOSE_PROJECT_NAME namespaces every container, network, and volume of this stack.
# It is unique to THIS copy of the distribution, so another extraction of the same zip
# elsewhere on this host cannot bind to this stack's volumes.
#
# Do not change or delete this line after the first start: the running data lives in
# volumes named <project>_platform-api-data etc., and a different name means the stack
# starts with an empty database.
COMPOSE_PROJECT_NAME=$name
EOF
    fi
}

# Written once, on the first run, then never rotated — not on a rerun, not under
# --force, not under --rotate-encryption-key. Rotating it would orphan the live
# volumes and hand the operator an empty stack, which looks exactly like the bug
# this pinning exists to fix. An operator who wants a specific name (including the
# old extracted-folder name, to adopt a pre-.env install's volumes) sets
# COMPOSE_PROJECT_NAME in the environment for that first run.
provision_project_name() {
    local existing chosen source
    existing="$(read_dotenv_project_name)"

    # An empty "COMPOSE_PROJECT_NAME=" line counts as unset — Compose would fall
    # back to the directory name, which is the collision being fixed.
    if [[ -n "$existing" ]]; then
        # A hand-edited pin gets the same check as a generated one — otherwise setup reports
        # success on a stack docker compose then refuses to start.
        validate_project_name "$existing" "               from the COMPOSE_PROJECT_NAME line in .env"
        PROJECT_NAME="$existing"
        log "  - .env already pins COMPOSE_PROJECT_NAME=$existing — keeping it"
        # docker compose reads the shell environment ahead of .env, so a stale export
        # here silently sends compose at a different project than this stack is pinned to.
        if [[ -n "${COMPOSE_PROJECT_NAME:-}" && "$COMPOSE_PROJECT_NAME" != "$existing" ]]; then
            log "    note: COMPOSE_PROJECT_NAME=$COMPOSE_PROJECT_NAME is exported in this shell and takes"
            log "          precedence over .env for every docker compose command run from it."
        fi
        return
    fi

    if [[ -n "${COMPOSE_PROJECT_NAME:-}" ]]; then
        chosen="$COMPOSE_PROJECT_NAME"; source="COMPOSE_PROJECT_NAME from the environment"
    else
        chosen="$(gen_project_name)"; source="generated"
    fi

    validate_project_name "$chosen"
    write_dotenv_project_name "$chosen"
    PROJECT_NAME="$chosen"
    log "  - COMPOSE_PROJECT_NAME=$chosen written to .env ($source)"
}

log "Setting default docker compose profiles ..."
# Resolution order: an explicit --profiles=<...> flag always wins; otherwise
# use the value `make dist` baked into DEFAULT_COMPOSE_PROFILES for a
# distribution zip; otherwise auto-detect from docker-compose.yaml, which is
# what happens when this script runs straight out of a repo checkout.
if [[ -n "$PROFILES_OVERRIDE" ]]; then
    COMPOSE_PROFILES_VALUE="$PROFILES_OVERRIDE"
elif [[ "$DEFAULT_COMPOSE_PROFILES" != "__DEFAULT_COMPOSE_PROFILES__" ]]; then
    COMPOSE_PROFILES_VALUE="$DEFAULT_COMPOSE_PROFILES"
else
    case "$PACK" in
        ai-workspace) COMPOSE_PROFILES_VALUE="ai-workspace,platform-api" ;;
        api-portal) COMPOSE_PROFILES_VALUE="api-portal,platform-api" ;;
        *) COMPOSE_PROFILES_VALUE="" ;;
    esac
fi

if [[ -n "$COMPOSE_PROFILES_VALUE" ]]; then
    touch "$COMPOSE_ENV_FILE"
    set_env_var "$COMPOSE_ENV_FILE" "COMPOSE_PROFILES" "$COMPOSE_PROFILES_VALUE"
else
    log "  - could not detect this pack's default profiles; pass --profiles=<a,b,...> or always use --profile explicitly with docker compose"
fi

# Same file, same section — .env is already created above and $PACK is resolved, and
# this one call site covers every path including the --certs-only exit below (which
# `make bff-run` uses, and which has already written .env by this point).
log "Provisioning the docker compose project name ..."
provision_project_name

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
        -addext "subjectAltName=DNS:localhost,DNS:*.localhost,DNS:platform-api,DNS:ai-workspace,DNS:api-portal,DNS:host.docker.internal,IP:127.0.0.1" \
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

log "Provisioning API Portal encryption key and session secret ..."
# Written to files (not api-platform.env) and read by config.toml via
# {{ file "/etc/api-portal/keys/encryption.key" }} / {{ file ".../session-secret" }}
# — the same pattern as the Platform API's at-rest encryption key below, and
# for the same reason: a value that never appears in `docker inspect`, a
# process environment dump, or api-platform.env is materially harder to
# exfiltrate than an env var. resources/keys is mounted into the API Portal
# container at /etc/api-portal/keys (see docker-compose.yaml), which is on
# API Portal's {{ file }} allowlist.
#
# api-portal-encryption.key encrypts subscription/webhook secrets at rest
# (see src/dao/subscriptionDao.js, webhookSubscriberDao.js) — exactly like
# the Platform API's encryption.key below, so it is preserved across reruns
# and rotated ONLY via --rotate-encryption-key, never by the generic --force.
# api-portal-session-secret only signs session cookies — rotating it merely
# invalidates existing sessions, so it follows --force like the TLS cert/JWT
# keypair.
if [[ -f "$KEYS_DIR/api-portal-encryption.key" && "$ROTATE_ENCRYPTION_KEY" == true ]]; then
    confirm_rotation_once "$KEYS_DIR/api-portal-encryption.key"
    openssl rand -hex 32 > "$KEYS_DIR/api-portal-encryption.key"
    restrict_secret_file "$KEYS_DIR/api-portal-encryption.key"
    log "  - API Portal encryption key ROTATED at $KEYS_DIR/api-portal-encryption.key"
elif [[ -f "$KEYS_DIR/api-portal-encryption.key" ]]; then
    log "  - $KEYS_DIR/api-portal-encryption.key already exists, leaving as-is (pass --rotate-encryption-key to replace it)"
else
    mkdir -p "$KEYS_DIR"
    openssl rand -hex 32 > "$KEYS_DIR/api-portal-encryption.key"
    restrict_secret_file "$KEYS_DIR/api-portal-encryption.key"
    log "  - API Portal encryption key generated at $KEYS_DIR/api-portal-encryption.key"
fi
if [[ "$FORCE" == false && -f "$KEYS_DIR/api-portal-session-secret" ]]; then
    log "  - $KEYS_DIR/api-portal-session-secret already exists, leaving as-is"
else
    mkdir -p "$KEYS_DIR"
    openssl rand -hex 32 > "$KEYS_DIR/api-portal-session-secret"
    restrict_secret_file "$KEYS_DIR/api-portal-session-secret"
    log "  - API Portal session secret generated at $KEYS_DIR/api-portal-session-secret"
fi

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
    confirm_rotation_once "$KEYS_DIR/encryption.key"
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
    echo "   (It is stored, bcrypt-hashed, in api-platform.env's APIP_CP_ADMIN_PASSWORD_HASH)"
    echo "  ------------------------------------------------------------------"
    echo
fi
echo "  Compose project:  ${PROJECT_NAME}   (pinned in .env)"
echo
echo "  Next step — choose which components:"
echo
# The first line always reflects $COMPOSE_PROFILES_VALUE — the value actually
# just written to .env — rather than assuming $PACK's usual default. Those
# can differ: --profiles=<...> or a dist-baked DEFAULT_COMPOSE_PROFILES can
# set .env to something other than this pack's normal two-service combo.
if [[ -n "$COMPOSE_PROFILES_VALUE" ]]; then
    echo "    docker compose up -d                                                    # ${COMPOSE_PROFILES_VALUE} (current .env default)"
else
    echo "    docker compose up -d                                                    # no default set — pass --profile explicitly"
fi
if [[ "$PACK" == "api-portal" ]]; then
    echo "    docker compose --profile api-portal --profile ai-workspace --profile platform-api up -d  # API Portal + AI Workspace + Platform API"
    echo "    docker compose --profile all up -d                                                              # AI Workspace + API Portal + Platform API"
    echo "    docker compose --profile platform-api up -d                                                     # Platform API only"
elif [[ "$PACK" == "ai-workspace" ]]; then
    echo "    docker compose --profile ai-workspace --profile api-portal --profile platform-api up -d  # AI Workspace + API Portal + Platform API"
    echo "    docker compose --profile all up -d                                                             # AI Workspace + API Portal + Platform API"
    echo "    docker compose --profile platform-api up -d                                                    # Platform API only"
else
    echo "    docker compose --profile <ai-workspace|api-portal|all|platform-api> up -d"
fi
echo
