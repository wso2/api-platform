#!/bin/bash -e
# Jenkins build script: api-gateway EKS performance test.
#
# === Required Jenkins build parameters (env vars) ===
#   BUILD_USER_EMAIL                 e.g. xyz@wso2.com
#   AWS_REGION                       e.g. us-east-1
#   GATEWAY_NODE_INSTANCE_TYPE       e.g. m5.2xlarge  (EKS gateway pods)
#   BACKEND_NODE_INSTANCE_TYPE       e.g. m5.xlarge   (EKS in-cluster backend)
#   JMETER_CLIENT_EC2_INSTANCE_TYPE  e.g. c5.2xlarge
#   JMETER_SERVER_EC2_INSTANCE_TYPE  e.g. c5.2xlarge
#   GATEWAY_HELM_CHART_VERSION       e.g. 1.2.0-rc
#   RUN_PERF_OPTS                    load/scenario flags only, e.g.:
#       "-u 1000 -b 1 -s 0 -d 900 -w 180 -i api_api_plain_get"
#       Multiple: -u 100 -u 500 -u 1000  -i api_api_plain_get -i api_api_header_get
#       Exclude:  -e api_api_jwt_get
#     Configurable: -u users, -b message bytes, -s backend sleep ms, -d duration,
#                   -w warmup, -i include scenario, -e exclude scenario.
#     Infra heaps/engines (-n/-m/-j/-k/-l/-r) are set by this script, not RUN_PERF_OPTS.
#
# === Optional Jenkins env (script sources) ===
#   PERF_SCRIPTS   repo@branch:subdir
#       default: https://github.com/wso2/api-platform.git@main:gateway/perf/performance-test-scripts
#   PERF_COMMON_REPO / PERF_COMMON_BRANCH
#       default: performance-common fork used for jtl-splitter
#   ROUTER_CONCURRENCY / GOMAXPROCS
#       default: 4 / 4  (runtime pod concurrency; keep ≤ GATEWAY_RUNTIME_CPU_LIMIT)
#   GOGC / GOMEMLIMIT
#       default: 400 / 1500MiB
#   PUBLISH_RESULTS_PR
#       default: 0 (skip). Set to 1 to open a PR appending summary.csv into RESULTS_PR_README
#   GH_TOKEN
#       required only when PUBLISH_RESULTS_PR=1 (PAT with repo + PR access)
#   RESULTS_PR_REPO / RESULTS_PR_BASE / RESULTS_PR_README
#       defaults: wso2/api-platform @ main : gateway/perf/README.md
#
# === Pre-placed on Jenkins slave (one-time setup) ===
#   ~/keys/apim-perf-test3.pem        EC2 SSH key (pem file)
#   ~/apache-jmeter-5.6.3.tgz         JMeter tarball
#   eksctl, kubectl, helm, aws CLI, envsubst, python3, jq, mvn  on PATH
#   AWS key pair "apim-perf-test3" registered in the target AWS region
#   Jenkins slave IAM role: EKS full + EC2 full + SSM read + ELB full
#   (Optional) helm registry login ghcr.io — only if using private chart images
#
# === Topology ===
#   EKS cluster with 2 node groups: gateway-ng + backend-ng
#   In-cluster Netty mock backend (perf-mock-backend ClusterIP service)
#   Internal AWS NLB for gateway runtime (port 8080, same-VPC access)
#   1 JMeter client EC2 + 2 JMeter server EC2s in EKS VPC (public subnet)

set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# Save Jenkins job workspace (job subdir) before we override WORKSPACE.
JENKINS_JOB_WORKSPACE="${WORKSPACE:-}"
# Always compute from script location — do not use Jenkins $WORKSPACE (it points to job subdir).
WORKSPACE="$(cd "${SCRIPT_DIR}/.." && pwd)"

# ─── Validate required vars ───────────────────────────────────────────────────
for v in BUILD_USER_EMAIL AWS_REGION \
          GATEWAY_NODE_INSTANCE_TYPE BACKEND_NODE_INSTANCE_TYPE \
          JMETER_CLIENT_EC2_INSTANCE_TYPE JMETER_SERVER_EC2_INSTANCE_TYPE \
          GATEWAY_HELM_CHART_VERSION RUN_PERF_OPTS; do
    [[ -n "${!v:-}" ]] || { echo "ERROR: env var ${v} is not set." >&2; exit 1; }
done

