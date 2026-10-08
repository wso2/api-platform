#!/usr/bin/env bash
# Boot platform-api from the current working tree: SQLite, internal_token auth without signature
# validation, plain HTTP for curl + HTTPS (self-signed) for gateways. Runs in the background.
set -euo pipefail
. "$(dirname "$0")/lib.sh"

ROOT=$(repo_root)
[ -f "$ROOT/platform-api/cmd/main.go" ] || die "no platform-api/ under $ROOT (set PAPI_REPO)"

for p in "$PAPI_HTTP_PORT" "$PAPI_HTTPS_PORT"; do
  pid=$(port_pid "$p"); [ -z "$pid" ] || die "port $p is in use by pid $pid ($(ps -o comm= -p "$pid")); stop it or set PAPI_HTTP_PORT/PAPI_HTTPS_PORT"
done

new_workdir   # fresh temp work dir for this run (state, logs, DB, gateway copies)
log "work dir: $WORK"
PDIR="$WORK/platform-api"
mkdir -p "$PDIR"

[ -f "$PDIR/encryption.key" ] || openssl rand -hex 32 > "$PDIR/encryption.key"
if [ ! -f "$PDIR/cert.pem" ]; then
  # Gateways dial wss://host.docker.internal:<https port>; insecure_skip_verify is on in the
  # shipped gateway configs, the SANs keep curl --cacert usable too.
  openssl req -x509 -newkey rsa:2048 -nodes -days 365 -keyout "$PDIR/key.pem" -out "$PDIR/cert.pem" \
    -subj /CN=localhost -addext "subjectAltName=DNS:localhost,DNS:host.docker.internal,IP:127.0.0.1" >/dev/null 2>&1
fi

cat > "$PDIR/config.toml" <<TOML
[platform_api.logging]
level = "${PAPI_LOG_LEVEL:-info}"

[platform_api.security]
encryption_key = "$(cat "$PDIR/encryption.key")"

[platform_api.database]
driver = "sqlite3"
path   = "$PDIR/api_platform.db"

# Inbound JWTs are parsed without signature, exp or issuer checks (local testing only).
[platform_api.auth]
mode = "internal_token"

[platform_api.auth.internal_token]
skip_validation = true

[platform_api.auth.authorization]
enabled = true
mode    = "scope"

[platform_api.server.http]
enabled = true
port    = ${PAPI_HTTP_PORT}

[platform_api.server.https]
enabled   = true
port      = ${PAPI_HTTPS_PORT}
cert_file = "$PDIR/cert.pem"
key_file  = "$PDIR/key.pem"
TOML

# Build first so the listener is the binary itself (go run would leave a child behind on kill).
log "building platform-api from $ROOT/platform-api"
( cd "$ROOT/platform-api" && go build -o "$PDIR/platform-api" ./cmd ) > "$LOGS/platform-api-build.log" 2>&1 \
  || { tail -30 "$LOGS/platform-api-build.log"; die "platform-api build failed"; }

# Relative defaults (schema, openapi.yaml, LLM templates) resolve against platform-api/.
# exec + full redirection: no shell lingers holding the caller's stdout (a piped caller would hang).
( cd "$ROOT/platform-api" && exec "$PDIR/platform-api" -config "$PDIR/config.toml" ) \
  > "$LOGS/platform-api.log" 2>&1 < /dev/null &
PAPI_PID=$!   # the exec'd server itself; teardown stops only this pid

wait_for 90 "platform-api /health" curl -sf "$PAPI_URL/health" \
  || { tail -40 "$LOGS/platform-api.log"; die "platform-api did not come up"; }
state_set PAPI_REPO "$ROOT"
state_set PAPI_PID "$PAPI_PID"
log "platform-api: $PAPI_URL (curl), https://localhost:${PAPI_HTTPS_PORT} (gateways); log $LOGS/platform-api.log"
