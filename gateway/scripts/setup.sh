#!/usr/bin/env bash
# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License. You may obtain a copy of the
# License at http://www.apache.org/licenses/LICENSE-2.0
# --------------------------------------------------------------------
# Gateway quickstart setup. See README.md -> "Run".
#
# The server never auto-generates keys or certificates and has no demo/development
# mode: this script provisions everything the gateway needs, and the server fails
# closed with a descriptive error if a required key or certificate is missing.
#
# Provisions:
#   - listener-certs/default-listener.{crt,key}   : router HTTPS listener certificate
#   - aesgcm-keys/default-aesgcm256-v1.bin         : AES-256 at-rest encryption key. The gateway's
#       docker compose bind-mounts this host file into the controller.
#   - api-platform.env                            : required runtime defaults for the gateway-runtime
#   - .env                                        : COMPOSE_PROJECT_NAME, unique to this copy of the
#       distribution. Read by the docker compose CLI (not by the containers) to prefix every container,
#       network, and volume, so another extraction of the same zip on this host cannot bind to this
#       stack's volumes — its gateway database and dynamically-managed certificates.
set -euo pipefail
cd "$(dirname "$0")"
# Distribution layout: scripts/setup.sh, one level below docker-compose.yaml.
[[ -f docker-compose.yaml ]] || cd ..

ENV_FILE="api-platform.env"

# Compose project name. DOTENV_FILE is what the docker compose CLI auto-loads from the
# project directory; unrelated to $ENV_FILE, which is an env_file: mounted into the services.
DOTENV_FILE=".env"
PROJECT_NAME_PREFIX="wso2apip-gateway"
PROJECT_NAME=""

# Router downstream (HTTPS ingress) listener cert/key. Referenced by
# [router.downstream_tls] in config.toml as ./listener-certs/default-listener.{crt,key}
# and mounted into the gateway-controller. The repo checkout keeps these under
# gateway-controller/listener-certs; the distribution zip stages them under resources/.
if [[ -d gateway-controller/listener-certs ]]; then
  CERTS_DIR="gateway-controller/listener-certs"
else
  CERTS_DIR="resources/listener-certs"
fi

# AES-256 at-rest encryption key.
if [[ -d gateway-controller ]]; then
  ENC_KEY_FILE="gateway-controller/aesgcm-keys/default-aesgcm256-v1.bin"
else
  ENC_KEY_FILE="resources/aesgcm-keys/default-aesgcm256-v1.bin"
fi

# Controller config file — read only to detect whether basic auth is enabled,
# which determines whether admin credentials are required in api-platform.env.
CONFIG_FILE="configs/config.toml"

FORCE=false
CERTS_ONLY=false

for arg in "$@"; do
  case "$arg" in
    --force) FORCE=true ;;
    --certs-only) CERTS_ONLY=true ;;
    -h|--help)
      cat <<'EOF'
Usage: ./scripts/setup.sh [--force] [--certs-only]

  --force        regenerate the certificate and encryption key (rotates them), rewrite api-platform.env,
                 and re-provision the admin credentials (rotates the password)
  --certs-only   generate only the listener TLS certificate (skip the encryption key, api-platform.env,
                 and .env)

Admin credentials (gateway-controller REST/management API basic auth):
  Set ADMIN_USERNAME and/or ADMIN_PASSWORD in the environment to run non-interactively (CI).
  When unset and stdin is a TTY the script prompts; username defaults to "admin" and an empty
  password is randomly generated. Only the bcrypt hash is stored (in api-platform.env); the
  plaintext password is printed once and never written to disk.

