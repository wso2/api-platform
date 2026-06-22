#!/usr/bin/env bash
# -----------------------------------------------------------------------------
# Install everything the WSO2 API Platform gateway needs on the conformance
# cluster: cert-manager (required by the gateway Helm chart), the gateway operator,
# and the GatewayClass the suite targets. The operator chart installs the bundled
# standard-channel Gateway API CRDs (v1.5.1) itself via gatewayApi.installStandardCRDs.
#
# Run AFTER kind/setup-kind.sh. Assumes kubectl context points at the conformance
# cluster (setup-kind.sh switches to it).
#
# Overridable via env:
#   OPERATOR_CHART   path/ref to the operator Helm chart
#   OPERATOR_NS      namespace for the operator (default: gateway-system)
# -----------------------------------------------------------------------------
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

OPERATOR_CHART="${OPERATOR_CHART:-${REPO_ROOT}/kubernetes/helm/operator-helm-chart}"
OPERATOR_NS="${OPERATOR_NS:-gateway-system}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# --- 1. cert-manager (the gateway chart creates Certificate/Issuer) ---------
echo ">> Installing cert-manager"
helm repo add jetstack https://charts.jetstack.io --force-update
helm repo update
helm upgrade --install cert-manager jetstack/cert-manager \
  --namespace cert-manager --create-namespace \
  --set crds.enabled=true \
  --wait
kubectl wait --namespace cert-manager \
  --for=condition=available deployment --all --timeout=180s

# --- 2. Gateway operator (installs the bundled Gateway API CRDs too) --------
echo ">> Installing the gateway operator from ${OPERATOR_CHART}"
helm upgrade --install gateway-operator "${OPERATOR_CHART}" \
  --namespace "${OPERATOR_NS}" --create-namespace \
  --set image.repository=ghcr.io/wso2/api-platform/gateway-operator \
  --set image.tag=0.8.1-SNAPSHOT \
  --set image.pullPolicy=IfNotPresent \
  --set gateway.values.gateway.controller.image.repository=ghcr.io/wso2/api-platform/gateway-controller \
  --set gateway.values.gateway.controller.image.tag=1.2.0-SNAPSHOT \
  --set gateway.values.gateway.controller.image.pullPolicy=IfNotPresent \
  --set gateway.values.gateway.gatewayRuntime.image.repository=ghcr.io/wso2/api-platform/gateway-runtime \
  --set gateway.values.gateway.gatewayRuntime.image.tag=1.2.0-SNAPSHOT \
  --set gateway.values.gateway.gatewayRuntime.image.pullPolicy=IfNotPresent \
  --set gateway.values.gateway.config.controller.storage.type=memory \
  --set gateway.values.gateway.controller.storage.type=sqlite \
  --set gateway.values.gateway.controller.persistence.enabled=false \
  --set gateway.values.gateway.gatewayRuntime.deployment.securityContext.runAsUser=0 \
  --set reconciliation.maxConcurrentReconciles=15 \
  --wait
kubectl rollout status --namespace "${OPERATOR_NS}" deployment --timeout=240s

# --- 3. GatewayClass --------------------------------------------------------
echo ">> Creating the GatewayClass"
kubectl apply -f "${SCRIPT_DIR}/manifests/gatewayclass.yaml"
echo ">> Waiting for GatewayClass to be Accepted"
kubectl wait --for=condition=Accepted gatewayclass/wso2-api-platform --timeout=120s || \
  echo "   (GatewayClass not yet Accepted — check operator logs in ns ${OPERATOR_NS})"

echo ">> Install complete. Run ./run-conformance.sh next."
