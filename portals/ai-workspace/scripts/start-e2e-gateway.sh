#!/usr/bin/env bash
# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License. You may obtain a copy of the
# License at http://www.apache.org/licenses/LICENSE-2.0
# --------------------------------------------------------------------
# Bring up the AI Workspace stack WITH a registered AI gateway, so the E2E
# specs tagged for deployment can run.
#
# Why this is a script and not just another compose profile: the gateway
# authenticates to the control plane with a registration token that platform-api
# mints at run time. So the bring-up is necessarily two-phase —
#
#   phase 1  platform-api + ai-workspace, wait for health
#   bootstrap  log in -> POST /gateways -> POST /gateways/{id}/tokens
#   phase 2  gateway-controller + gateway-runtime, with that token injected
#
# — which is the same sequence tests/integration-e2e/suite_test.go performs for
# the godog suite. Re-running is safe: an existing gateway of the same id is
# reused and its token rotated.
set -euo pipefail

cd "$(dirname "$0")/.."

COMPOSE_FILES=(-f docker-compose.yaml -f docker-compose.gateway.yaml)
PLATFORM_API="${PLATFORM_API_URL:-https://localhost:9243}"
ADMIN_USER="${CYPRESS_ADMIN_USER:-admin}"
ADMIN_PASSWORD="${CYPRESS_ADMIN_PASSWORD:-admin}"
# Must match what the Cypress specs look for; keep in sync with
# cypress.config.js env.E2E_GATEWAY_NAME.
GATEWAY_ID="${E2E_GATEWAY_NAME:-e2e-gw}"
# The endpoint the control plane hands to clients as this gateway's ingress.
# 18080 is gateway-runtime's HTTP ingress as published by the overlay.
GATEWAY_ENDPOINT="${E2E_GATEWAY_ENDPOINT:-http://localhost:18080}"

# Self-signed certs everywhere in a local stack, hence -k throughout.
curl_cp() { curl -sk --max-time 15 "$@"; }

log() { printf '==> %s\n' "$*"; }

# --- Phase 1: control plane ------------------------------------------------
log "Starting control plane (platform-api + ai-workspace)"
docker compose "${COMPOSE_FILES[@]}" --profile platform-api --profile ai-workspace \
  up -d --wait --wait-timeout 300 platform-api ai-workspace

log "Waiting for platform-api to report healthy"
for _ in $(seq 1 60); do
  if curl_cp -o /dev/null -w '%{http_code}' "$PLATFORM_API/health" | grep -q '^200$'; then
    break
  fi
  sleep 2
done
if ! curl_cp -o /dev/null -w '%{http_code}' "$PLATFORM_API/health" | grep -q '^200$'; then
  echo "platform-api did not become healthy at $PLATFORM_API" >&2
  exit 1
fi

# --- Bootstrap: mint a registration token ----------------------------------
log "Authenticating as $ADMIN_USER"
TOKEN="$(curl_cp -X POST "$PLATFORM_API/api/portal/v0.9/auth/login" \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode "username=$ADMIN_USER" \
  --data-urlencode "password=$ADMIN_PASSWORD" \
  | sed -n 's/.*"token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
if [ -z "$TOKEN" ]; then
  echo "login failed against $PLATFORM_API — check credentials" >&2
  exit 1
fi

# functionalityType MUST be "ai" — that is what makes this gateway selectable as
# a deployment target for LLM providers/proxies in AI Workspace. A "regular"
# gateway (what tests/integration-e2e creates for REST APIs) is created happily
# but never appears in the AI Workspace deploy dialog.
log "Creating gateway '$GATEWAY_ID' (reused if it already exists)"
CREATE_RESPONSE="$(curl_cp -X POST "$PLATFORM_API/api/v0.9/gateways" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"id\":\"$GATEWAY_ID\",\"displayName\":\"$GATEWAY_ID\",\"endpoints\":[\"$GATEWAY_ENDPOINT\"],\"functionalityType\":\"ai\"}")"

# The gateway is addressed by its handle, which is exactly the id we just sent —
# there is no separate generated uuid to scrape. So a re-run that collides (409)
# needs no lookup: confirm the gateway is there and carry on.
if ! curl_cp -o /dev/null -w '%{http_code}' "$PLATFORM_API/api/v0.9/gateways/$GATEWAY_ID" \
  -H "Authorization: Bearer $TOKEN" | grep -q '^200$'; then
  echo "could not create or find gateway '$GATEWAY_ID'. Response: $CREATE_RESPONSE" >&2
  exit 1
fi

# A gateway caps how many tokens may be active at once (2 today), and every run
# of this script mints one. Without revoking the previous ones the third run
# would fail with GATEWAY_TOKEN_LIMIT_REACHED — so clear them first. Revoking
# the token the currently-running controller holds is fine: it is replaced by
# the fresh one below, and phase 2 recreates the container with it.
log "Revoking previously issued tokens for '$GATEWAY_ID'"
# Read ids line by line rather than word-splitting an unquoted variable: that
# splits per-shell (zsh does not split by default) and a single mangled id
# silently turns into one bogus DELETE that revokes nothing.
curl_cp "$PLATFORM_API/api/v0.9/gateways/$GATEWAY_ID/tokens" \
  -H "Authorization: Bearer $TOKEN" \
  | tr '{' '\n' | sed -n 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
  | while IFS= read -r token_id; do
      [ -n "$token_id" ] || continue
      curl_cp -o /dev/null -X DELETE \
        "$PLATFORM_API/api/v0.9/gateways/$GATEWAY_ID/tokens/$token_id" \
        -H "Authorization: Bearer $TOKEN" || true
    done

log "Minting a registration token for gateway '$GATEWAY_ID'"
REGISTRATION_TOKEN="$(curl_cp -X POST "$PLATFORM_API/api/v0.9/gateways/$GATEWAY_ID/tokens" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{}' \
  | sed -n 's/.*"token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
if [ -z "$REGISTRATION_TOKEN" ]; then
  echo "failed to mint a registration token for gateway '$GATEWAY_ID'" >&2
  exit 1
fi

# --- Phase 2: data plane ---------------------------------------------------
log "Starting the gateway data plane with the minted token"
GATEWAY_REGISTRATION_TOKEN="$REGISTRATION_TOKEN" \
  docker compose "${COMPOSE_FILES[@]}" up -d --wait --wait-timeout 300 \
  gateway-controller gateway-runtime sample-backend

log "Waiting for the gateway to register with the control plane"
for _ in $(seq 1 60); do
  STATE="$(curl_cp "$PLATFORM_API/api/v0.9/gateways/$GATEWAY_ID" \
    -H "Authorization: Bearer $TOKEN" || true)"
  # The control plane flips isActive once the controller completes its
  # registration handshake; it is false from creation until then.
  if printf '%s' "$STATE" | grep -q '"isActive"[[:space:]]*:[[:space:]]*true'; then
    log "Gateway '$GATEWAY_ID' is connected"
    exit 0
  fi
  sleep 2
done

echo "gateway '$GATEWAY_ID' did not report isActive=true within 120s" >&2
echo "check: docker compose ${COMPOSE_FILES[*]} logs gateway-controller" >&2
exit 1
