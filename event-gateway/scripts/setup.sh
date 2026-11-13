#!/usr/bin/env bash
# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License. You may obtain a copy of the
# License at http://www.apache.org/licenses/LICENSE-2.0
# --------------------------------------------------------------------
# Generates the self-signed HTTPS listener certificate/key that
# docker-compose.yaml mounts for both gateway-controller and event-gateway
# (./listener-certs, next to this script's parent directory). Works the same
# whether run from a repo checkout (event-gateway/) or an extracted
# distribution zip — both lay out listener-certs/ as a sibling of
# docker-compose.yaml.
#
# Never generates anything for docker-compose.dev.yaml or the
# docker-compose.test.*.yaml files — those intentionally reuse the main
# gateway's own listener cert (../gateway/gateway-controller/listener-certs,
# via gateway/scripts/setup.sh) so a local checkout doesn't end up with two
# different self-signed certs for the two stacks.
set -euo pipefail
cd "$(dirname "$0")/.."

CERTS_DIR="listener-certs"
FORCE=false
[[ "${1:-}" == "--force" ]] && FORCE=true

log() { echo "[setup] $*"; }

command -v openssl >/dev/null 2>&1 || { echo "error: openssl is required" >&2; exit 1; }

if [[ "$FORCE" == false && -f "$CERTS_DIR/default-listener.crt" && -f "$CERTS_DIR/default-listener.key" ]]; then
  log "$CERTS_DIR/default-listener.crt already exists — keeping it (rerun with --force to rotate it)"
  exit 0
fi

mkdir -p "$CERTS_DIR"
openssl req -x509 -newkey rsa:2048 -sha256 -days 365 -nodes \
  -keyout "$CERTS_DIR/default-listener.key" -out "$CERTS_DIR/default-listener.crt" \
  -subj "/O=WSO2 API Platform/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,DNS:*.localhost,DNS:host.docker.internal,IP:127.0.0.1" >/dev/null 2>&1

# Both gateway-controller and event-gateway read this key, as different non-root
# container UIDs (gateway-controller: 10001; event-gateway: an Alpine
# adduser-assigned UID that isn't fixed or discoverable ahead of time). This is a
# per-install, self-signed, localhost-only dev/test cert, not a production
# secret, so world-readable is a deliberate, proportionate choice here — and
# still a large improvement over committing it to git, which is strictly more
# exposed than "other local users on this one host."
chmod 644 "$CERTS_DIR/default-listener.crt" "$CERTS_DIR/default-listener.key"
log "self-signed listener certificate generated at $CERTS_DIR/default-listener.crt"
