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
# Usage:
#   ./install-wso2-gateway.sh            # install (default)
#   ./install-wso2-gateway.sh cleanup    # tear everything back down (reuse the cluster)
#
# `cleanup` removes what this script installs PLUS whatever the conformance suite
# leaves behind (Gateways/HTTPRoutes/ReferenceGrants + the gateway-conformance-*
# namespaces), so you can re-install and re-run without recreating the kind cluster.
# Gateway API CRDs are left in place by default (a re-install re-applies them);
# pass PURGE_CRDS=1 to also delete them.
#
# Overridable via env:
#   OPERATOR_CHART   path/ref to the operator Helm chart
#   OPERATOR_NS      namespace for the operator (default: gateway-system)
#   PURGE_CRDS       cleanup only: also delete the Gateway API CRDs (default: unset)
# -----------------------------------------------------------------------------
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

OPERATOR_CHART="${OPERATOR_CHART:-${REPO_ROOT}/kubernetes/helm/operator-helm-chart}"
OPERATOR_NS="${OPERATOR_NS:-gateway-system}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Namespaces the Gateway API conformance suite creates for its fixtures.
CONFORMANCE_NAMESPACES=(
  gateway-conformance-infra
  gateway-conformance-app-backend
  gateway-conformance-web-backend
)

# Gateway-API finalizers the operator adds; a lingering one blocks namespace deletion.
FINALIZER_KINDS=(httproutes.gateway.networking.k8s.io gateways.gateway.networking.k8s.io)

# strip_finalizers <kind>: remove finalizers from any remaining objects of <kind> so a
# stuck deletion can complete. Fallback for when the operator is already gone/slow and
# cannot process its own finalizers.
strip_finalizers() {
  local kind="$1"
  kubectl get "${kind}" -A \
    -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.metadata.name}{"\n"}{end}' 2>/dev/null \
  | while read -r ns name; do
      [ -z "${name:-}" ] && continue
      kubectl patch "${kind}" "${name}" -n "${ns}" --type=merge \
        -p '{"metadata":{"finalizers":[]}}' >/dev/null 2>&1 || true
    done
}

cleanup() {
  echo ">> Cleaning up WSO2 gateway + conformance resources (cluster is preserved)"

  # 1. Delete Gateway-API resources FIRST, while the operator is still running so it
  #    can process its finalizers. ReferenceGrants have no finalizer; do them too.
  echo ">> Deleting HTTPRoutes / Gateways / ReferenceGrants"
  kubectl delete httproutes.gateway.networking.k8s.io --all -A --ignore-not-found --timeout=60s || true
  kubectl delete referencegrants.gateway.networking.k8s.io --all -A --ignore-not-found --timeout=60s || true
  kubectl delete gateways.gateway.networking.k8s.io --all -A --ignore-not-found --timeout=60s || true

  # 2. Fallback: strip any finalizers left behind so nothing hangs Terminating.
  for kind in "${FINALIZER_KINDS[@]}"; do
    strip_finalizers "${kind}"
  done

  # 3. Conformance fixture namespaces.
  echo ">> Deleting conformance namespaces"
  kubectl delete namespace "${CONFORMANCE_NAMESPACES[@]}" --ignore-not-found --timeout=120s || true

  # 4. GatewayClass.
  echo ">> Deleting the GatewayClass"
  kubectl delete -f "${SCRIPT_DIR}/manifests/gatewayclass.yaml" --ignore-not-found || true

  # 5. Uninstall the operator (only now that its finalized resources are gone), then cert-manager.
  echo ">> Uninstalling the gateway operator"
  helm uninstall gateway-operator --namespace "${OPERATOR_NS}" --wait 2>/dev/null || true
  kubectl delete namespace "${OPERATOR_NS}" --ignore-not-found --timeout=120s || true

  echo ">> Uninstalling cert-manager"
  helm uninstall cert-manager --namespace cert-manager --wait 2>/dev/null || true
  kubectl delete namespace cert-manager --ignore-not-found --timeout=120s || true

  # 6. Optionally remove the Gateway API CRDs (default: keep them for the next install).
  if [ -n "${PURGE_CRDS:-}" ]; then
    echo ">> Deleting Gateway API CRDs (PURGE_CRDS set)"
    kubectl get crd -o name 2>/dev/null | grep 'gateway.networking.k8s.io$' \
      | xargs -r kubectl delete --ignore-not-found || true
  fi

  echo ">> Cleanup complete. Re-run ./install-wso2-gateway.sh to reinstall."
  exit 0
}

case "${1:-install}" in
  cleanup|uninstall|--cleanup|clean)
    cleanup
    ;;
  install|"")
    ;;
  *)
    echo "Usage: $0 [install|cleanup]" >&2
    exit 2
    ;;
esac

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
  --set reconciliation.interval=150 \
  --wait
kubectl rollout status --namespace "${OPERATOR_NS}" deployment --timeout=240s

# --- 3. GatewayClass --------------------------------------------------------
echo ">> Creating the GatewayClass"
kubectl apply -f "${SCRIPT_DIR}/manifests/gatewayclass.yaml"
echo ">> Waiting for GatewayClass to be Accepted"
kubectl wait --for=condition=Accepted gatewayclass/wso2-api-platform --timeout=120s || \
  echo "   (GatewayClass not yet Accepted — check operator logs in ns ${OPERATOR_NS})"

echo ">> Install complete. Run ./run-conformance.sh next."
