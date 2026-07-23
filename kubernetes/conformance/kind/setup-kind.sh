#!/usr/bin/env bash
# -----------------------------------------------------------------------------
# Create a KinD cluster and install MetalLB for Gateway API conformance, then
# validate that LoadBalancer traffic routing actually works.
#
# Usage:
#   ./setup-kind.sh                # create cluster "wso2-conformance" + MetalLB + validate
#   CLUSTER_NAME=foo ./setup-kind.sh
#   ./setup-kind.sh --delete       # tear the cluster down
#
# Env:
#   CLUSTER_NAME            cluster name (default: wso2-conformance)
#   CLUSTER_NODE_VERSION   kindest/node image (pinned by digest)
#   METALLB_VERSION        MetalLB release (default: v0.14.8)
#   SKIP_REACHABILITY_CHECK=1   skip the host->LoadBalancer validation. On a bare
#                               Linux host you normally leave this unset.
# -----------------------------------------------------------------------------
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

CLUSTER_NAME="${CLUSTER_NAME:-wso2-conformance}"
# Pin the Kubernetes version by digest for reproducibility. Override CLUSTER_NODE_VERSION
# to certify against a different Kubernetes release.
CLUSTER_NODE_VERSION="${CLUSTER_NODE_VERSION:-v1.34.0@sha256:7416a61b42b1662ca6ca89f02028ac133a309a2a30ba309614e8ec94d976dc5a}"
METALLB_VERSION="${METALLB_VERSION:-v0.14.8}"

need() { command -v "$1" >/dev/null 2>&1 || { echo "!! missing required tool: $1" >&2; exit 1; }; }

if [[ "${1:-}" == "--delete" ]]; then
  need kind
  echo ">> Deleting KinD cluster '${CLUSTER_NAME}'"
  kind delete cluster --name "${CLUSTER_NAME}"
  exit 0
fi

# --- 0. Dependencies --------------------------------------------------------
for t in kind kubectl docker jq curl; do need "$t"; done

# --- 1. Cluster -------------------------------------------------------------
if kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
  echo ">> KinD cluster '${CLUSTER_NAME}' already exists, reusing it"
else
  echo ">> Creating KinD cluster '${CLUSTER_NAME}' (node ${CLUSTER_NODE_VERSION})"
  kind create cluster \
    --name "${CLUSTER_NAME}" \
    --image "kindest/node:${CLUSTER_NODE_VERSION}" \
    --config "${SCRIPT_DIR}/cluster.yaml"
fi
kubectl config use-context "kind-${CLUSTER_NAME}"

# --- 2. MetalLB -------------------------------------------------------------
echo ">> Installing MetalLB ${METALLB_VERSION}"
kubectl apply -f "https://raw.githubusercontent.com/metallb/metallb/${METALLB_VERSION}/config/manifests/metallb-native.yaml"
echo ">> Waiting for MetalLB controller to be ready"
kubectl wait --namespace metallb-system \
  --for=condition=available deployment/controller \
  --timeout=180s
# The speaker is a DaemonSet; wait for its pods to be Ready before advertising.
kubectl rollout status --namespace metallb-system daemonset/speaker --timeout=180s
# The controller serves a validating webhook for IPAddressPool/L2Advertisement; wait
# for its endpoints so the apply below doesn't race the webhook ("connection refused").
echo ">> Waiting for the MetalLB webhook endpoints"
for _ in $(seq 1 30); do
  if [[ -n "$(kubectl get endpoints metallb-webhook-service -n metallb-system \
        -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null)" ]]; then
    break
  fi
  sleep 2
done

# --- 3. Address pool derived from the "kind" docker network -----------------
# Take the IPv4 subnet KinD's docker bridge uses and reserve a high band of it
# (x.x.255.200 - x.x.255.250) that won't collide with node container IPs.
SUBNET="$(docker network inspect kind \
  | jq -r '.[].IPAM.Config[].Subnet | select(contains(":") | not)' \
  | head -n1 | cut -d '.' -f1,2)"
