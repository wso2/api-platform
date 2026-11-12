#!/usr/bin/env bash
# Shared settings for the scripts. Reads the Terraform outputs into environment variables.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

tf_output() {
  terraform -chdir="${ROOT_DIR}/terraform" output -raw "$1"
}

AWS_REGION="$(tf_output region)"
BEDROCK_MODEL_ID="$(tf_output model_id)"
GATEWAY_ROLE_ARN="$(tf_output gateway_role_arn)"
BASELINE_GUARDRAIL_ID="$(tf_output baseline_guardrail_id)"
BASELINE_GUARDRAIL_VERSION="$(tf_output baseline_guardrail_version)"
SUPPORT_GUARDRAIL_ID="$(tf_output support_guardrail_id)"
SUPPORT_GUARDRAIL_VERSION="$(tf_output support_guardrail_version)"
export AWS_REGION BEDROCK_MODEL_ID GATEWAY_ROLE_ARN \
  BASELINE_GUARDRAIL_ID BASELINE_GUARDRAIL_VERSION SUPPORT_GUARDRAIL_ID SUPPORT_GUARDRAIL_VERSION
export AWS_DEFAULT_REGION="${AWS_REGION}"

export SECRET_PREFIX="wso2-ai-gateway/apps"
export ADMIN_SECRET="wso2-ai-gateway/admin-password"
export APPS="support-assistant employee-assistant"

# deploy-artifacts.sh passes the admin password to curl in a .netrc file, which splits fields on
# whitespace and treats quotes and backslashes specially. Returns 1 for a password with those characters.
valid_admin_password() {
  case "$1" in
    *[[:space:]\"\\]*) return 1 ;;
  esac
}

# Only these variables are substituted, so other ${...} text stays intact.
# shellcheck disable=SC2016 # Single quotes are intentional: envsubst expands these names.
export SUBST_VARS='${AWS_REGION} ${GATEWAY_ROLE_ARN} ${BASELINE_GUARDRAIL_ID} ${BASELINE_GUARDRAIL_VERSION} ${SUPPORT_GUARDRAIL_ID} ${SUPPORT_GUARDRAIL_VERSION}'

# The gateway folder that start-gateway.sh created (wso2apip-ai-gateway). Pass it as the first argument.
gateway_dir() {
  local dir="${1:?Pass the gateway folder, wso2apip-ai-gateway.}"
  [ -f "${dir}/api-platform.env" ] || { echo "No api-platform.env in ${dir}. Run ./scripts/start-gateway.sh first." >&2; return 1; }
  (cd "${dir}" && pwd)
}

# The guardrail policy version built into the gateway images. v1.3.0 adds applicableStatusCodes,
# so a response check skips error responses (such as another guardrail's HTTP 422).
export GUARDRAIL_POLICY_VERSION="v1.3.0"

# Runs docker compose with the gateway's env files: .env (the Compose project name that setup pins)
# and api-platform.env. Docker Compose also merges docker-compose.override.yaml automatically.
gateway_compose() {
  local args=()
  [ -f .env ] && args+=(--env-file .env)
  [ -f api-platform.env ] && args+=(--env-file api-platform.env)
  docker compose "${args[@]}" "$@"
}
