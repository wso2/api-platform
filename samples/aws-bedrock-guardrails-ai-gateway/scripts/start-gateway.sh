#!/usr/bin/env bash
# Downloads the open-source WSO2 AI Gateway, sets it up in
# standalone mode, and starts it with your temporary AWS credentials (through run-gateway.sh).
# Then deploy the provider and proxies with ./scripts/deploy-artifacts.sh.
# Usage: ./scripts/start-gateway.sh
set -euo pipefail
# shellcheck source=scripts/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

GATEWAY_VERSION="1.2.0"
GATEWAY_ZIP_URL="https://github.com/wso2/api-platform/releases/download/ai-gateway/v${GATEWAY_VERSION}/wso2apip-ai-gateway-${GATEWAY_VERSION}.zip"
GATEWAY_ZIP_SHA256="6fe3581223b2522d41a79fdebdfb1f04232f5c35dc8a2daeff54fdc80fce5308"
GATEWAY_DIR="${ROOT_DIR}/wso2apip-ai-gateway"

# 1. Download the distribution and verify its checksum.
if [ ! -d "${GATEWAY_DIR}" ]; then
  curl -fsSL -o "${ROOT_DIR}/gateway.zip" "${GATEWAY_ZIP_URL}"
  echo "${GATEWAY_ZIP_SHA256}  ${ROOT_DIR}/gateway.zip" | shasum -a 256 -c -
  # The zip extracts to wso2apip-ai-gateway-<version>; rename it to a version-independent folder.
  (cd "${ROOT_DIR}" && unzip -q gateway.zip && rm gateway.zip \
    && mv "wso2apip-ai-gateway-${GATEWAY_VERSION}" wso2apip-ai-gateway)
fi

# Asks for the management API admin password, without echoing it, and prints it to stdout for the caller.
# Set ADMIN_PASSWORD to skip the prompt, for example in CI.
read_admin_password() {
  local password confirm
  if [ -n "${ADMIN_PASSWORD:-}" ]; then
    printf '%s' "${ADMIN_PASSWORD}"
    return
  fi
  [ -t 0 ] || { echo "Set ADMIN_PASSWORD, or run this script in a terminal so that it can ask for the password." >&2; exit 1; }
  while :; do
    read -r -s -p "Choose an admin password for the gateway management API: " password \
      || { echo >&2; echo "No password entered." >&2; exit 1; }
    echo >&2
    # An empty password would make the gateway's setup generate its own, which wouldn't match Secrets Manager.
    if [ -z "${password}" ]; then
      echo "Enter a password." >&2
      continue
    fi
    read -r -s -p "Enter the password again: " confirm || { echo >&2; echo "No password entered." >&2; exit 1; }
    echo >&2
    [ "${password}" = "${confirm}" ] && break
    echo "The passwords don't match. Try again." >&2
  done
  printf '%s' "${password}"
}

# 2. First run only: store your admin password in Secrets Manager, then run the distribution's setup.
#    --force regenerates the listener certificate and AES key, because the ones in the zip are public.
if [ ! -f "${GATEWAY_DIR}/api-platform.env" ]; then
  if ! aws secretsmanager describe-secret --secret-id "${ADMIN_SECRET}" >/dev/null 2>&1; then
    admin_password="$(read_admin_password)"
    printf '%s' "${admin_password}" | aws secretsmanager create-secret --name "${ADMIN_SECRET}" \
      --description "WSO2 AI Gateway management API admin password" \
      --secret-string file:///dev/stdin >/dev/null
    unset admin_password
    echo "Stored the admin password in Secrets Manager as ${ADMIN_SECRET}."
  fi
  (cd "${GATEWAY_DIR}" && ADMIN_USERNAME=admin ADMIN_PASSWORD="$(aws secretsmanager get-secret-value --secret-id "${ADMIN_SECRET}" \
    --query SecretString --output text)" ./scripts/setup.sh --force)
fi

# 3. Build images with the newer guardrail policy, then add the allowlist and credentials override and
#    start the gateway.
exec "${ROOT_DIR}/scripts/run-gateway.sh" "${GATEWAY_DIR}"