if [[ -z "${SUBNET}" ]]; then
  echo "!! Could not determine the 'kind' docker network subnet" >&2
  exit 1
fi
ADDR_RANGE="${SUBNET}.255.200-${SUBNET}.255.250"
echo ">> MetalLB address pool: ${ADDR_RANGE}"

apply_metallb_pool() {
  kubectl apply -f - <<EOF
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: conformance-pool
  namespace: metallb-system
spec:
  addresses:
    - ${ADDR_RANGE}
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: conformance-l2
  namespace: metallb-system
spec:
  ipAddressPools:
    - conformance-pool
EOF
}
# Retry: even after the endpoints exist the webhook can briefly refuse connections.
for attempt in $(seq 1 15); do
  if apply_metallb_pool; then
    break
  fi
  if [[ "${attempt}" -eq 15 ]]; then
    echo "!! Failed to apply the MetalLB address pool after 15 attempts" >&2
    exit 1
  fi
  echo "   (MetalLB webhook not ready yet, retrying ${attempt}/15)"
  sleep 4
done

echo ">> Cluster '${CLUSTER_NAME}' is ready with MetalLB serving ${ADDR_RANGE}."

# --- 4. Validate LoadBalancer routing (host -> MetalLB LB IP) ---------------
# Confirms MetalLB actually assigns a LB IP and that traffic routes to a backend.
# Skipped when SKIP_REACHABILITY_CHECK=1 (e.g. setup-colima.sh runs this after it
# adds its host route, so the in-line check here would fire prematurely).
if [[ "${SKIP_REACHABILITY_CHECK:-}" == "1" ]]; then
  echo ">> Skipping LoadBalancer reachability check (SKIP_REACHABILITY_CHECK=1)."
else
  echo ">> Validating LoadBalancer traffic routing"
  kubectl delete svc lb-smoke --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete deployment lb-smoke --ignore-not-found >/dev/null 2>&1 || true
  kubectl create deployment lb-smoke \
    --image=registry.k8s.io/e2e-test-images/agnhost:2.39 -- /agnhost netexec --http-port=8080 >/dev/null
  kubectl expose deployment lb-smoke --type=LoadBalancer --port=80 --target-port=8080 >/dev/null

  echo ">> Waiting for the smoke backend pod to be Ready"
  kubectl rollout status deployment/lb-smoke --timeout=180s

  IP=""
  for _ in $(seq 1 30); do
    IP="$(kubectl get svc lb-smoke -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)"
    [[ -n "${IP}" ]] && break
    sleep 2
  done
  echo ">> LoadBalancer IP: ${IP:-<none>}"

  CODE="000"
  if [[ -n "${IP}" ]]; then
    for _ in $(seq 1 15); do
      CODE="$(curl -s -m 5 -o /dev/null -w '%{http_code}' "http://${IP}/" || true)"
      [[ "${CODE}" == "200" ]] && break
      sleep 3
    done
  fi

  kubectl delete svc lb-smoke --ignore-not-found >/dev/null 2>&1 || true
  kubectl delete deployment lb-smoke --ignore-not-found >/dev/null 2>&1 || true

  if [[ "${IP}" == "" ]]; then
    echo "!! MetalLB did not assign a LoadBalancer IP — check the speaker pods and the address pool." >&2
    exit 1
  fi
  if [[ "${CODE}" == "200" ]]; then
    echo ">> host -> LoadBalancer = 200 ✓  LoadBalancer routing works; the suite can reach the gateway."
  else
    echo "!! host -> LoadBalancer failing (got ${CODE})." >&2
    echo "   MetalLB assigned ${IP} but the host could not reach it." >&2
    echo "   On Linux the kind bridge is host-routable, so check MetalLB speaker logs / firewall." >&2
    echo "   On macOS (Docker/Rancher/Colima) the bridge is NOT host-routable — use ./setup-colima.sh," >&2
    echo "   or run the conformance suite from inside the cluster." >&2
    exit 1
  fi
fi

echo ">> Done."
echo "   Next: install the gateway operator and the GatewayClass (./install-wso2-gateway.sh; see README.md)."