Compose project name (data isolation):
  Setup writes COMPOSE_PROJECT_NAME into .env on the first run, unique to this copy of the
  distribution. It prefixes every container, network, and volume, so another extraction of this
  zip on the same host cannot bind to this stack's volumes (its gateway database and
  dynamically-managed certificates).
  The name is pinned once and never changes afterwards - not on a rerun, not with --force. A new
  name would leave the running data behind in the old volumes and start the gateway with an empty
  database. Deleting .env has that same effect, so leave it in place.
  To choose the name yourself, set COMPOSE_PROJECT_NAME in the environment for the FIRST run:
    COMPOSE_PROJECT_NAME=my-gateway ./scripts/setup.sh
  It must match ^[a-z0-9][a-z0-9_-]*$ (no dots, no uppercase - docker compose rejects those).
  That is also the upgrade path from a distribution that had no .env: its data lives in volumes
  prefixed with the old extracted-folder name (find it with: docker volume ls), so pass that name
  on the first run to keep it.

The control-plane connection is optional and is NOT configured here: to connect to a control
plane, add APIP_GW_CONTROLLER_CONTROLPLANE_HOST and APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN to
api-platform.env by hand (both default to empty = standalone mode).
EOF
      exit 0
      ;;
    *) echo "unknown option: $1 (try --help)" >&2; exit 2 ;;
  esac
  shift
done

command -v openssl >/dev/null 2>&1 || { echo "error: openssl is required" >&2; exit 1; }

log() { echo "[setup] $*"; }

# Every gateway image runs as this fixed non-root UID
CONTAINER_UID=10001

# Non-sensitive files (the public listener certificate) — left world-readable so
# the container UID above can read them across the bind mount.
FILE_MODE=644

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

# bcrypt isn't in openssl; use htpasswd when available, else the httpd image.
# The gateway-controller's basic authenticator accepts bcrypt ($2a/$2b/$2y$) hashes.
bcrypt_hash() {
  local password="$1"
  if command -v htpasswd >/dev/null 2>&1; then
    printf '%s' "$password" | htpasswd -niB -C 10 "" | cut -d: -f2 | tr -d '\r\n'
  elif command -v docker >/dev/null 2>&1; then
    printf '%s' "$password" | docker run --rm -i httpd:2.4-alpine htpasswd -niB -C 10 "" | cut -d: -f2 | tr -d '\r\n'
  else
    echo "error: need either htpasswd (apache2-utils / httpd-tools) or docker to bcrypt-hash the admin password" >&2
    exit 1
  fi
}

gen_cert() {
  if [[ "$FORCE" == false && -f "$CERTS_DIR/default-listener.crt" && -f "$CERTS_DIR/default-listener.key" ]]; then
    chmod "$FILE_MODE" "$CERTS_DIR/default-listener.crt"
    restrict_secret_file "$CERTS_DIR/default-listener.key"
    log "  - $CERTS_DIR/default-listener.crt already exists — keeping it"
    return
  fi
  mkdir -p "$CERTS_DIR"
  openssl req -x509 -newkey rsa:2048 -sha256 -days 365 -nodes \
    -keyout "$CERTS_DIR/default-listener.key" -out "$CERTS_DIR/default-listener.crt" \
    -subj "/O=WSO2 API Platform/CN=localhost" \
    -addext "subjectAltName=DNS:localhost,DNS:*.localhost,DNS:host.docker.internal,IP:127.0.0.1" >/dev/null 2>&1
  chmod "$FILE_MODE" "$CERTS_DIR/default-listener.crt"
  restrict_secret_file "$CERTS_DIR/default-listener.key"
  log "  - self-signed listener certificate generated at $CERTS_DIR/default-listener.crt"
}

gen_encryption_key() {
  if [[ "$FORCE" == false && -f "$ENC_KEY_FILE" ]]; then
    restrict_secret_file "$ENC_KEY_FILE"
    log "  - $ENC_KEY_FILE already exists — keeping it"
    return
  fi
  mkdir -p "$(dirname "$ENC_KEY_FILE")"
  ( umask 177; openssl rand 32 > "$ENC_KEY_FILE" )
  restrict_secret_file "$ENC_KEY_FILE"
  log "  - AES-256 encryption key generated at $ENC_KEY_FILE"
}

