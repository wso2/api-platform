#!/usr/bin/env bash
# -----------------------------------------------------------------------------
# Shared version/registry resolution for the conformance scripts. Source it, don't
# run it — keeping this in one place is what guarantees the images that get built,
# loaded, deployed and reported on all carry the same tag.
#
# Sets (each overridable via env):
#   REPO_ROOT, REGISTRY, GW_VERSION, OPERATOR_VERSION
# -----------------------------------------------------------------------------

# This file lives at kubernetes/conformance/, so the repo root is two levels up.
REPO_ROOT="${REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"

REGISTRY="${REGISTRY:-ghcr.io/wso2/api-platform}"
GW_VERSION="${GW_VERSION:-$(cat "${REPO_ROOT}/gateway/VERSION")}"
OPERATOR_VERSION="${OPERATOR_VERSION:-$(sed -nE 's/^VERSION[[:space:]]*\?=[[:space:]]*([^[:space:]]+).*/\1/p' \
  "${REPO_ROOT}/kubernetes/gateway-operator/Makefile" | head -1)}"

if [ -z "${GW_VERSION}" ] || [ -z "${OPERATOR_VERSION}" ]; then
  echo "error: could not determine image versions (GW_VERSION='${GW_VERSION}', OPERATOR_VERSION='${OPERATOR_VERSION}')." >&2
  exit 1
fi
