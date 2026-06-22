#!/usr/bin/env bash
# -----------------------------------------------------------------------------
# macOS / Colima helper: bring up a Colima VM with a host-reachable IP, create the
# KinD + MetalLB conformance cluster, and route the kind bridge subnet through the
# VM so the conformance suite (running on the host) can dial the gateway's
# LoadBalancer IP — i.e. Linux-equivalent reachability on a Mac.
#
# This wraps setup-kind.sh; on Linux you don't need any of this.
#
# Usage:
#   ./setup-colima.sh            # start Colima + cluster + route + reachability gate
#   ./setup-colima.sh --delete   # delete route, cluster, and stop Colima
#
# Overridable via env (passed through to setup-kind.sh too):
#   COLIMA_CPU (6)  COLIMA_MEMORY (10)
#   CLUSTER_NAME (wso2-conformance)  CLUSTER_NODE_VERSION  METALLB_VERSION
# -----------------------------------------------------------------------------
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

COLIMA_CPU="${COLIMA_CPU:-6}"
COLIMA_MEMORY="${COLIMA_MEMORY:-10}"
COLIMA_DISK="${COLIMA_DISK:-60}"
CLUSTER_NAME="${CLUSTER_NAME:-wso2-conformance}"

need() { command -v "$1" >/dev/null 2>&1 || { echo "!! missing required tool: $1" >&2; exit 1; }; }

vm_ip() { colima ls --json 2>/dev/null | jq -r 'select(.address != null and .address != "") | .address' | head -n1; }

kind_subnet() {
  docker network inspect kind 2>/dev/null \
    | jq -r '.[].IPAM.Config[].Subnet | select(contains(":") | not)' | head -n1
}

# --------------------------------------------------------------------------- #
# Teardown
# --------------------------------------------------------------------------- #
if [[ "${1:-}" == "--delete" ]]; then
  SUBNET="$(kind_subnet || true)"
  if [[ -n "${SUBNET}" ]] && netstat -rn -f inet | grep -q "${SUBNET%%/*}"; then
    echo ">> Removing host route for ${SUBNET}"
    sudo route -n delete -net "${SUBNET}" >/dev/null 2>&1 || true
  fi
  echo ">> Deleting KinD cluster '${CLUSTER_NAME}'"
  CLUSTER_NAME="${CLUSTER_NAME}" "${SCRIPT_DIR}/setup-kind.sh" --delete || true
  echo ">> Stopping Colima"
  colima stop || true
  echo ">> Done. (Run 'colima delete' to remove the VM entirely.)"
  exit 0
fi

# --------------------------------------------------------------------------- #
# 0. Dependencies
# --------------------------------------------------------------------------- #
# need colima; need jq; need kind; need kubectl
# if ! brew list socket_vmnet >/dev/null 2>&1; then
#   echo "!! socket_vmnet is required for 'colima start --network-address'." >&2
#   echo "   Install it with: brew install socket_vmnet" >&2
#   exit 1
# fi


# ---- 1. Colima VM with a reachable IP --------------------------------------
if colima status >/dev/null 2>&1; then
  echo ">> Colima already running, reusing it"
  if [[ -z "$(vm_ip)" ]]; then
    echo "!! Colima is running WITHOUT a network address. Restart it so the host can" >&2
    echo "   reach the cluster:  colima stop && colima start --network-address ..." >&2
    exit 1
  fi
else
  echo ">> Starting Colima (${COLIMA_CPU} CPU / ${COLIMA_MEMORY} GB, --network-address)"
  colima start \
    --cpu "${COLIMA_CPU}" --memory "${COLIMA_MEMORY}" \
    --network-address
fi
docker context use colima >/dev/null
need docker

VMIP="$(vm_ip)"
[[ -n "${VMIP}" ]] || { echo "!! could not determine Colima VM IP from 'colima ls --json'" >&2; exit 1; }
echo ">> Colima VM IP: ${VMIP}"

# --- 2. KinD + MetalLB cluster (delegated to the shared script) ----------------
# Skip setup-kind.sh's own reachability check: on macOS the host can't reach the LB
# IP until we add the route + VM forwarding below. Otherwise the in-kind check would
# fail prematurely.
SKIP_REACHABILITY_CHECK=1 CLUSTER_NAME="${CLUSTER_NAME}" "${SCRIPT_DIR}/setup-kind.sh"


# ---- 3. Host route to the kind bridge subnet via the VM ----------------------
SUBNET="$(kind_subnet)"
[[ -n "${SUBNET}" ]] || { echo "!! could not determine 'kind' docker network subnet" >&2; exit 1; }
echo ">> Adding host route: ${SUBNET} -> ${VMIP} (sudo)"
sudo route -n delete -net "${SUBNET}" >/dev/null 2>&1 || true
sudo route -n add -net "${SUBNET}" "${VMIP}"

# ----- 4. Enable forwarding through the VM (proactively, not as a fallback) ---
# Routing host -> LB IP sends packets to the VM, which must forward them onto its
# kind bridge. This essentially always requires ip_forward + a FORWARD ACCEPT for
# the bridge subnet, so apply it up front. Use -C (check) to stay idempotent.
echo ">> Enabling packet forwarding for ${SUBNET} inside the Colima VM"
colima ssh -- sudo sysctl -w net.ipv4.ip_forward=1 >/dev/null
colima ssh -- sudo iptables -C FORWARD -d "${SUBNET}" -j ACCEPT 2>/dev/null \
  || colima ssh -- sudo iptables -I FORWARD -d "${SUBNET}" -j ACCEPT
colima ssh -- sudo iptables -C FORWARD -s "${SUBNET}" -j ACCEPT 2>/dev/null \
  || colima ssh -- sudo iptables -I FORWARD -s "${SUBNET}" -j ACCEPT


# ---- 5. Reachability gate (host -> MetalLB LoadBalancer IP) ----------------
# Create the smoke workload ONCE, wait for the backend pod to be Ready (a fresh VM
# has to pull the image first; an LB IP is assigned long before the pod is up), then
# retry the probe so ARP/endpoints have time to settle.
echo ">> Checking host -> LoadBalancer reachability"
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

if [[ "${CODE}" == "200" ]]; then
  echo ">> host -> LoadBalancer = 200 ✓  The conformance suite can reach the gateway from the host."
else
  echo "!! host -> LoadBalancer failing (got ${CODE})." >&2
  echo "   The cluster is up, but the host can't route to ${SUBNET} via ${VMIP}. Check:" >&2
  echo "     - route present:   netstat -rn | grep ${SUBNET%%/*}" >&2
  echo "     - VM reachable:    ping -c1 ${VMIP}" >&2
  echo "     - forwarding on:   colima ssh -- sudo sysctl net.ipv4.ip_forward" >&2
  echo "   You can still run the suite in-cluster." >&2
  exit 1
fi

echo ">> Done. Next: ./install-wso2-gateway.sh && ./run-conformance.sh"
echo "   (route is not persistent — re-run this script after a reboot or 'colima restart')"
