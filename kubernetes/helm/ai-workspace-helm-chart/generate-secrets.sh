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
# generate-secrets.sh — secret provisioning for the ai-workspace umbrella.
#
# Model: "setup generates, startup only checks." Creates the Kubernetes Secrets
# the subcharts reference and writes values-secrets.yaml with the secret
# REFERENCES only (never secret values). Component enablement is controlled
# separately by your -f my_values.yaml — this script never toggles components.
#
# CUMULATIVE + IDEMPOTENT: a re-run never rotates or drops anything. Existing
# Secrets are left untouched (the RS256 public key is re-read from the existing
# Platform API Secret so the portal keeps verifying its tokens), and
# values-secrets.yaml is rebuilt to reference every component Secret that exists.
#
# The Platform API signs its RS256 JWTs with a private key; the API Portal
# verifies them with the matching PUBLIC key — the same keypair, no shared HMAC
# secret. The file-mode admin credential is GENERATED here (random password,
# bcrypt hash stored in the Secret) — there is no admin/admin default.
#
# Usage:
#   [flags] [inputs] ./generate-secrets.sh <namespace> [release-name]
#     <namespace>     Namespace to create the Secrets in (required).
#     [release-name]  Helm release name; names the Secrets (default: ai-workspace).
#
# Component flags:
#   API_PORTAL=true   Also provision the API Portal Secret.
#   (The shared Platform API Secret is always ensured. The AI Workspace UI Secret
#    is provisioned only when APIP_AIW_AUTH_OIDC_CLIENT_SECRET is set — basic mode needs none.)
#
# Optional inputs (only wired if set). Each is named EXACTLY like the key it
# becomes inside the Secret, the env var the container receives, and the
# {{ env "..." }} token in config.toml that reads it — one name for the whole
# chain, so there is nothing to translate between this script and the config.
#
#   Platform API ([platform_api.*], APIP_CP_*)
#     APIP_CP_ADMIN_USERNAME             File-mode admin username (default: admin).
#     APIP_CP_DATABASE_PASSWORD          Database password (postgres/sqlserver).
#     APIP_CP_WEBHOOK_SECRET             HMAC secret for the webhook receiver.
#
#   AI Workspace BFF ([ai_workspace.*], APIP_AIW_*)
#     APIP_AIW_AUTH_OIDC_CLIENT_SECRET   OIDC client secret (OIDC mode only).
#
#   API Portal ([api_portal.*], APIP_AP_*) — needs API_PORTAL=true
#     APIP_AP_DATABASE_PASSWORD          Database password (postgres/mssql).
#     APIP_AP_AUTH_IDP_CLIENT_SECRET     OIDC/IDP client secret (idp mode only).
# --------------------------------------------------------------------
set -euo pipefail

NAMESPACE="${1:-}"
RELEASE="${2:-ai-workspace}"
API_PORTAL="${API_PORTAL:-false}"
APIP_CP_ADMIN_USERNAME="${APIP_CP_ADMIN_USERNAME:-admin}"

if [[ -z "$NAMESPACE" ]]; then
  echo "Usage: [API_PORTAL=true] $0 <namespace> [release-name]" >&2
  exit 1
fi
command -v kubectl  >/dev/null 2>&1 || { echo "error: kubectl not found in PATH" >&2; exit 1; }
command -v openssl  >/dev/null 2>&1 || { echo "error: openssl not found in PATH" >&2; exit 1; }

PA_SECRET="${RELEASE}-platform-api-secrets"
UI_SECRET="${RELEASE}-ai-workspace-ui-secrets"
AP_SECRET="${RELEASE}-api-portal-ui-secrets"
OUT_FILE="values-secrets.yaml"

TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT

kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || {
  echo "==> Creating namespace $NAMESPACE"; kubectl create namespace "$NAMESPACE";
}