# Infra flags below are appended by Step 13. run-scenario.sh treats -n/-m/-j/-k/-l/-r as
# repeatable matrix dimensions, so leaving one in RUN_PERF_OPTS silently doubles every
# scenario (e.g. "-r 1" here plus the injected "-r 1" = two identical passes).
sanitize_run_perf_opts() {
    local -a toks kept=()
    local i=0
    read -ra toks <<<"$1"
    while ((i < ${#toks[@]})); do
        case "${toks[i]}" in
            -n | -m | -j | -k | -l | -r)
                echo "WARNING: dropping '${toks[i]} ${toks[i + 1]:-}' from RUN_PERF_OPTS — set by this script." >&2
                i=$((i + 2))
                continue
                ;;
        esac
        kept+=("${toks[i]}")
        i=$((i + 1))
    done
    echo "${kept[*]}"
}
RUN_PERF_OPTS="$(sanitize_run_perf_opts "${RUN_PERF_OPTS}")"

# ─── Test identity ─────────────────────────────────────────────────────────────
BUILD_NUMBER="${BUILD_NUMBER:-0}"
TEST_ID="${BUILD_NUMBER}-$(date +%Y%m%d-%H%M%S)"
EKS_CLUSTER_NAME="apigw-jenkins-${TEST_ID}"
export CURRENT_DIR="${CURRENT_DIR:-$(realpath .)}"
export RESULTS_DIR="${RESULTS_DIR:-$(realpath "results-${TEST_ID}")}"
mkdir -p "${RESULTS_DIR}"

# ─── Constants (override via env if needed) ────────────────────────────────────
JMETER_KEY="${JMETER_KEY:-${HOME}/keys/apim-perf-test3.pem}"
JMETER_KEY_NAME="${JMETER_KEY_NAME:-apim-perf-test3}"
JMETER_TGZ="${JMETER_TGZ:-${HOME}/apache-jmeter-5.6.3.tgz}"
JMETER_USER="${JMETER_USER:-ec2-user}"
GATEWAY_HELM_CHART="${GATEWAY_HELM_CHART:-oci://ghcr.io/wso2/api-platform/helm-charts/gateway}"
EKS_NAMESPACE="${EKS_NAMESPACE:-api-gateway}"
EKS_RELEASE_NAME="${EKS_RELEASE_NAME:-ap-gateway}"
EKS_K8S_VERSION="${EKS_K8S_VERSION:-1.32}"
GATEWAY_RUNTIME_REPLICAS="${GATEWAY_RUNTIME_REPLICAS:-1}"
BACKEND_PORT=8688
EKS_STORAGE_CLASS="gp3-csi-auto"
GATEWAY_CONTROLLER_PORT="19090"
GATEWAY_MGMT_API_BASE="/api/management/v1"
JMETER_SERVERS_COUNT=2
# Results-path / JMeter infra defaults (override via env if needed; not part of RUN_PERF_OPTS).
PERF_HEAP_LABEL="${PERF_HEAP_LABEL:-16G}"
JMETER_SERVER_HEAP="${JMETER_SERVER_HEAP:-4G}"
JMETER_CLIENT_HEAP="${JMETER_CLIENT_HEAP:-2G}"
NETTY_SERVICE_HEAP="${NETTY_SERVICE_HEAP:-4G}"
RESPONSE_SIZE_BYTES="${RESPONSE_SIZE_BYTES:-1}"

PERF_COMMON_REPO="${PERF_COMMON_REPO:-https://github.com/Milanka00/performance-common.git}"
PERF_COMMON_BRANCH="${PERF_COMMON_BRANCH:-470-ai-api-perf}"
# API Gateway EKS JMeter scripts: single Jenkins param PERF_SCRIPTS=repo@branch:subdir
# (SSH remotes like git@host:org/repo.git@branch:subdir are supported — last @ / last : win.)
# Accept PERF_SCRIPT (singular) as an alias — common Jenkins naming slip.
# Defaults to the fork until gateway/perf/performance-test-scripts is merged into wso2/api-platform.
PERF_SCRIPTS="${PERF_SCRIPTS:-${PERF_SCRIPT:-https://github.com/wso2/api-platform.git@main:gateway/perf/performance-test-scripts}}"
PERF_SCRIPTS_SUBDIR="${PERF_SCRIPTS##*:}"
_perf_scripts_rest="${PERF_SCRIPTS%:*}"
PERF_SCRIPTS_BRANCH="${_perf_scripts_rest##*@}"
PERF_SCRIPTS_REPO="${_perf_scripts_rest%@*}"
unset _perf_scripts_rest
if [[ -z "${PERF_SCRIPTS_REPO}" || -z "${PERF_SCRIPTS_BRANCH}" || -z "${PERF_SCRIPTS_SUBDIR}" \
        || "${PERF_SCRIPTS_REPO}" == "${PERF_SCRIPTS}" ]]; then
    echo "ERROR: PERF_SCRIPTS must be repo@branch:subdir (got: ${PERF_SCRIPTS})" >&2
    exit 1
fi
# Stable local name after clone (rsync'd to JMeter EC2s under this path).
MANUAL_DIR_NAME="${MANUAL_DIR_NAME:-performance-test-scripts}"
MANUAL_DIR="${WORKSPACE}/${MANUAL_DIR_NAME}"
STATE_FILE="${CURRENT_DIR}/jmeter-ec2-state-${TEST_ID}.env"

# SSH helper used for all JMeter EC2 connections.
SSH_KEY="${JMETER_KEY}"
ssh_cmd() { ssh -o StrictHostKeyChecking=no -o ConnectTimeout=20 -i "${SSH_KEY}" "$@"; }
scp_cmd() { scp -o StrictHostKeyChecking=no -o ConnectTimeout=20 -i "${SSH_KEY}" "$@"; }
rsync_cmd() {
    rsync -az -e "ssh -o StrictHostKeyChecking=no -o ConnectTimeout=20 -i ${SSH_KEY}" "$@"
}

export EKS_CLUSTER_NAME AWS_REGION STATE_FILE EKS_DIR

echo "==================================================================="
echo " api-gateway EKS performance test"
echo " TEST_ID:  ${TEST_ID}"
echo " CLUSTER:  ${EKS_CLUSTER_NAME}"
echo " REGION:   ${AWS_REGION}"
echo " CHART:    ${GATEWAY_HELM_CHART}:${GATEWAY_HELM_CHART_VERSION}"
echo " SCRIPTS:  ${PERF_SCRIPTS}"
echo " RUN_OPTS: ${RUN_PERF_OPTS}"
echo "==================================================================="

# ─── EXIT trap ────────────────────────────────────────────────────────────────
cleanup_and_archive() {
    local rv=$?
    echo ""
    echo "==> Exit handler (rv=${rv})"
    "${SCRIPT_DIR}/cleanup.sh" || true
    local archive="${CURRENT_DIR}/archive"
    if [[ $rv -eq 0 ]]; then
        mkdir -p "${archive}/successful"
        [[ -d "${RESULTS_DIR}" ]] && mv -v "${RESULTS_DIR}" "${archive}/successful/" || true
        # Keep only the last 5 successful result sets.
        ls -1dt "${archive}/successful"/results-* 2>/dev/null | tail -n +6 | xargs rm -rf || true
    else
        mkdir -p "${archive}/failed"
        [[ -d "${RESULTS_DIR}" ]] && mv -v "${RESULTS_DIR}" "${archive}/failed/" || true
        # Keep only the last 3 failed result sets.
        ls -1dt "${archive}/failed"/results-* 2>/dev/null | tail -n +4 | xargs rm -rf || true
    fi
}
trap cleanup_and_archive EXIT

# ─── Step 1: clone deps (performance-common + gateway/perf scripts) ───────────
echo ""
echo "==> Step 1a: clone performance-common and build jtl-splitter"
PERF_COMMON_DIR="${WORKSPACE}/performance-common"
if [[ -d "${PERF_COMMON_DIR}/.git" ]]; then
    echo "    Already present — fetching ${PERF_COMMON_BRANCH}"
    git -C "${PERF_COMMON_DIR}" fetch --depth 1 origin "${PERF_COMMON_BRANCH}" 2>&1 | tail -3 || true
    git -C "${PERF_COMMON_DIR}" checkout -q FETCH_HEAD 2>/dev/null || \
        git -C "${PERF_COMMON_DIR}" checkout -q "${PERF_COMMON_BRANCH}" 2>/dev/null || true
else
    rm -rf "${PERF_COMMON_DIR}"
    git clone --depth 1 --branch "${PERF_COMMON_BRANCH}" "${PERF_COMMON_REPO}" "${PERF_COMMON_DIR}"
fi

echo "    Building jtl-splitter JAR (needed for generate-summary.sh)"
mvn -q -f "${PERF_COMMON_DIR}/components/jtl-splitter" package -DskipTests 2>&1 | tail -5

echo ""
echo "==> Step 1b: sparse-clone perf scripts only (${PERF_SCRIPTS})"
# Do NOT download the whole api-platform tree: depth-1 + tree:0 + cone sparse-checkout
# fetches only objects needed for PERF_SCRIPTS_SUBDIR (tiny compared to full clone).
PERF_SCRIPTS_CLONE="${WORKSPACE}/.perf-scripts-src"
rm -rf "${PERF_SCRIPTS_CLONE}"
git clone --depth 1 --filter=tree:0 --sparse \
    --branch "${PERF_SCRIPTS_BRANCH}" \
    "${PERF_SCRIPTS_REPO}" "${PERF_SCRIPTS_CLONE}"
git -C "${PERF_SCRIPTS_CLONE}" sparse-checkout set --cone "${PERF_SCRIPTS_SUBDIR}"

SRC_SCRIPTS="${PERF_SCRIPTS_CLONE}/${PERF_SCRIPTS_SUBDIR}"
[[ -d "${SRC_SCRIPTS}/api-gateway/eks" && -d "${SRC_SCRIPTS}/jmeter" ]] || {
    echo "ERROR: ${SRC_SCRIPTS} missing api-gateway/eks or jmeter after sparse clone." >&2
    echo "       Check PERF_SCRIPTS=${PERF_SCRIPTS}" >&2
    echo "       ${PERF_SCRIPTS_SUBDIR} does not exist on ${PERF_SCRIPTS_REPO}@${PERF_SCRIPTS_BRANCH}." >&2
    echo "       Checked out:" >&2
    (cd "${PERF_SCRIPTS_CLONE}" && find . -maxdepth 3 -type d -not -path './.git*' | head -20) >&2
    exit 1
}
# Flatten into a stable MANUAL_DIR name for remote EC2 paths.
rm -rf "${MANUAL_DIR}"
mkdir -p "${MANUAL_DIR}"
rsync -a --exclude='.git' "${SRC_SCRIPTS}/" "${MANUAL_DIR}/"
EKS_DIR="${MANUAL_DIR}/api-gateway/eks"
export EKS_DIR
echo "    MANUAL_DIR=${MANUAL_DIR}"

export PERF_ROOT="${WORKSPACE}"

# ─── Step 2: generate env.eks for this run ────────────────────────────────────
echo ""
echo "==> Step 2: generate env.eks"

EKS_GATEWAY_NODE_DESIRED="${GATEWAY_RUNTIME_REPLICAS}"
BACKEND_NAMESPACE="${EKS_NAMESPACE}"
MOCK_BACKEND_URL="http://perf-mock-backend.${BACKEND_NAMESPACE}.svc.cluster.local:${BACKEND_PORT}/v1"
PERF_CONFIG_TOML="${MANUAL_DIR}/api-gateway/config.perf-overlay.toml"

cat >"${EKS_DIR}/env.eks" <<ENVEKS
# Auto-generated by run-api-gateway-eks-tests.sh — do not edit.
export PERF_ROOT="${WORKSPACE}"
export API_GATEWAY_DIR="${MANUAL_DIR}/api-gateway"
export EKS_DIR="${EKS_DIR}"

export GATEWAY_HELM_CHART="${GATEWAY_HELM_CHART}"
export GATEWAY_HELM_CHART_VERSION="${GATEWAY_HELM_CHART_VERSION}"

export AWS_REGION="${AWS_REGION}"
export EKS_CLUSTER_NAME="${EKS_CLUSTER_NAME}"
export EKS_K8S_VERSION="${EKS_K8S_VERSION}"
export EKS_NODE_INSTANCE_TYPE="${GATEWAY_NODE_INSTANCE_TYPE}"
export EKS_GATEWAY_NODE_DESIRED="${EKS_GATEWAY_NODE_DESIRED}"
export EKS_NODE_MIN=1
export EKS_NODE_MAX=8
export EKS_BACKEND_NODE_INSTANCE_TYPE="${BACKEND_NODE_INSTANCE_TYPE}"

export EKS_NAMESPACE="${EKS_NAMESPACE}"
export EKS_RELEASE_NAME="${EKS_RELEASE_NAME}"
export EKS_STORAGE_CLASS="${EKS_STORAGE_CLASS}"
export EKS_LB_SCHEME="internal"

export GATEWAY_RUNTIME_REPLICAS="${GATEWAY_RUNTIME_REPLICAS}"
export GATEWAY_CONTROLLER_CPU_LIMIT="1"
export GATEWAY_CONTROLLER_MEM_LIMIT="2Gi"
export GATEWAY_CONTROLLER_PVC_SIZE="1Gi"
export GATEWAY_CONTROLLER_SQLITE_PATH="/app/data/gateway.db"
export GATEWAY_RUNTIME_CPU_LIMIT="4"
export GATEWAY_RUNTIME_MEM_LIMIT="2Gi"
export ROUTER_CONCURRENCY="${ROUTER_CONCURRENCY:-4}"
export GOMAXPROCS="${GOMAXPROCS:-4}"
export GOGC="${GOGC:-400}"
export GOMEMLIMIT="${GOMEMLIMIT:-1500MiB}"
export LOG_LEVEL="error"
export POLICY_ENGINE_METRICS_ENABLED="false"

export BACKEND_IN_CLUSTER=1
export BACKEND_NAMESPACE="${BACKEND_NAMESPACE}"
export BACKEND_SERVICE_NAME="perf-mock-backend"
export BACKEND_PORT="${BACKEND_PORT}"
export MOCK_BACKEND_URL="${MOCK_BACKEND_URL}"

export GATEWAY_CONTROLLER_PORT="${GATEWAY_CONTROLLER_PORT}"
export GATEWAY_MGMT_API_BASE="${GATEWAY_MGMT_API_BASE}"
export API_KEY_COUNT=10
export ARTIFACTS_DIR="\${ARTIFACTS_DIR:-\${HOME}/perf-artifacts}"
export JWT_KEYMANAGER_NAME="test"
# Resolves YOUR_TENANT in config.perf-overlay.toml; must match the app minting jwt-tokens.csv.
${JWT_OAUTH_TOKEN_URL:+export JWT_OAUTH_TOKEN_URL=${JWT_OAUTH_TOKEN_URL}}
${JWT_TENANT:+export JWT_TENANT=${JWT_TENANT}}
export RATELIMIT_REQUESTS=1000000000
export RATELIMIT_DURATION="24h"

export PERF_CONFIG_TOML="${PERF_CONFIG_TOML}"
export GATEWAY_CONTROLLER_ENCRYPTION_SECRET="${EKS_RELEASE_NAME}-controller-encryption-keys"
ENVEKS

echo "    env.eks written."

# ─── Step 3: create EKS cluster ───────────────────────────────────────────────
echo ""
echo "==> Step 3: create EKS cluster ${EKS_CLUSTER_NAME} (15-25 min)"

CLUSTER_CFG="${SCRIPT_DIR}/.eks-cluster-perf.generated.yaml"
export EKS_GATEWAY_NODE_DESIRED BACKEND_NODE_INSTANCE_TYPE EKS_K8S_VERSION
export EKS_NODE_INSTANCE_TYPE="${GATEWAY_NODE_INSTANCE_TYPE}"
export EKS_BACKEND_NODE_INSTANCE_TYPE="${BACKEND_NODE_INSTANCE_TYPE}"
envsubst <"${SCRIPT_DIR}/eks-cluster-perf.yaml.template" >"${CLUSTER_CFG}"

eksctl create cluster -f "${CLUSTER_CFG}"
aws eks update-kubeconfig --name "${EKS_CLUSTER_NAME}" --region "${AWS_REGION}"
kubectl get nodes -o wide
echo "    Waiting 30s for node daemonsets to initialize..."
sleep 30

# ─── Step 4: install gp3 storage class ────────────────────────────────────────
echo ""
echo "==> Step 4: install gp3-csi-auto storage class"
kubectl apply -f - <<'STORAGECLASS'
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: gp3-csi-auto
provisioner: ebs.csi.aws.com
volumeBindingMode: WaitForFirstConsumer
parameters:
  type: gp3
reclaimPolicy: Delete
allowVolumeExpansion: true
STORAGECLASS

# ─── Step 5: deploy in-cluster mock backend ───────────────────────────────────
echo ""
echo "==> Step 5: deploy in-cluster mock backend"
kubectl create namespace "${EKS_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f "${EKS_DIR}/backend-mock-eks.yaml"
if ! kubectl rollout status deployment/perf-mock-backend -n "${EKS_NAMESPACE}" --timeout=300s; then
    echo "ERROR: perf-mock-backend did not become ready. Diagnostics:"
    kubectl get nodes -o wide --show-labels || true
    kubectl get pods -n "${EKS_NAMESPACE}" -o wide || true
    kubectl describe pods -n "${EKS_NAMESPACE}" -l app=perf-mock-backend || true
    kubectl get events -n "${EKS_NAMESPACE}" --sort-by='.lastTimestamp' | tail -30 || true
    exit 1
fi
kubectl get pods -n "${EKS_NAMESPACE}" -l app=perf-mock-backend -o wide

# ─── Step 6: install gateway via Helm ─────────────────────────────────────────
echo ""
echo "==> Step 6: install gateway (Helm ${GATEWAY_HELM_CHART_VERSION})"
(cd "${EKS_DIR}" && source ./env.eks && ./install-gateway.sh)

# ─── Step 7: deploy minimal APIs ──────────────────────────────────────────────
echo ""
echo "==> Step 7: deploy minimal RestApis (api-plain, api-header, api-jwt)"
(cd "${EKS_DIR}" && source ./env.eks && ./deploy-apis-eks-minimal.sh)

# ─── Step 8: get NLB hostname ─────────────────────────────────────────────────
echo ""
echo "==> Step 8: wait for gateway NLB hostname"
GATEWAY_HOST=""
for attempt in $(seq 1 36); do
    GATEWAY_HOST=$(kubectl get svc "${EKS_RELEASE_NAME}-gateway-runtime" \
        -n "${EKS_NAMESPACE}" \
        -o jsonpath='{.status.loadBalancer.ingress[0].hostname}' 2>/dev/null || true)
    [[ -n "${GATEWAY_HOST}" && "${GATEWAY_HOST}" != "None" ]] && break
    echo "    Attempt ${attempt}/36: NLB pending, waiting 15s..."
    sleep 15
done
[[ -n "${GATEWAY_HOST}" ]] || { echo "ERROR: gateway NLB hostname not available after 9 min." >&2; exit 1; }
echo "    GATEWAY_HOST=${GATEWAY_HOST}"

# ─── Step 9: create JMeter EC2s ───────────────────────────────────────────────
echo ""
echo "==> Step 9: create JMeter EC2s in EKS VPC"
export JMETER_SG_TAG="jmeter-${TEST_ID}"
export JMETER_CLIENT_EC2_INSTANCE_TYPE JMETER_SERVER_EC2_INSTANCE_TYPE JMETER_KEY_NAME
"${SCRIPT_DIR}/create-jmeter-ec2s.sh"
# shellcheck source=/dev/null
source "${STATE_FILE}"

echo "==> Waiting 90s for EC2 cloud-init and SSH to be available..."
sleep 90

wait_for_ssh() {
    local host="$1"
    for attempt in $(seq 1 15); do
        ssh_cmd "${JMETER_USER}@${host}" "echo ssh-ok" 2>/dev/null && return 0
        echo "    SSH to ${host}: attempt ${attempt}/15, retrying..."
        sleep 15
    done
    echo "ERROR: SSH to ${host} timed out." >&2; return 1
}

wait_for_ssh "${CLIENT_PUBLIC_IP}"
wait_for_ssh "${SERVER1_PUBLIC_IP}"
wait_for_ssh "${SERVER2_PUBLIC_IP}"

# ─── Step 10: sync perf repo and setup JMeter on all 3 EC2s ──────────────────
echo ""
echo "==> Step 10: rsync perf repo + JMeter tarball to EC2s"

REMOTE_PERF_ROOT="~/perf-scripts"
REMOTE_PERF_HOME="~/perf-api-gateway-manual"

for host in "${CLIENT_PUBLIC_IP}" "${SERVER1_PUBLIC_IP}" "${SERVER2_PUBLIC_IP}"; do
    echo "    rsync → ${JMETER_USER}@${host}"
    rsync_cmd \
        --exclude='.git' --exclude='*.log' --exclude='results-*' --exclude='archive' \
        "${WORKSPACE}/" "${JMETER_USER}@${host}:${REMOTE_PERF_ROOT}/"
    echo "    scp JMeter tarball → ${host}"
    scp_cmd "${JMETER_TGZ}" "${JMETER_USER}@${host}:~/"
done

echo ""
echo "==> Step 10b: run setup-jmeter.sh on all EC2s"
for host in "${CLIENT_PUBLIC_IP}" "${SERVER1_PUBLIC_IP}" "${SERVER2_PUBLIC_IP}"; do
    echo "    setup-jmeter on ${JMETER_USER}@${host}"
    ssh_cmd "${JMETER_USER}@${host}" \
        "PERF_ROOT=${REMOTE_PERF_ROOT} PERF_HOME=${REMOTE_PERF_HOME} \
         JMETER_TGZ=~/$(basename "${JMETER_TGZ}") \
         ${REMOTE_PERF_ROOT}/${MANUAL_DIR_NAME}/jmeter/setup-jmeter.sh"
done

# ─── Step 11: start JMeter servers ────────────────────────────────────────────
# jmeter-server-start.sh requires -n <rmi_hostname> and -i <parent_of_apache-jmeter-*>.
# RMI hostname MUST be the private IP (same as -R list used by the client).
echo ""
echo "==> Step 11: start JMeter server processes on server EC2s"
declare -a _jmeter_server_public=("${SERVER1_PUBLIC_IP}" "${SERVER2_PUBLIC_IP}")
declare -a _jmeter_server_private=("${SERVER1_PRIVATE_IP}" "${SERVER2_PRIVATE_IP}")
for ix in "${!_jmeter_server_public[@]}"; do
    host="${_jmeter_server_public[$ix]}"
    private_ip="${_jmeter_server_private[$ix]}"
    echo "    Starting JMeter server on ${host} (RMI hostname=${private_ip})"
    # Do not wrap in outer nohup/& — the start script already nohups jmeter-server
    # and exits non-zero if ApacheJMeter.jar does not come up.
    ssh_cmd "${JMETER_USER}@${host}" \
        "${REMOTE_PERF_HOME}/jmeter/jmeter-server-start.sh -n ${private_ip} -i \$HOME -m ${JMETER_SERVER_HEAP}" \
        | tee "${RESULTS_DIR}/jmeter-server-${private_ip}.log" || {
            echo "ERROR: JMeter server failed to start on ${host}." >&2
            ssh_cmd "${JMETER_USER}@${host}" \
                "tail -80 ~/server.out 2>/dev/null; tail -40 ~/jmeter-server.log 2>/dev/null; pgrep -af ApacheJMeter || true" || true
            exit 1
        }
done

# Check JMeter RMI port from the client EC2 (not slave — port 1099 is only open within the JMeter SG).
echo "    Waiting for JMeter RMI port (1099) on server EC2s (checked from client)..."
for private_ip in "${SERVER1_PRIVATE_IP}" "${SERVER2_PRIVATE_IP}"; do
    ready=0
    for attempt in $(seq 1 24); do
        if ssh_cmd "${JMETER_USER}@${CLIENT_PUBLIC_IP}" \
            "nc -z -w3 ${private_ip} 1099 2>/dev/null"; then
            echo "    ${private_ip}:1099 open"
            ready=1
            break
        fi
        echo "    ${private_ip}:1099 not ready, attempt ${attempt}/24, waiting 5s..."
        sleep 5
    done
    if [[ "${ready}" -ne 1 ]]; then
        echo "ERROR: ${private_ip}:1099 never opened after ~2 min." >&2
        exit 1
    fi
done

# ─── Step 12: configure JMeter client (SSH aliases + env.jmeter) ──────────────
echo ""
echo "==> Step 12: configure JMeter client distributed SSH and env"

# Copy EC2 key to client for server-to-server SSH (needed by JMeter RMI setup).
scp_cmd "${SSH_KEY}" "${JMETER_USER}@${CLIENT_PUBLIC_IP}:~/.ssh/jmeter-servers.pem"
ssh_cmd "${JMETER_USER}@${CLIENT_PUBLIC_IP}" "chmod 600 ~/.ssh/jmeter-servers.pem"

# Write ~/.ssh/config on client with jmeter1/jmeter2 aliases (private IPs).
ssh_cmd "${JMETER_USER}@${CLIENT_PUBLIC_IP}" bash <<SSHCONFIG
mkdir -p ~/.ssh && chmod 700 ~/.ssh
cat >>~/.ssh/config <<'CFGENTRY'

Host jmeter1
    HostName ${SERVER1_PRIVATE_IP}
    User ${JMETER_USER}
    IdentityFile ~/.ssh/jmeter-servers.pem
    StrictHostKeyChecking no

Host jmeter2
    HostName ${SERVER2_PRIVATE_IP}
    User ${JMETER_USER}
    IdentityFile ~/.ssh/jmeter-servers.pem
    StrictHostKeyChecking no
CFGENTRY
chmod 600 ~/.ssh/config
SSHCONFIG

# Generate env.jmeter locally and scp to client (avoids remote heredoc quoting issues).
# IMPORTANT: also write env.jmeter.api — run-scenario.sh sources that profile AFTER
# env.jmeter and would otherwise fall back to env.jmeter.api.example (hardcoded EC2 IPs).
ENV_JMETER_TMP=$(mktemp)
cat >"${ENV_JMETER_TMP}" <<ENVJMETER
# Auto-generated by Jenkins api-gateway perf job — do not edit.
_DIR="\$(cd "\$(dirname "\${BASH_SOURCE[0]}")" && pwd)"
_MANUAL_DIR="\$(cd "\${_DIR}/.." && pwd)"
export PERF_ROOT=${REMOTE_PERF_ROOT}
export MANUAL_DIR="\${_MANUAL_DIR}"
source "\${_MANUAL_DIR}/common.env.example"

export PERF_GATEWAY_TYPE=api-gateway
export GATEWAY_HOST=${GATEWAY_HOST}
export GATEWAY_PORT=8080

# Backend is in-cluster — JMeter does not SSH to it; gateway pods reach ClusterIP mock.
export BACKEND_IN_CLUSTER=1
export BACKEND_HOST=127.0.0.1
export BACKEND_IMPL=docker
unset BACKEND_SSH BACKEND_SSH_KEY || true
export BACKEND_SSH=
export BACKEND_SSH_KEY=
export BACKEND_PORT=${BACKEND_PORT}

# Asgardeo OAuth for JWT token generation (only needed for api_api_jwt_* scenarios).
# If not set, jwt-tokens.csv must already exist on the JMeter client.
${JWT_OAUTH_TOKEN_URL:+export JWT_OAUTH_TOKEN_URL=${JWT_OAUTH_TOKEN_URL}}
${JWT_OAUTH_CLIENT_ID:+export JWT_OAUTH_CLIENT_ID=${JWT_OAUTH_CLIENT_ID}}
${JWT_OAUTH_CLIENT_SECRET:+export JWT_OAUTH_CLIENT_SECRET=${JWT_OAUTH_CLIENT_SECRET}}

export PERF_HOME=${REMOTE_PERF_HOME}
export GATEWAY_ARTIFACTS=~/perf-artifacts
export JMETER_ALIAS_PREFIX=jmeter

# Skip per-scenario JTL split — generate-summary.sh handles it at the end.
export SKIP_JTL_SPLIT=true
# api-gateway does not need AI payload files.
export SKIP_PAYLOAD_GENERATION=true
ENVJMETER

for _env_name in env.jmeter env.jmeter.api; do
    scp_cmd "${ENV_JMETER_TMP}" \
        "${JMETER_USER}@${CLIENT_PUBLIC_IP}:${REMOTE_PERF_ROOT}/${MANUAL_DIR_NAME}/jmeter/${_env_name}"
done
rm -f "${ENV_JMETER_TMP}"
echo "    env.jmeter + env.jmeter.api uploaded (GATEWAY_HOST=${GATEWAY_HOST})."
# ─── Step 13: run perf scenarios on JMeter client ────────────────────────────
echo ""
echo "==> Step 13: run perf scenarios (RUN_PERF_OPTS: ${RUN_PERF_OPTS})"
echo "    Infra defaults: -n ${JMETER_SERVERS_COUNT} -m ${PERF_HEAP_LABEL} -j ${JMETER_SERVER_HEAP} -k ${JMETER_CLIENT_HEAP} -l ${NETTY_SERVICE_HEAP} -r ${RESPONSE_SIZE_BYTES}"

# Write run script on client to avoid shell-quoting issues with RUN_PERF_OPTS.
# RUN_PERF_OPTS should only carry -u/-b/-s/-d/-w/-i/-e (and repeats of those).
ssh_cmd "${JMETER_USER}@${CLIENT_PUBLIC_IP}" bash <<RUNSCRIPT
set -eo pipefail
cd ${REMOTE_PERF_ROOT}/${MANUAL_DIR_NAME}/jmeter
source ./env.jmeter
./run-scenario.sh -g api-gateway \
    -n ${JMETER_SERVERS_COUNT} \
    -m ${PERF_HEAP_LABEL} \
    -j ${JMETER_SERVER_HEAP} \
    -k ${JMETER_CLIENT_HEAP} \
    -l ${NETTY_SERVICE_HEAP} \
    -r ${RESPONSE_SIZE_BYTES} \
    ${RUN_PERF_OPTS} \
    2>&1 | tee /tmp/perf-run.log
RUNSCRIPT

# ─── Step 14: generate summary on JMeter client ───────────────────────────────
echo ""
echo "==> Step 14: generate summary CSV on JMeter client"
ssh_cmd "${JMETER_USER}@${CLIENT_PUBLIC_IP}" bash <<GENSUMMARY
set -e
cd ${REMOTE_PERF_ROOT}/${MANUAL_DIR_NAME}/jmeter
source ./env.jmeter
PERF_HOME=${REMOTE_PERF_HOME} ./generate-summary.sh
GENSUMMARY

# ─── Step 15: download results to slave ───────────────────────────────────────
echo ""
echo "==> Step 15: download results to ${RESULTS_DIR}"
mkdir -p "${RESULTS_DIR}/jmeter-results"

rsync_cmd \
    "${JMETER_USER}@${CLIENT_PUBLIC_IP}:${REMOTE_PERF_HOME}/jmeter/results/" \
    "${RESULTS_DIR}/jmeter-results/" 2>/dev/null || true

rsync_cmd \
    "${JMETER_USER}@${CLIENT_PUBLIC_IP}:${REMOTE_PERF_HOME}/jmeter/summary.csv" \
    "${RESULTS_DIR}/" 2>/dev/null || true

rsync_cmd \
    "${JMETER_USER}@${CLIENT_PUBLIC_IP}:/tmp/perf-run.log" \
    "${RESULTS_DIR}/" 2>/dev/null || true

if [[ -f "${RESULTS_DIR}/summary.csv" ]]; then
    echo ""
    echo "==================================================================="
    echo " RESULTS SUMMARY"
    echo "==================================================================="
    column -t -s ',' "${RESULTS_DIR}/summary.csv"
    echo "==================================================================="
    # Copy to Jenkins job workspace so "Archive the artifacts" can expose it.
    if [[ -n "${JENKINS_JOB_WORKSPACE}" ]]; then
        cp "${RESULTS_DIR}/summary.csv" \
           "${JENKINS_JOB_WORKSPACE}/summary-${TEST_ID}.csv"
        echo "    Archived to Jenkins workspace: summary-${TEST_ID}.csv"
    fi
fi

echo ""
echo "==================================================================="
echo " Performance test complete."
echo " Summary: ${RESULTS_DIR}/summary.csv"
echo " Results: ${RESULTS_DIR}/jmeter-results/"
echo "==================================================================="

# ─── Optional: publish formatted results PR ───────────────────────────────────
# Set PUBLISH_RESULTS_PR=1 (and GH_TOKEN) on the Jenkins job to open a PR that
# updates RESULTS_PR_README (default gateway/perf/README.md) in RESULTS_PR_REPO.
if [[ "${PUBLISH_RESULTS_PR:-0}" == "1" ]]; then
    echo ""
    echo "==> Step 16: publish results PR"
    SUMMARY_CSV="${JENKINS_JOB_WORKSPACE}/summary-${TEST_ID}.csv"
    [[ -f "${SUMMARY_CSV}" ]] || SUMMARY_CSV="${RESULTS_DIR}/summary.csv"
    SUMMARY_CSV="${SUMMARY_CSV}" \
    TEST_ID="${TEST_ID}" \
    JENKINS_JOB_WORKSPACE="${JENKINS_JOB_WORKSPACE}" \
        "${SCRIPT_DIR}/publish-results-pr.sh" || {
            echo "WARNING: publish-results-pr.sh failed (perf run itself succeeded)." >&2
        }
fi

# EXIT trap runs cleanup_and_archive → cleanup.sh
