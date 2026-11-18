#!/usr/bin/env bash
# Shared settings for the scripts. Reads the Terraform outputs into environment variables.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

tf_output() {
  terraform -chdir="${ROOT_DIR}/terraform" output -raw "$1"
}

AWS_REGION="$(tf_output region)"
BEDROCK_MODEL_ID="$(tf_output model_id)"
BASELINE_GUARDRAIL_ID="$(tf_output baseline_guardrail_id)"
BASELINE_GUARDRAIL_VERSION="$(tf_output baseline_guardrail_version)"
SUPPORT_GUARDRAIL_ID="$(tf_output support_guardrail_id)"
SUPPORT_GUARDRAIL_VERSION="$(tf_output support_guardrail_version)"
export AWS_REGION BEDROCK_MODEL_ID \
  BASELINE_GUARDRAIL_ID BASELINE_GUARDRAIL_VERSION SUPPORT_GUARDRAIL_ID SUPPORT_GUARDRAIL_VERSION
export AWS_DEFAULT_REGION="${AWS_REGION}"

export SECRET_PREFIX="wso2-ai-gateway/apps"
export ADMIN_SECRET="wso2-ai-gateway/admin-password"
export APPS="support-assistant employee-assistant"

# Only these variables are substituted, so other ${...} text stays intact.
# shellcheck disable=SC2016 # Single quotes are intentional: envsubst expands these names.
export SUBST_VARS='${AWS_REGION} ${BASELINE_GUARDRAIL_ID} ${BASELINE_GUARDRAIL_VERSION} ${SUPPORT_GUARDRAIL_ID} ${SUPPORT_GUARDRAIL_VERSION}'

# The downloaded gateway distribution, used only to build images, and the Helm release settings.
export GATEWAY_DIR="${ROOT_DIR}/wso2-ai-gateway"
export RELEASE="wso2-ai-gateway"
NAMESPACE="$(tf_output namespace)"
CLUSTER_NAME="$(tf_output cluster_name)"
export NAMESPACE CLUSTER_NAME
