#!/usr/bin/env bash
# Uninstalls the gateway and cert-manager, deletes the application key and admin password secrets, and
# removes the downloaded tools. Safe to rerun. Run terraform destroy afterwards.
# Usage: ./scripts/cleanup.sh
set -euo pipefail
# shellcheck source=scripts/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

if aws eks update-kubeconfig --name "${CLUSTER_NAME}" --region "${AWS_REGION}" >/dev/null 2>&1; then
  helm uninstall "${RELEASE}" --namespace "${NAMESPACE}" --wait 2>/dev/null || true
  helm uninstall cert-manager --namespace cert-manager --wait 2>/dev/null || true
  kubectl delete namespace "${NAMESPACE}" cert-manager --ignore-not-found --wait=true
else
  echo "No cluster to clean up. Continuing."
fi

rm -rf "${GATEWAY_DIR}" "${ROOT_DIR}/.tools"   # The distribution and the ap CLI that prepare-gateway.sh downloaded

# Deletes only the secrets that the scripts created: the admin password and one key per application.
# The database password secret belongs to Amazon RDS, and terraform destroy removes it.
secrets=("${ADMIN_SECRET}")
for app in ${APPS}; do secrets+=("${SECRET_PREFIX}/${app}"); done
for secret in "${secrets[@]}"; do
  if aws secretsmanager describe-secret --secret-id "${secret}" >/dev/null 2>&1; then
    aws secretsmanager delete-secret --secret-id "${secret}" --force-delete-without-recovery >/dev/null
    echo "Deleted secret ${secret}"
  fi
done
echo "Done. Now run terraform destroy."
