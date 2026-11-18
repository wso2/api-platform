#!/usr/bin/env bash
# Deploys the provider and both App LLM proxies through the gateway's management API, and stores one
# proxy key per application in Secrets Manager. Run it while ./scripts/port-forward.sh is running.
# Usage: ./scripts/deploy-artifacts.sh
set -euo pipefail
# shellcheck source=scripts/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

API="http://localhost:9090/api/management/v1"
NETRC_FILE="$(mktemp)"
trap 'rm -f "${NETRC_FILE}"' EXIT
chmod 600 "${NETRC_FILE}"
aws secretsmanager get-secret-value --secret-id "${ADMIN_SECRET}" --query SecretString --output text \
  | {
      read -r pw
      # curl reads the password from a .netrc file, which splits fields on whitespace and treats
      # quotes and backslashes specially. install-gateway.sh generates a password without them.
      case "${pw}" in
        *[[:space:]\"\\]*)
          echo "The admin password in ${ADMIN_SECRET} contains spaces, quotes, or backslashes, which curl can't read from a .netrc file." >&2
          exit 1 ;;
      esac
      printf 'machine localhost login admin password %s\n' "${pw}"
    } > "${NETRC_FILE}"

api() {
  curl --fail-with-body -sS --netrc-file "${NETRC_FILE}" "$@"
}

# Wait for the management API, which port-forward.sh makes available on this machine.
ready=0
for _ in $(seq 1 30); do
  if api -o /dev/null "${API}/llm-providers" 2>/dev/null; then ready=1; break; fi
  sleep 2
done
[ "${ready}" = "1" ] || { echo "Can't reach the management API on port 9090. Run ./scripts/port-forward.sh in a second terminal, then rerun." >&2; exit 1; }

# Creates an API key and prints its value. A key with the same name, left by an earlier run that
# failed before storing it, is deleted first, because key names must be unique.
create_key() {
  local kind="$1" id="$2" name="$3"
  api -o /dev/null -X DELETE "${API}/${kind}/${id}/api-keys/${name}" 2>/dev/null || true
  api -X POST "${API}/${kind}/${id}/api-keys" \
    -H "Content-Type: application/json" -d "{\"name\":\"${name}\"}" | jq -er '.apiKey.apiKey // empty'
}

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
    loopback_key="$(create_key llm-providers bedrock-provider "${app}-loopback")"
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
    app_key="$(create_key llm-proxies "${app}" "${app}-app")"
    printf '%s' "${app_key}" | aws secretsmanager create-secret --name "${SECRET_PREFIX}/${app}" \
      --description "Gateway API key for ${app}" \
      --secret-string file:///dev/stdin >/dev/null
    unset app_key
    echo "Stored the ${app} key in Secrets Manager as ${SECRET_PREFIX}/${app}"
  fi
done

echo "Done. Test with: ./scripts/test-guardrails.sh"