gen_key()       { openssl rand -hex 32; }
secret_exists() { kubectl -n "$NAMESPACE" get secret "$1" >/dev/null 2>&1; }
# NB: a missing map key renders as the literal "<no value>" under go-template,
# which a bare `grep -q .` would wrongly treat as present — filter it out.
secret_has_key(){ local v; v="$(kubectl -n "$NAMESPACE" get secret "$1" -o "go-template={{ index .data \"$2\" }}" 2>/dev/null)"; [[ -n "$v" && "$v" != "<no value>" ]]; }
# bcrypt via htpasswd, else the httpd docker image (matches the distribution setup.sh).
bcrypt_hash() {
  local pw="$1"
  if command -v htpasswd >/dev/null 2>&1; then
    printf '%s' "$pw" | htpasswd -niB -C 10 "" | cut -d: -f2 | tr -d '\r\n'
  elif command -v docker >/dev/null 2>&1; then
    printf '%s' "$pw" | docker run --rm -i httpd:2.4-alpine htpasswd -niB -C 10 "" | cut -d: -f2 | tr -d '\r\n'
  else
    echo "error: need htpasswd (apache2-utils/httpd-tools) or docker to bcrypt-hash the admin password" >&2
    exit 1
  fi
}

# --- Platform API (shared): encryption key, RS256 keypair, generated admin creds ---
if secret_exists "$PA_SECRET"; then
  echo "==> $PA_SECRET already exists — leaving it untouched"
  kubectl -n "$NAMESPACE" get secret "$PA_SECRET" -o "go-template={{ index .data \"jwt_public.pem\" }}" | base64 --decode > "$TMP/jwt_public.pem"
else
  echo "==> Creating $PA_SECRET"
  openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$TMP/jwt_private.pem" 2>/dev/null
  openssl rsa -in "$TMP/jwt_private.pem" -pubout -out "$TMP/jwt_public.pem" 2>/dev/null
  ADMIN_PASSWORD="$(openssl rand -base64 24 | tr -dc 'A-Za-z0-9' | cut -c1-20)"
  APIP_CP_ADMIN_PASSWORD_HASH="$(bcrypt_hash "$ADMIN_PASSWORD")"
  args=(
    --from-literal=APIP_CP_ENCRYPTION_KEY="$(gen_key)"
    --from-literal=APIP_CP_ADMIN_USERNAME="$APIP_CP_ADMIN_USERNAME"
    --from-literal=APIP_CP_ADMIN_PASSWORD_HASH="$APIP_CP_ADMIN_PASSWORD_HASH"
    --from-file=jwt_public.pem="$TMP/jwt_public.pem"
    --from-file=jwt_private.pem="$TMP/jwt_private.pem"
  )
  [[ -n "${APIP_CP_DATABASE_PASSWORD:-}" ]] && args+=( --from-literal=APIP_CP_DATABASE_PASSWORD="$APIP_CP_DATABASE_PASSWORD" )
  [[ -n "${APIP_CP_WEBHOOK_SECRET:-}"   ]] && args+=( --from-literal=APIP_CP_WEBHOOK_SECRET="$APIP_CP_WEBHOOK_SECRET" )
  kubectl -n "$NAMESPACE" create secret generic "$PA_SECRET" "${args[@]}"
  echo "    Generated file-mode admin credential (shown once — store it now):"
  echo "      username: ${APIP_CP_ADMIN_USERNAME}"
  echo "      password: ${ADMIN_PASSWORD}"
fi

# --- AI Workspace UI: OIDC client secret (OIDC mode only) ---
if [[ -n "${APIP_AIW_AUTH_OIDC_CLIENT_SECRET:-}" ]]; then
  if secret_exists "$UI_SECRET"; then
    echo "==> $UI_SECRET already exists — leaving it untouched"
  else
    echo "==> Creating $UI_SECRET"
    kubectl -n "$NAMESPACE" create secret generic "$UI_SECRET" \
      --from-literal=APIP_AIW_AUTH_OIDC_CLIENT_SECRET="$APIP_AIW_AUTH_OIDC_CLIENT_SECRET"
  fi
else
  echo "==> APIP_AIW_AUTH_OIDC_CLIENT_SECRET not set — skipping UI Secret (only needed for OIDC mode)"
fi

# --- API Portal: encryption key + session secret + PA public key (+optional) ---
if [[ "$API_PORTAL" == "true" ]]; then
  if secret_exists "$AP_SECRET"; then
    echo "==> $AP_SECRET already exists — leaving it untouched"
  else
    echo "==> Creating $AP_SECRET"
    args=(
      --from-literal=APIP_AP_SECURITY_ENCRYPTION_KEY="$(gen_key)"
      --from-literal=APIP_AP_SECURITY_SESSION_SECRET="$(gen_key)"
      --from-file=jwt_public.pem="$TMP/jwt_public.pem"
    )
    [[ -n "${APIP_AP_AUTH_IDP_CLIENT_SECRET:-}" ]] && args+=( --from-literal=APIP_AP_AUTH_IDP_CLIENT_SECRET="$APIP_AP_AUTH_IDP_CLIENT_SECRET" )
    [[ -n "${APIP_AP_DATABASE_PASSWORD:-}"  ]] && args+=( --from-literal=APIP_AP_DATABASE_PASSWORD="$APIP_AP_DATABASE_PASSWORD" )
    kubectl -n "$NAMESPACE" create secret generic "$AP_SECRET" "${args[@]}"
  fi
fi

# --- Write values-secrets.yaml CUMULATIVELY: reference every Secret that exists now ---
echo "==> Writing $OUT_FILE (references for all provisioned components)"
{
  echo "# Generated by generate-secrets.sh — secret REFERENCES only (no secret values)."
  echo "# Cumulative: re-runs keep every previously-provisioned reference. Safe to commit."
  echo "# Install/upgrade with:"
  echo "#   helm upgrade --install $RELEASE ./ai-workspace-helm-chart -n $NAMESPACE \\"
  echo "#     -f $OUT_FILE -f my_values.yaml"
  echo "platform-api:"
  echo "  secrets:"
  echo "    existingSecret: $PA_SECRET"
  if secret_exists "$UI_SECRET"; then
    echo "ai-workspace-ui:"
    echo "  secrets:"
    echo "    existingSecret: $UI_SECRET"
  fi
  if secret_exists "$AP_SECRET"; then
    echo "api-portal-ui:"
    echo "  secrets:"
    echo "    existingSecret: $AP_SECRET"
    echo "    hasPublicKey: true"
    secret_has_key "$AP_SECRET" APIP_AP_AUTH_IDP_CLIENT_SECRET      && echo "    hasIdpClientSecret: true"
  fi
} > "$OUT_FILE"

echo
echo "Done. Next:"
echo "  helm dependency update ./ai-workspace-helm-chart   # first time / after editing deps"
echo "  helm upgrade --install $RELEASE ./ai-workspace-helm-chart -n $NAMESPACE \\"
echo "    -f $OUT_FILE -f my_values.yaml"
