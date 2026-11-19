#!/usr/bin/env bash
# Builds gateway images with the policy versions in gateway/policy-versions.env, using the ap CLI and
# the build.yaml that ships with the gateway, and pushes them to Amazon ECR for linux/amd64 nodes.
# Usage: ./scripts/prepare-gateway.sh
set -euo pipefail
# shellcheck source=scripts/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

GATEWAY_VERSION="1.2.0"
GATEWAY_ZIP_URL="https://github.com/wso2/api-platform/releases/download/ai-gateway/v${GATEWAY_VERSION}/wso2apip-ai-gateway-${GATEWAY_VERSION}.zip"
GATEWAY_ZIP_SHA256="6fe3581223b2522d41a79fdebdfb1f04232f5c35dc8a2daeff54fdc80fce5308"
BUILD_FILE="${GATEWAY_DIR}/build.yaml"
VERSIONS_FILE="${ROOT_DIR}/gateway/policy-versions.env"

# Download the distribution and verify its checksum. The build needs only its build.yaml.
if [ ! -d "${GATEWAY_DIR}" ]; then
  curl -fsSL -o "${ROOT_DIR}/gateway.zip" "${GATEWAY_ZIP_URL}"
  echo "${GATEWAY_ZIP_SHA256}  ${ROOT_DIR}/gateway.zip" | shasum -a 256 -c -
  (cd "${ROOT_DIR}" && unzip -q gateway.zip && rm gateway.zip \
    && mv "wso2apip-ai-gateway-${GATEWAY_VERSION}" "$(basename "${GATEWAY_DIR}")")
fi
[ -f "${BUILD_FILE}" ] || { echo "No build.yaml in ${GATEWAY_DIR}" >&2; exit 1; }

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

# The ap CLI, pinned and checksum-verified. It goes into .tools/ in this repository, not onto your PATH,
# and cleanup.sh removes it. The image names below are read from this version's output.
AP_VERSION="0.9.1"
AP="${ROOT_DIR}/.tools/ap-v${AP_VERSION}/ap"
if [ ! -x "${AP}" ]; then
  case "$(uname -s)" in
    Darwin) os=darwin ;;
    Linux) os=linux ;;
    *) echo "Unsupported operating system: $(uname -s). Use macOS, Linux, or WSL." >&2; exit 1 ;;
  esac
  case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 ;;
    arm64 | aarch64) arch=arm64 ;;
    *) echo "Unsupported CPU architecture: $(uname -m)." >&2; exit 1 ;;
  esac
  case "${os}-${arch}" in
    darwin-amd64) ap_sha256="38d31dfb78517fdbad6919ea1676be13e20ccfef99002eb8547102a98cc9a704" ;;
    darwin-arm64) ap_sha256="e4995d25672c22ba8112c47ae92b8195ca201b33b06f4d6a3e58807a530134da" ;;
    linux-amd64) ap_sha256="c5831a1114984d4b34d2f8f435a74bbd94094f480a5d4fe0d7348f71a16b313d" ;;
    linux-arm64) ap_sha256="38a003f2cf484fd66a3e9d4b8a8e562b39088e62c8360e33931341a65b1b4592" ;;
  esac
  ap_name="ap-${os}-${arch}-v${AP_VERSION}"
  echo "Downloading the ap CLI v${AP_VERSION} (${os}-${arch}) into .tools/ ..."
  curl -fsSL -o "${TMP_DIR}/ap.zip" \
    "https://github.com/wso2/api-platform/releases/download/ap/v${AP_VERSION}/${ap_name}.zip"
  echo "${ap_sha256}  ${TMP_DIR}/ap.zip" | shasum -a 256 -c -
  unzip -q "${TMP_DIR}/ap.zip" -d "${TMP_DIR}"
  mkdir -p "$(dirname "${AP}")"
  install -m 0755 "${TMP_DIR}/${ap_name}/ap" "${AP}"
fi

# 1. Set the policy versions in build.yaml.
while IFS='=' read -r policy version; do
  case "${policy}" in '' | \#*) continue ;; esac
  [ -n "${version}" ] || continue
  sed -i.bak -E "s#(gateway-controllers/policies/${policy})@[^[:space:]\"]+#\1@${version}#" "${BUILD_FILE}"
  rm -f "${BUILD_FILE}.bak"
  grep -q "policies/${policy}@${version}" "${BUILD_FILE}" || {
    echo "Couldn't set ${policy}@${version} in ${BUILD_FILE}" >&2
    exit 1
  }
done < "${VERSIONS_FILE}"

# 2. Sign in to Amazon ECR, then build and push the controller and runtime images.
aws ecr get-login-password | docker login --username AWS --password-stdin "$(tf_output ecr_registry)"
LOG="${TMP_DIR}/build.log"
# ap passes --platform only to the final image build, so the default platform makes its builder container
# compile the policy engine for linux/amd64 too, on any host.
(cd "${GATEWAY_DIR}" && DOCKER_DEFAULT_PLATFORM=linux/amd64 "${AP}" gateway image build --name bedrock-guardrails \
  --repository "$(tf_output image_repository)" --platform linux/amd64 --push) | tee "${LOG}"
CONTROLLER_IMAGE="$(grep -o '[^[:space:]]*-gateway-controller:[^[:space:]]*' "${LOG}" | tail -1)"
RUNTIME_IMAGE="$(grep -o '[^[:space:]]*-gateway-runtime:[^[:space:]]*' "${LOG}" | tail -1)"
[ -n "${CONTROLLER_IMAGE}" ] && [ -n "${RUNTIME_IMAGE}" ] || {
  echo "Couldn't find the built image names in the ap output." >&2
  exit 1
}

# 3. Record the images for install-gateway.sh, and show what was built.
printf 'CONTROLLER_IMAGE=%s\nRUNTIME_IMAGE=%s\n' "${CONTROLLER_IMAGE}" "${RUNTIME_IMAGE}" > "${GATEWAY_DIR}/.sample-images"
echo "Pushed ${CONTROLLER_IMAGE} and ${RUNTIME_IMAGE}."
echo "Policy versions in this build:"
grep -o 'gateway-controllers/policies/[^[:space:]"]*' "${BUILD_FILE}" | sed 's#gateway-controllers/policies/#  #'
