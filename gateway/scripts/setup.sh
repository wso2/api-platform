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
set -euo pipefail
cd "$(dirname "$0")"
# Distribution layout: scripts/setup.sh, one level below docker-compose.yaml.
[[ -f docker-compose.yaml ]] || cd ..

ENV_FILE="api-platform.env"

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
  --certs-only   generate only the listener TLS certificate (skip the encryption key and api-platform.env)

Admin credentials (gateway-controller REST/management API basic auth):
  Set ADMIN_USERNAME and/or ADMIN_PASSWORD in the environment to run non-interactively (CI).
  When unset and stdin is a TTY the script prompts; username defaults to "admin" and an empty
  password is randomly generated. Only the bcrypt hash is stored (in api-platform.env); the
  plaintext password is printed once and never written to disk.

The control-plane connection is optional and is NOT configured here: to connect to a control
plane, add APIP_GW_CONTROLLER_CONTROLPLANE_HOST and APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN to
api-platform.env by hand (both default to empty = standalone mode).
EOF
      exit 0
      ;;
    *) echo "unknown option: $arg (try --help)" >&2; exit 2 ;;
  esac
done

command -v openssl >/dev/null 2>&1 || { echo "error: openssl is required" >&2; exit 1; }

log() { echo "[setup] $*"; }

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
    log "  - $CERTS_DIR/default-listener.crt already exists — keeping it"
    return
  fi
  mkdir -p "$CERTS_DIR"
  openssl req -x509 -newkey rsa:2048 -sha256 -days 365 -nodes \
    -keyout "$CERTS_DIR/default-listener.key" -out "$CERTS_DIR/default-listener.crt" \
    -subj "/O=WSO2 API Platform/CN=localhost" \
    -addext "subjectAltName=DNS:localhost,DNS:*.localhost,DNS:host.docker.internal,IP:127.0.0.1" >/dev/null 2>&1
  chmod 644 "$CERTS_DIR/default-listener.key" "$CERTS_DIR/default-listener.crt"
  log "  - self-signed listener certificate generated at $CERTS_DIR/default-listener.crt"
}

gen_encryption_key() {
  if [[ "$FORCE" == false && -f "$ENC_KEY_FILE" ]]; then
    log "  - $ENC_KEY_FILE already exists — keeping it"
    return
  fi
  mkdir -p "$(dirname "$ENC_KEY_FILE")"
  ( umask 177; openssl rand 32 > "$ENC_KEY_FILE" )
  log "  - AES-256 encryption key generated at $ENC_KEY_FILE"
}

log "Provisioning listener TLS certificate ..."
gen_cert

if [[ "$CERTS_ONLY" == true ]]; then
  exit 0
fi

log "Provisioning AES-256 encryption key ..."
gen_encryption_key

if [[ "$FORCE" == false && -f "$ENV_FILE" ]]; then
  log "$ENV_FILE already exists — keeping it (rerun with --force to rewrite it)"
  echo
  log "Setup complete."
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
  read -r -p "Admin username [admin]: " ADMIN_USERNAME
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
echo "  ------------------------------------------------------------------"
echo "   Gateway-controller admin login:  $ADMIN_USERNAME / $ADMIN_PASSWORD"
echo "   This password will not be shown again — copy it now."
echo "   (Only its bcrypt hash is stored, in $ENV_FILE)"
echo "  ------------------------------------------------------------------"
echo
echo "  Next step:"
echo "    docker compose up"
