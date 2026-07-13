#!/usr/bin/env bash
# -----------------------------------------------------------------------------
# Load the locally-built WSO2 API Platform gateway images into the KinD
# conformance cluster.
#
# Run AFTER building the images (README step 2) and AFTER kind/setup-kind.sh has
# created the cluster, and BEFORE install-wso2-gateway.sh — the operator chart
# deploys everything with imagePullPolicy=IfNotPresent, so the images must
# already be present on the cluster node.
#
# The tags are derived from the SAME source of truth the build uses, so the
# loaded images always match exactly what install-wso2-gateway.sh deploys:
#   - gateway-controller / gateway-runtime : gateway/VERSION
#   - gateway-operator                     : VERSION in the operator Makefile
# Each image is verified to exist in the local Docker daemon before loading, so
# a forgotten/failed build fails fast with a clear message instead of a later,
# confusing ImagePull error inside the cluster.
#
# Overridable via env:
#   CLUSTER_NAME       KinD cluster name         (default: wso2-conformance)
#   REGISTRY           image registry/namespace  (default: ghcr.io/wso2/api-platform)
#   GW_VERSION         controller/runtime tag    (default: contents of gateway/VERSION)
#   OPERATOR_VERSION   operator image tag        (default: VERSION from the operator Makefile)
# -----------------------------------------------------------------------------
set -euo pipefail

# This script lives at kubernetes/conformance/, so the repo root is two levels up.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

CLUSTER_NAME="${CLUSTER_NAME:-wso2-conformance}"
REGISTRY="${REGISTRY:-ghcr.io/wso2/api-platform}"

# Derive tags from the build's own sources so they match install-wso2-gateway.sh.
GW_VERSION="${GW_VERSION:-$(cat "${REPO_ROOT}/gateway/VERSION")}"
OPERATOR_VERSION="${OPERATOR_VERSION:-$(sed -nE 's/^VERSION[[:space:]]*\?=[[:space:]]*([^[:space:]]+).*/\1/p' \
  "${REPO_ROOT}/kubernetes/gateway-operator/Makefile" | head -1)}"

if [ -z "${GW_VERSION}" ] || [ -z "${OPERATOR_VERSION}" ]; then
  echo "error: could not determine image versions (GW_VERSION='${GW_VERSION}', OPERATOR_VERSION='${OPERATOR_VERSION}')." >&2
  exit 1
fi

IMAGES=(
  "${REGISTRY}/gateway-controller:${GW_VERSION}"
  "${REGISTRY}/gateway-runtime:${GW_VERSION}"
  "${REGISTRY}/gateway-operator:${OPERATOR_VERSION}"
)

# The cluster must already exist — kind load targets its node(s).
if ! kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
  echo "error: KinD cluster '${CLUSTER_NAME}' not found. Run ./kind/setup-kind.sh first." >&2
  exit 1
fi

# Verify every image was actually built before loading any of them.
missing=0
for img in "${IMAGES[@]}"; do
  if ! docker image inspect "${img}" >/dev/null 2>&1; then
    echo "  MISSING: ${img}" >&2
    missing=1
  fi
done
if [ "${missing}" -ne 0 ]; then
  echo "error: one or more images are not present in the local Docker daemon." >&2
  echo "       Build them first (README step 2):" >&2
  echo "         (cd ${REPO_ROOT}/gateway && make build)" >&2
  echo "         (cd ${REPO_ROOT}/kubernetes/gateway-operator && make docker-build)" >&2
  echo "       Or override the tags via GW_VERSION / OPERATOR_VERSION." >&2
  exit 1
fi

echo ">> Loading ${#IMAGES[@]} images into KinD cluster '${CLUSTER_NAME}'"
for img in "${IMAGES[@]}"; do
  echo "   -> ${img}"
  kind load docker-image "${img}" --name "${CLUSTER_NAME}"
done

echo ">> Done. All images loaded into '${CLUSTER_NAME}'."
