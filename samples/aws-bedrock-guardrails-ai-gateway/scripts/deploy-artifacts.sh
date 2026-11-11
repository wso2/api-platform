#!/usr/bin/env bash
# Deploys the provider and both App LLM proxies through the
# standalone gateway's management API, and stores one proxy key per application in Secrets Manager.
# Run it after ./scripts/start-gateway.sh.
# Usage: ./scripts/deploy-artifacts.sh
set -euo pipefail
# shellcheck source=scripts/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

GATEWAY_DIR="${ROOT_DIR}/wso2apip-ai-gateway"
API="http://localhost:9090/api/management/v1"
NETRC_FILE="$(mktemp)"
trap 'rm -f "${NETRC_FILE}"' EXIT
chmod 600 "${NETRC_FILE}"
aws secretsmanager get-secret-value --secret-id "${ADMIN_SECRET}" --query SecretString --output text \
  | { read -r pw; printf 'machine localhost login admin password %s\n' "${pw}"; } > "${NETRC_FILE}"

api() {
  curl --fail-with-body -sS --netrc-file "${NETRC_FILE}" "$@"
}

# Wait for the controller's admin API.
for _ in $(seq 1 30); do
  curl -fs http://localhost:9094/api/admin/v1/health >/dev/null && break
  sleep 2
done

# Creates the artifact, or replaces it if it already exists.
apply_artifact() {
  local kind="$1" name="$2" file="$3"
  local method="POST" url="${API}/${kind}"
  if api -o /dev/null "${API}/${kind}/${name}" 2>/dev/null; then
    method="PUT"
    url="${API}/${kind}/${name}"
  fi
  envsubst "${SUBST_VARS}" < "${file}" \
    | api -o /dev/null -X "${method}" "${url}" -H "Content-Type: application/yaml" --data-binary @-
  echo "Deployed ${kind}/${name}"
}

apply_artifact llm-providers bedrock-provider "${ROOT_DIR}/gateway/bedrock-provider.yaml"

for app in ${APPS}; do
  # Provider key that the proxy sends on the internal route, stored as an encrypted gateway secret.
  if ! api -o /dev/null "${API}/secrets/${app}-loopback" 2>/dev/null; then
    loopback_key="$(api -X POST "${API}/llm-providers/bedrock-provider/api-keys" \
      -H "Content-Type: application/json" -d "{\"name\":\"${app}-loopback\"}" | jq -r '.apiKey.apiKey')"
    api -o /dev/null -X POST "${API}/secrets" -H "Content-Type: application/yaml" --data-binary @- <<SECRET
apiVersion: gateway.api-platform.wso2.com/v1
kind: Secret
metadata:
  name: ${app}-loopback
spec:
  displayName: ${app} provider key
  value: "${loopback_key}"
SECRET
    unset loopback_key
  fi

  apply_artifact llm-proxies "${app}" "${ROOT_DIR}/gateway/${app}-proxy.yaml"

  # Application key, piped straight into Secrets Manager so it never appears in the terminal.
  if ! aws secretsmanager describe-secret --secret-id "${SECRET_PREFIX}/${app}" >/dev/null 2>&1; then
    api -X POST "${API}/llm-proxies/${app}/api-keys" \
      -H "Content-Type: application/json" -d "{\"name\":\"${app}-app\"}" \
      | jq -r '.apiKey.apiKey' \
      | aws secretsmanager create-secret --name "${SECRET_PREFIX}/${app}" \
          --description "Gateway API key for ${app}" \
          --secret-string file:///dev/stdin >/dev/null
    echo "Stored the ${app} key in Secrets Manager as ${SECRET_PREFIX}/${app}"
  fi
done

echo "Done. Test with: ./scripts/test-guardrails.sh ${GATEWAY_DIR}"
