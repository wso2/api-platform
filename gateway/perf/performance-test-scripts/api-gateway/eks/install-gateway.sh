#!/bin/bash -e
# Install / upgrade API Platform Gateway on EKS via official OCI Helm chart.
#
# Prerequisite: EKS cluster created (AWS Console) and kubeconfig connected.
# Usage:
#   cp env.eks.example env.eks && edit && source env.eks
#   ./connect-kubeconfig.sh
#   ./install-gateway.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=eks-common.sh
source "${SCRIPT_DIR}/eks-common.sh"
eks_load_env
eks_require_cmd kubectl helm aws
eks_kubecontext

echo "==> Namespace: ${EKS_NAMESPACE}"
kubectl create namespace "${EKS_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
eks_ensure_ghcr_pull_secret
eks_ensure_controller_encryption_key_secret
eks_validate_storage_class
eks_fix_controller_pvc_storage_class

eks_write_generated_values
gen_values="$(eks_generated_values_path)"
base_values="${EKS_DIR}/values.perf.yaml"

helm_args=(-n "${EKS_NAMESPACE}")
eks_helm_append_chart_ref helm_args
if [[ -f "$base_values" ]]; then
    helm_args+=(-f "$base_values")
fi
helm_args+=(-f "$gen_values")

echo "==> Helm chart: ${GATEWAY_HELM_CHART}"
[[ -n "${GATEWAY_HELM_CHART_VERSION:-}" ]] && echo "    version: ${GATEWAY_HELM_CHART_VERSION}"
echo "==> Runtime replicas: ${GATEWAY_RUNTIME_REPLICAS}"
echo "==> Runtime CPU/mem: ${GATEWAY_RUNTIME_CPU_LIMIT} / ${GATEWAY_RUNTIME_MEM_LIMIT}"
echo "==> Config: ${PERF_CONFIG_TOML}"
echo "==> Images: $(eks_helm_images_label)"

if helm status "${EKS_RELEASE_NAME}" -n "${EKS_NAMESPACE}" >/dev/null 2>&1; then
    eks_reset_runtime_deployment_for_helm
    helm upgrade "${helm_args[@]}"
else
    helm install "${helm_args[@]}"
fi

echo "==> Enforce replica counts (controller=1, runtime=${GATEWAY_RUNTIME_REPLICAS:-1})"
kubectl scale deployment "${EKS_RELEASE_NAME}-controller" -n "${EKS_NAMESPACE}" --replicas=1
kubectl scale deployment "${EKS_RELEASE_NAME}-gateway-runtime" -n "${EKS_NAMESPACE}" \
    --replicas="${GATEWAY_RUNTIME_REPLICAS:-1}"

echo "==> Patch runtime args"
if [[ -n "${GATEWAY_RUNTIME_EXTRA_ARGS:-}" ]]; then
    eks_patch_runtime_args
else
    echo "    (skipped — GATEWAY_RUNTIME_EXTRA_ARGS unset; required for official chart images)"
fi

echo "==> Waiting for pods..."
kubectl wait -n "${EKS_NAMESPACE}" \
    --for=condition=ready pod \
    -l "app.kubernetes.io/instance=${EKS_RELEASE_NAME}" \
    --timeout=300s || true

kubectl get pods -n "${EKS_NAMESPACE}" -o wide
kubectl get svc -n "${EKS_NAMESPACE}"

host="$(eks_wait_runtime_lb || true)"
if [[ -n "${host:-}" ]]; then
    echo ""
    echo "Gateway runtime LB: http://${host}:8080"
    echo "Save for JMeter: export GATEWAY_HOST=${host}"
fi

cat <<EOF

Deploy APIs:  ./deploy-apis-eks-minimal.sh

EOF
