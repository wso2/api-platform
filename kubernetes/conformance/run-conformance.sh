#!/usr/bin/env bash
# -----------------------------------------------------------------------------
# Run the Gateway API conformance suite against the WSO2 API Platform gateway
# and write the ConformanceReport.
#
# Run AFTER install-wso2-gateway.sh, with kubectl pointed at the conformance
# cluster. The suite drives the cluster via the current kube context: it creates
# Gateways of the GatewayClass below, applies HTTPRoutes + backends, then sends
# real requests to the MetalLB-assigned Gateway address.
#
# The suite is consumed as a Go module dependency (kubernetes/conformance/runner),
# NOT from a clone of the kubernetes-sigs/gateway-api repo — its test manifests are
# embedded in the module, so `go test` resolves everything from the module cache.
#
# Overridable via env:
#   GATEWAY_CLASS        GatewayClass name              (default: wso2-api-platform)
#   PROFILE              conformance profile            (default: GATEWAY-HTTP)
#   SUPPORTED_FEATURES   comma-separated features       (default: Gateway,HTTPRoute)
#   IMPL_VERSION         implementation version string  (default: 1.1.0)
#   REPORT_OUT           report output path
# -----------------------------------------------------------------------------
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNNER_DIR="${SCRIPT_DIR}/runner"

GATEWAY_CLASS="${GATEWAY_CLASS:-wso2-api-platform}"
PROFILE="${PROFILE:-GATEWAY-HTTP}"
# Keep this on ONE line: a line break (even with a trailing backslash) leaves the
# continuation's leading whitespace inside the value, corrupting the feature name
# right after each break so that feature silently fails to register.
SUPPORTED_FEATURES="${SUPPORTED_FEATURES:-Gateway,HTTPRoute,HTTPRouteSchemeRedirect,HTTPRoutePortRedirect,HTTPRoute303RedirectStatusCode,HTTPRoute307RedirectStatusCode,HTTPRoute308RedirectStatusCode}"
IMPL_VERSION="${IMPL_VERSION:-1.2.0-milestone}"
REPORT_OUT="${REPORT_OUT:-${SCRIPT_DIR}/wso2-api-platform-${IMPL_VERSION}-report.yaml}"

echo ">> Running conformance: profile=${PROFILE} gateway-class=${GATEWAY_CLASS}"
echo ">> Report will be written to ${REPORT_OUT}"

cd "${RUNNER_DIR}"
go test -tags conformance -v -run TestConformance -timeout 20m -args \
  -gateway-class="${GATEWAY_CLASS}" \
  -supported-features="${SUPPORTED_FEATURES}" \
  -conformance-profiles="${PROFILE}" \
  -cleanup-base-resources=true \
  -organization=WSO2 \
  -project=api-platform-gateway \
  -url=https://github.com/wso2/api-platform \
  -version="${IMPL_VERSION}" \
  -contact=https://github.com/wso2/api-platform/issues \
  -report-output="${REPORT_OUT}" \
  -allow-crds-mismatch=true

echo ">> Conformance run finished. Report: ${REPORT_OUT}"