# --- Compose project name -------------------------------------------------------
# Compose accepts ^[a-z0-9][a-z0-9_-]*$ only — no dots, no uppercase. $2 is an optional
# extra line naming where the rejected value came from.
validate_project_name() {
  [[ "$1" =~ ^[a-z0-9][a-z0-9_-]*$ ]] && return
  echo "error: invalid project name '$1' — must start with a lowercase letter or digit and contain only [a-z0-9_-]" >&2
  [[ -n "${2:-}" ]] && echo "$2" >&2
  exit 2
}

# gateway.version from build.yaml — NOT the top-level "version: v1" schema key above it.
dist_version() {
  [[ -f build.yaml ]] || return 0
  awk '
    /^gateway:[[:space:]]*$/         { in_gw = 1; next }
    /^[^[:space:]#]/                 { in_gw = 0 }
    in_gw && /^[[:space:]]+version:/ {
      sub(/^[[:space:]]*version:[[:space:]]*/, "")
      gsub(/[\047"]/, "")
      print; exit
    }
  ' build.yaml 2>/dev/null || true
}

# <prefix>-<sanitized version>-<6 hex>, e.g. wso2apip-gateway-1-2-0-rc-a3f19c.
# The version is cosmetic: if build.yaml can't be read or sanitizes to nothing, fall
# back to <prefix>-<6 hex> rather than failing setup. The suffix alone is what makes
# the name unique.
gen_project_name() {
  local version suffix candidate
  version="$(dist_version)"
  version="${version%-SNAPSHOT}"
  version="$(printf '%s' "$version" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9_-' '-' | tr -s '-' | sed 's/^-*//; s/-*$//')"
  suffix="$(openssl rand -hex 3)"
  candidate="$PROJECT_NAME_PREFIX-$suffix"
  if [[ -n "$version" && "$PROJECT_NAME_PREFIX-$version-$suffix" =~ ^[a-z0-9][a-z0-9_-]*$ ]]; then
    candidate="$PROJECT_NAME_PREFIX-$version-$suffix"
  fi
  printf '%s' "$candidate"
}

# Reads the value the way compose itself does — it strips whitespace around an unquoted
# value, so "  name  " and "name" are the same pin and must validate the same way.
read_dotenv_project_name() {
  [[ -f "$DOTENV_FILE" ]] || return 0
  local line
  line="$(grep -E '^[[:space:]]*COMPOSE_PROJECT_NAME=' "$DOTENV_FILE" | tail -n1 || true)"
  [[ -n "$line" ]] || return 0
  line="${line#*=}"          # value only
  line="${line%$'\r'}"       # tolerate a CRLF-written .env
  line="${line//\"/}"        # unquote
  line="${line//\'/}"
  line="${line#"${line%%[![:space:]]*}"}"   # trim leading whitespace
  line="${line%"${line##*[![:space:]]}"}"   # trim trailing whitespace
  printf '%s' "$line"
}

# Key-scoped write: .env may already hold unrelated keys (a dev's MSSQL_SA_PASSWORD,
# read by docker-compose.sqlserver.yaml), so never rewrite the whole file.
write_dotenv_project_name() {
  local name="$1" tmp
  if [[ ! -f "$DOTENV_FILE" ]]; then
    cat > "$DOTENV_FILE" <<EOF
# Generated by scripts/setup.sh on $(date -u +"%Y-%m-%dT%H:%M:%SZ").
# Read automatically by "docker compose" from this directory. NOT passed to the containers.
#
# COMPOSE_PROJECT_NAME namespaces every container, network, and volume of this stack. It is
# unique to THIS copy of the distribution, so another extraction of the same zip elsewhere on
# this host cannot bind to this stack's volumes.
#
# Do not change or delete this file after the first start: the running data lives in volumes
# named <project>_controller-data etc., and a different name means the gateway starts with an
# empty database.
COMPOSE_PROJECT_NAME=$name
EOF
    return
  fi
  if grep -qE '^[[:space:]]*COMPOSE_PROJECT_NAME=' "$DOTENV_FILE"; then
    tmp="$(mktemp "${DOTENV_FILE}.XXXXXX")"
    awk -v val="$name" '
      /^[[:space:]]*COMPOSE_PROJECT_NAME=/ {
        if (!seen) { print "COMPOSE_PROJECT_NAME=" val; seen = 1 }
        next
      }
      { print }
    ' "$DOTENV_FILE" > "$tmp"
    # Copy over the original rather than mv, so the existing file's permissions stay.
    cat "$tmp" > "$DOTENV_FILE"
    rm -f "$tmp"
  else
    cat >> "$DOTENV_FILE" <<EOF

# Added by scripts/setup.sh on $(date -u +"%Y-%m-%dT%H:%M:%SZ").
# Namespaces every container, network, and volume of this stack; unique to this copy of the
# distribution. Changing it after the first start hides the existing data.
COMPOSE_PROJECT_NAME=$name
EOF
  fi
}

# Written once, on the first run, then never rotated — not on a rerun, not under --force.
# Rotating it would orphan the live volumes and hand the operator an empty gateway, which
# looks exactly like the bug this pinning exists to fix. An operator who wants a specific
# name (including the old extracted-folder name, to adopt a pre-.env install's volumes)
# sets COMPOSE_PROJECT_NAME in the environment for that first run.
provision_project_name() {
  local existing chosen source
  existing="$(read_dotenv_project_name)"

  if [[ -n "$existing" ]]; then
    # A hand-edited pin gets the same check as a generated one — otherwise setup reports
    # success on a stack docker compose then refuses to start.
    validate_project_name "$existing" "       from the COMPOSE_PROJECT_NAME line in $DOTENV_FILE"
    PROJECT_NAME="$existing"
    log "  - $DOTENV_FILE already pins COMPOSE_PROJECT_NAME=$existing — keeping it"
    # docker compose reads the shell environment ahead of .env, so a stale export here
    # silently sends compose at a different project than the one this stack is pinned to.
    if [[ -n "${COMPOSE_PROJECT_NAME:-}" && "$COMPOSE_PROJECT_NAME" != "$existing" ]]; then
      log "    note: COMPOSE_PROJECT_NAME=$COMPOSE_PROJECT_NAME is exported in this shell and takes"
      log "          precedence over $DOTENV_FILE for every docker compose command run from it."
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
  log "  - COMPOSE_PROJECT_NAME=$chosen written to $DOTENV_FILE ($source)"
}

log "Provisioning listener TLS certificate ..."
gen_cert

if [[ "$CERTS_ONLY" == true ]]; then
  exit 0
fi

log "Provisioning AES-256 encryption key ..."
gen_encryption_key

# Before the $ENV_FILE branch below, so both the fresh install and the "api-platform.env
# already exists" path provision it — the latter is what an operator upgrading from a
# distribution that predates .env hits.
log "Provisioning the docker compose project name ..."
provision_project_name

if [[ "$FORCE" == false && -f "$ENV_FILE" ]]; then
  log "$ENV_FILE already exists — keeping it (rerun with --force to rewrite it)"

  # Since the env file is kept as-is (not regenerated), it may predate the
  # current gateway and be missing settings the runtime now requires. Check
  # each required key and refuse to report success if any are absent/empty.
  missing=0
  check_env_key() {
    local key="$1" line
    line="$(grep -E "^[[:space:]]*${key}=" "$ENV_FILE" | tail -n1 || true)"
    if [[ -z "$line" ]]; then
      echo "    [MISSING] $key"; missing=$((missing + 1))
    elif [[ -z "${line#*=}" ]]; then
      echo "    [empty]   $key (defined but has no value)"; missing=$((missing + 1))
    else
      echo "    [ok]      $key"
    fi
  }

  # Admin credentials are required only when controller.auth.basic is enabled.
  # Detect that from config.toml so we don't demand them when basic auth is off.
  basic_auth_enabled() {
    [[ -f "$CONFIG_FILE" ]] || return 1
    awk '
      /^\[controller\.auth\.basic\][[:space:]]*$/ { in_sec = 1; next }
      /^\[/                                        { in_sec = 0 }
      in_sec && /^[[:space:]]*enabled[[:space:]]*=[[:space:]]*true/ { found = 1 }
      END { exit(found ? 0 : 1) }
    ' "$CONFIG_FILE"
  }

  echo
  log "Verify $ENV_FILE still defines the settings the gateway requires:"
  check_env_key GATEWAY_CONTROLLER_HOST
  check_env_key LOG_LEVEL
  if basic_auth_enabled; then
    echo "  Required because controller.auth.basic is enabled in $CONFIG_FILE:"
    check_env_key APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_USERNAME
    check_env_key APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_PASSWORD_HASH
  fi

  echo
  if [[ "$missing" -gt 0 ]]; then
    log "$missing required setting(s) are missing or empty — setup is NOT complete."
    echo
    echo "  Before starting the gateway, either:"
    echo "    - edit $ENV_FILE and fill in the value(s) flagged above, or"
    echo "    - rerun to regenerate it from scratch:  ./scripts/setup.sh --force"
    exit 1
  fi

  log "Setup complete."
  echo
  echo "  Compose project:  $PROJECT_NAME   (pinned in $DOTENV_FILE)"
  echo
  echo "  Next step:"
  echo "    docker compose up"
  exit 0
fi

# Admin credentials for the gateway-controller REST/management API (basic auth).
# Precedence: ADMIN_USERNAME/ADMIN_PASSWORD env vars > interactive prompt > defaults.
# Only the bcrypt hash is persisted; the plaintext password is shown once at the end.
GENERATED_PASSWORD="$(openssl rand -base64 24 | tr -d '/+=' | cut -c1-20)"

if [[ -z "${ADMIN_USERNAME:-}" && -t 0 ]]; then
  read -r -p "Admin username [press Enter to use the default username 'admin']: " ADMIN_USERNAME
fi
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"

if [[ -z "${ADMIN_PASSWORD:-}" && -t 0 ]]; then
  read -r -s -p "Admin password [press Enter to generate one]: " ADMIN_PASSWORD
  echo
fi
ADMIN_PASSWORD="${ADMIN_PASSWORD:-$GENERATED_PASSWORD}"

log "Provisioning admin credentials ..."
ADMIN_PASSWORD_HASH="$(bcrypt_hash "$ADMIN_PASSWORD")"
log "  - APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_PASSWORD_HASH generated (bcrypt)"

log "Writing $ENV_FILE ..."
umask 177
cat > "$ENV_FILE" <<EOF
# Generated by scripts/setup.sh on $(date -u +"%Y-%m-%dT%H:%M:%SZ").
# The admin password is not stored here; it was printed once by scripts/setup.sh.
#
# Required runtime settings — read directly by the gateway-runtime entrypoint / policy-engine:
GATEWAY_CONTROLLER_HOST=gateway-controller
LOG_LEVEL=info

APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_USERNAME=$ADMIN_USERNAME
APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_PASSWORD_HASH=$ADMIN_PASSWORD_HASH
EOF
umask 022
log "  - $ENV_FILE written"

echo
log "Setup complete."
echo
echo "  =================================================================="
echo "   ADMIN CREDENTIALS"
echo
echo "     Username:  $ADMIN_USERNAME"
echo "     Password:  $ADMIN_PASSWORD"
echo
echo "   !!  THIS PASSWORD WILL NOT BE SHOWN AGAIN — COPY IT NOW  !!"
echo "  =================================================================="
echo
echo "  Compose project:  $PROJECT_NAME   (pinned in $DOTENV_FILE)"
echo
echo "  Next step:"
echo "    docker compose up"
