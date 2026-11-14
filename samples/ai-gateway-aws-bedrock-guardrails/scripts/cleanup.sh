#!/usr/bin/env bash
# Stops the gateway, removes its data, and deletes the application key secrets.
# Run this script, and then run terraform destroy.
# Usage: ./scripts/cleanup.sh <gateway-folder>
set -euo pipefail
# shellcheck source=scripts/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
GATEWAY_DIR="$(gateway_dir "${1:-}")"

(cd "${GATEWAY_DIR}" && gateway_compose down --volumes && rm -f docker-compose.override.yaml)
if [ -f "${GATEWAY_DIR}/.sample-images" ]; then
  # shellcheck disable=SC1091 # Written by build-gateway-image.sh.
  source "${GATEWAY_DIR}/.sample-images"
  docker image rm "${CONTROLLER_IMAGE}" "${RUNTIME_IMAGE}" >/dev/null 2>&1 || true
  rm -f "${GATEWAY_DIR}/.sample-images"
fi
rm -rf "${ROOT_DIR}/.tools"   # The ap CLI that build-gateway-image.sh downloaded

# Deletes only the secrets that the scripts created: the admin password and one key per application.
secrets=("${ADMIN_SECRET}")
for app in ${APPS}; do secrets+=("${SECRET_PREFIX}/${app}"); done
for secret in "${secrets[@]}"; do
  if aws secretsmanager describe-secret --secret-id "${secret}" >/dev/null 2>&1; then
    aws secretsmanager delete-secret --secret-id "${secret}" --force-delete-without-recovery >/dev/null
    echo "Deleted secret ${secret}"
  fi
done
echo "Done. Now run terraform destroy."
