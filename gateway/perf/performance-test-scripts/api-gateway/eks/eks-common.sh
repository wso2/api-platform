#!/bin/bash
# Shared helpers for EKS gateway perf scripts.

eks_require_cmd() {
    local cmd
    for cmd in "$@"; do
        command -v "$cmd" >/dev/null 2>&1 || {
            echo "Missing required command: ${cmd}" >&2
            exit 1
        }
    done
}

eks_load_env() {
    local script_dir
    script_dir="$(cd "$(dirname "${BASH_SOURCE[1]}")" && pwd)"
    if [[ -f "${script_dir}/env.eks" ]]; then
        # shellcheck source=env.eks
        source "${script_dir}/env.eks"
    elif [[ -f "${script_dir}/env.eks.example" ]]; then
        echo "WARNING: using env.eks.example — copy to env.eks and edit" >&2
        # shellcheck source=env.eks.example
        source "${script_dir}/env.eks.example"
    else
        echo "Missing env.eks (copy from env.eks.example)" >&2
        exit 1
    fi
    export EKS_DIR="${EKS_DIR:-${script_dir}}"
}

eks_clear_localhost_proxy_if_needed() {
    local _p _v
    [[ -z "${EKS_KEEP_PROXY:-}" ]] || return 0
    for _p in HTTP_PROXY HTTPS_PROXY ALL_PROXY http_proxy https_proxy all_proxy; do
        _v="${!_p:-}"
        if [[ "$_v" =~ ^https?://127\.0\.0\.1: ]] || [[ "$_v" =~ ^socks5?://127\.0\.0\.1: ]]; then
            unset "$_p"
        fi
    done
}

eks_helm_chart_ok() {
    if [[ "${GATEWAY_HELM_CHART}" == oci://* ]]; then
        return 0
    fi
    [[ -f "${GATEWAY_HELM_CHART}/Chart.yaml" ]] || {
        echo "Helm chart not found: ${GATEWAY_HELM_CHART}" >&2
        exit 1
    }
}

eks_kubecontext() {
    eks_clear_localhost_proxy_if_needed
    local _args=(--name "${EKS_CLUSTER_NAME}" --region "${AWS_REGION}")
    [[ -n "${AWS_PROFILE:-}" ]] && _args+=(--profile "${AWS_PROFILE}")
    aws eks describe-cluster "${_args[@]}" >/dev/null 2>&1 || {
        echo "EKS cluster '${EKS_CLUSTER_NAME}' not found in ${AWS_REGION}" >&2
        echo "Create it in AWS Console, then: ./connect-kubeconfig.sh" >&2
        exit 1
    }
    aws eks update-kubeconfig "${_args[@]}" >/dev/null
}

# Build helm chart ref for install/upgrade (bash 3.2 safe — no mapfile).
# Usage: helm_args=(-n "$EKS_NAMESPACE"); eks_helm_append_chart_ref helm_args; helm_args+=(-f ...)
eks_helm_append_chart_ref() {
    local _var="$1"
    eval "${_var}+=(\"${EKS_RELEASE_NAME}\" \"${GATEWAY_HELM_CHART}\")"
    if [[ -n "${GATEWAY_HELM_CHART_VERSION:-}" ]]; then
        eval "${_var}+=(--version \"${GATEWAY_HELM_CHART_VERSION}\")"
    fi
}

eks_controller_svc() {
    echo "${EKS_RELEASE_NAME}-controller"
}

eks_runtime_svc() {
    echo "${EKS_RELEASE_NAME}-gateway-runtime"
}

# Validate selected storage class and ensure it is CSI-backed.
eks_validate_storage_class() {
    local sc prov
    sc="${EKS_STORAGE_CLASS:-}"
    [[ -n "$sc" ]] || {
        echo "EKS_STORAGE_CLASS is empty. Set a CSI-backed class in env.eks (e.g., gp3-csi-auto)." >&2
        return 1
    }
    prov="$(kubectl get storageclass "$sc" -o jsonpath='{.provisioner}' 2>/dev/null || true)"
    [[ -n "$prov" ]] || {
        echo "StorageClass '${sc}' not found. Create one and set EKS_STORAGE_CLASS." >&2
        return 1
    }
    if [[ "$prov" == "kubernetes.io/aws-ebs" ]]; then
        echo "StorageClass '${sc}' uses legacy provisioner kubernetes.io/aws-ebs." >&2
        echo "Use CSI-backed class (provisioner ebs.csi.eks.amazonaws.com) to avoid Pending PVC." >&2
        return 1
    fi
}

# Patch controller PVC storage class if it exists with empty storageClassName.
eks_fix_controller_pvc_storage_class() {
    local pvc sc current_sc
    pvc="${EKS_RELEASE_NAME}-controller-data"
    sc="${EKS_STORAGE_CLASS:-}"
    [[ -n "$sc" ]] || return 0
    if ! kubectl get pvc "$pvc" -n "${EKS_NAMESPACE}" >/dev/null 2>&1; then
        return 0
    fi
    current_sc="$(kubectl get pvc "$pvc" -n "${EKS_NAMESPACE}" -o jsonpath='{.spec.storageClassName}' 2>/dev/null || true)"
    if [[ -z "$current_sc" ]]; then
        echo "==> Patching PVC ${pvc} storageClassName=${sc}"
        kubectl patch pvc "$pvc" -n "${EKS_NAMESPACE}" -p "{\"spec\":{\"storageClassName\":\"${sc}\"}}" >/dev/null || true
    fi
}

# The committed overlay ships a YOUR_TENANT placeholder in the jwt-auth issuer/JWKS URLs.
# Resolve it from JWT_TENANT, or derive it from JWT_OAUTH_TOKEN_URL (the same Asgardeo app
# that mints the test tokens) so the validator and the token source cannot drift apart.
eks_jwt_tenant() {
    if [[ -n "${JWT_TENANT:-}" ]]; then
        echo "${JWT_TENANT}"
        return 0
    fi
    # https://api.asgardeo.io/t/<tenant>/oauth2/token -> <tenant>
    if [[ "${JWT_OAUTH_TOKEN_URL:-}" =~ /t/([^/]+)/ ]]; then
        echo "${BASH_REMATCH[1]}"
    fi
}

# Sections from config.perf-overlay.toml safe to append after Helm-generated config.toml
# (no duplicate [router], [controller], [policy_engine], etc.).
eks_append_perf_config_toml() {
    local overlay="${PERF_CONFIG_TOML:?Set PERF_CONFIG_TOML}"
    [[ -f "$overlay" ]] || {
        echo "Perf overlay not found: ${overlay}" >&2
        return 1
    }
    local tenant
    tenant="$(eks_jwt_tenant)"
    if [[ -z "${tenant}" ]] && grep -q 'YOUR_TENANT' "$overlay"; then
        # Every jwt-auth request 401s when the issuer stays on the placeholder.
        echo "WARNING: overlay still has YOUR_TENANT and no tenant resolved." >&2
        echo "         Set JWT_TENANT or JWT_OAUTH_TOKEN_URL, else jwt scenarios return 401." >&2
    fi
    awk '
        /^\[router\.upstream\.circuit_breakers\]/ { emit = 1 }
        /^\[policy_configurations/ { emit = 1; in_policy = 1 }
        /^\[\[policy_configurations/ { emit = 1; in_policy = 1 }
        /^\[immutable_gateway\]/ { emit = 0; in_policy = 0; next }
        /^\[/ {
            if (emit && !in_policy && $0 !~ /^\[router\.upstream\.circuit_breakers\]/) {
                emit = 0
            }
            if (emit && in_policy && $0 !~ /policy_configurations/) {
                in_policy = 0
                emit = 0
            }
        }
        emit { print }
    ' "$overlay" \
        | if [[ -n "${tenant}" ]]; then sed "s#YOUR_TENANT#${tenant}#g"; else cat; fi
}

eks_generated_values_path() {
    echo "${EKS_DIR}/.generated-values.yaml"
}

# Build runtime extraEnv lines (GOGC / GOMEMLIMIT when set — Jenkins defaults both on).
eks_runtime_extra_env_yaml() {
    if [[ -n "${GOGC:-}" ]]; then
        echo "        - name: GOGC"
        echo "          value: \"${GOGC}\""
    fi
    if [[ -n "${GOMEMLIMIT:-}" ]]; then
        echo "        - name: GOMEMLIMIT"
        echo "          value: \"${GOMEMLIMIT}\""
    fi
}

# Recreate runtime Deployment before helm upgrade (clears kubectl-patch SSA conflicts).
eks_reset_runtime_deployment_for_helm() {
    local deploy
    deploy="${EKS_RELEASE_NAME}-gateway-runtime"
    if kubectl get deployment "$deploy" -n "${EKS_NAMESPACE}" >/dev/null 2>&1; then
        echo "==> Recreating ${deploy} (clear field-manager conflicts from kubectl patch)"
        kubectl delete deployment "$deploy" -n "${EKS_NAMESPACE}" --wait=true
    fi
}

# Patch runtime Deployment args (custom images only; e.g. --rtr.disable-hot-restart).
eks_patch_runtime_args() {
    local deploy extra
    deploy="${EKS_RELEASE_NAME}-gateway-runtime"
    extra="${GATEWAY_RUNTIME_EXTRA_ARGS:-}"
    [[ -n "$extra" ]] || return 0

    kubectl patch deployment "$deploy" -n "${EKS_NAMESPACE}" --type=strategic -p "
spec:
  template:
    spec:
      containers:
      - name: gateway-runtime
        args:
        - --pol.config
        - /etc/policy-engine/config.toml
        - ${extra}
"
    kubectl rollout status -n "${EKS_NAMESPACE}" "deployment/${deploy}" --timeout=300s
}

# Optional YAML fragment for image override (omit when unset → chart defaults).
eks_helm_controller_image_yaml() {
    [[ -n "${GATEWAY_CONTROLLER_IMAGE:-}" ]] || return 0
    cat <<EOF
    image:
      repository: ${GATEWAY_CONTROLLER_IMAGE%:*}
      tag: ${GATEWAY_CONTROLLER_IMAGE##*:}
      pullPolicy: IfNotPresent
EOF
}

eks_helm_runtime_image_yaml() {
    [[ -n "${GATEWAY_RUNTIME_IMAGE:-}" ]] || return 0
    cat <<EOF
    image:
      repository: ${GATEWAY_RUNTIME_IMAGE%:*}
      tag: ${GATEWAY_RUNTIME_IMAGE##*:}
      pullPolicy: IfNotPresent
EOF
}

eks_helm_images_label() {
    if [[ -n "${GATEWAY_CONTROLLER_IMAGE:-}" || -n "${GATEWAY_RUNTIME_IMAGE:-}" ]]; then
        echo "controller=${GATEWAY_CONTROLLER_IMAGE:-chart default} runtime=${GATEWAY_RUNTIME_IMAGE:-chart default}"
    else
        echo "Helm chart defaults (ghcr.io/wso2/api-platform gateway-controller and gateway-runtime)"
    fi
}

# Private GHCR images (non-wso2 org) need a pull secret on EKS nodes.
eks_uses_private_ghcr_images() {
    [[ -n "${EKS_GHCR_PULL_SECRET:-}" ]] && return 0
    [[ "${GATEWAY_CONTROLLER_IMAGE:-}" == *ghcr.io* && "${GATEWAY_CONTROLLER_IMAGE:-}" != *wso2/api-platform* ]] && return 0
    [[ "${GATEWAY_RUNTIME_IMAGE:-}" == *ghcr.io* && "${GATEWAY_RUNTIME_IMAGE:-}" != *wso2/api-platform* ]] && return 0
    return 1
}

eks_ensure_ghcr_pull_secret() {
    local secret_name docker_config
    eks_uses_private_ghcr_images || return 0
    secret_name="${EKS_GHCR_PULL_SECRET:-ghcr-pull}"
    docker_config="${DOCKER_CONFIG:-${HOME}/.docker}/config.json"
    [[ -f "$docker_config" ]] || {
        echo "ERROR: ${docker_config} not found — run 'docker login ghcr.io' first" >&2
        exit 1
    }
    if ! grep -q '"ghcr.io"' "$docker_config" 2>/dev/null; then
        echo "ERROR: no ghcr.io credentials in ${docker_config} — run 'docker login ghcr.io'" >&2
        exit 1
    fi
    echo "==> Ensure image pull secret: ${secret_name}"
    kubectl create secret generic "${secret_name}" \
        --from-file=.dockerconfigjson="${docker_config}" \
        --type=kubernetes.io/dockerconfigjson \
        -n "${EKS_NAMESPACE}" \
        --dry-run=client -o yaml | kubectl apply -f -
}

eks_helm_image_pull_secrets_yaml() {
    local secret_name
    eks_uses_private_ghcr_images || return 0
    secret_name="${EKS_GHCR_PULL_SECRET:-ghcr-pull}"
    cat <<EOF
imagePullSecrets:
  - ${secret_name}
EOF
}

eks_controller_encryption_secret_name() {
    echo "${GATEWAY_CONTROLLER_ENCRYPTION_SECRET:-${EKS_RELEASE_NAME}-controller-encryption-keys}"
}

eks_ensure_controller_encryption_key_secret() {
    local secret_name tmp_dir key_file
    secret_name="$(eks_controller_encryption_secret_name)"
    if kubectl get secret "${secret_name}" -n "${EKS_NAMESPACE}" >/dev/null 2>&1; then
        return 0
    fi

    tmp_dir="$(mktemp -d)"
    key_file="${tmp_dir}/default-aesgcm256-v1.bin"
    python3 - "$key_file" <<'PY'
import os
import sys
with open(sys.argv[1], "wb") as f:
    f.write(os.urandom(32))
PY

    kubectl create secret generic "${secret_name}" \
        --from-file=default-aesgcm256-v1.bin="${key_file}" \
        -n "${EKS_NAMESPACE}" \
        --dry-run=client -o yaml | kubectl apply -f -

    rm -rf "${tmp_dir}"
}

# Build Helm values aligned with docker-compose.perf.yaml + config.perf-overlay.toml.
eks_write_generated_values() {
    local out replicas ctrl_cpu ctrl_mem ctrl_pvc_size ctrl_sqlite_path rt_cpu rt_mem lb_type extra_env
    local ctrl_image_yaml rt_image_yaml pull_secrets_yaml
    local ctrl_encryption_secret
    out="$(eks_generated_values_path)"
    replicas="${GATEWAY_RUNTIME_REPLICAS:-1}"
    ctrl_cpu="${GATEWAY_CONTROLLER_CPU_LIMIT:-1}"
    ctrl_mem="${GATEWAY_CONTROLLER_MEM_LIMIT:-2Gi}"
    ctrl_pvc_size="${GATEWAY_CONTROLLER_PVC_SIZE:-1Gi}"
    ctrl_sqlite_path="${GATEWAY_CONTROLLER_SQLITE_PATH:-/app/data/gateway.db}"
    rt_cpu="${GATEWAY_RUNTIME_CPU_LIMIT:-4}"
    rt_mem="${GATEWAY_RUNTIME_MEM_LIMIT:-2Gi}"
    lb_type="nlb"
    extra_env="$(eks_runtime_extra_env_yaml)"
    ctrl_image_yaml="$(eks_helm_controller_image_yaml)"
    rt_image_yaml="$(eks_helm_runtime_image_yaml)"
    pull_secrets_yaml="$(eks_helm_image_pull_secrets_yaml)"
    ctrl_encryption_secret="$(eks_controller_encryption_secret_name)"
    log_level="${LOG_LEVEL:-info}"
    policy_engine_metrics="${POLICY_ENGINE_METRICS_ENABLED:-true}"

  cat >"$out" <<EOF
# Auto-generated — mirrors docker-compose.perf.yaml + config.perf-overlay.toml
# Re-run: ./install-gateway.sh
${pull_secrets_yaml}
gateway:
  # APIP_GW_DEVELOPMENT_MODE=true (compose controller env)
  developmentMode: true

  controller:
${ctrl_image_yaml}
    controlPlane:
      # Standalone gateway mode: no external APIM control-plane.
      host: ""
      token:
        value: ""
    tls:
      enabled: false
    encryptionKeys:
      enabled: true
      secretName: "${ctrl_encryption_secret}"
    persistence:
      enabled: true
      size: ${ctrl_pvc_size}
      storageClass: "${EKS_STORAGE_CLASS}"
    storage:
      type: sqlite
      sqlitePath: ${ctrl_sqlite_path}
    deployment:
      replicaCount: 1
      # gateway-controller image runs as wso2 (uid/gid 10001); EBS PVCs mount root-owned
      # unless fsGroup is set — without it SQLite fails with "unable to open database file".
      podSecurityContext:
        fsGroup: 10001
      securityContext:
        runAsUser: 10001
        runAsGroup: 10001
        runAsNonRoot: true
      strategy:
        type: RollingUpdate
        rollingUpdate:
          maxSurge: 0
          maxUnavailable: 1
      resources:
        limits:
          cpu: ${ctrl_cpu}
          memory: ${ctrl_mem}
        requests:
          cpu: ${ctrl_cpu}
          memory: ${ctrl_mem}
    logging:
      level: ${log_level}

  gatewayRuntime:
${rt_image_yaml}
    deployment:
      replicaCount: ${replicas}
      strategy:
        type: RollingUpdate
        rollingUpdate:
          maxSurge: 0
          maxUnavailable: 1
      env:
        logLevel: ${log_level}
      extraEnv:
        - name: ROUTER_CONCURRENCY
          value: "${ROUTER_CONCURRENCY:-4}"
        - name: GOMAXPROCS
          value: "${GOMAXPROCS:-4}"
        - name: APIP_GW_POLICY_ENGINE_METRICS_ENABLED
          value: "${policy_engine_metrics}"
${extra_env}
      resources:
        limits:
          cpu: ${rt_cpu}
          memory: ${rt_mem}
        requests:
          cpu: ${rt_cpu}
          memory: ${rt_mem}
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchLabels:
                    app.kubernetes.io/component: gateway-runtime
                topologyKey: kubernetes.io/hostname
    service:
      type: LoadBalancer
      annotations:
        service.beta.kubernetes.io/aws-load-balancer-type: "${lb_type}"
        service.beta.kubernetes.io/aws-load-balancer-scheme: "${EKS_LB_SCHEME:-internal}"
        service.beta.kubernetes.io/aws-load-balancer-cross-zone-load-balancing-enabled: "true"

  config:
    analytics:
      enabled: false
    immutable_gateway:
      enabled: false
      artifacts_dir: "/etc/api-platform-gateway/immutable_gateway/artifacts"
    controller:
      storage:
        type: sqlite
        sqlite:
          path: ${ctrl_sqlite_path}
      server:
        gateway_id: platform-gateway-id
      logging:
        level: ${log_level}
      controlplane:
        insecure_skip_verify: true
        gateway_name: default
      auth:
        basic:
          enabled: true
          users:
            - username: admin
              password: admin
              password_hashed: false
              roles: ["admin"]
    router:
      gateway_host: "*"
      https_enabled: false
      access_logs:
        enabled: false
    policy_engine:
      logging:
        level: ${log_level}
      metrics:
        enabled: ${policy_engine_metrics}

  # Append-only overlay sections (policies, circuit breakers, api_key) — avoids TOML duplicate tables.
  config_toml: |
EOF

    eks_append_perf_config_toml | sed 's/^/    /' >>"$out"
}

eks_port_forward_controller() {
    local pidfile="${EKS_DIR}/.controller-port-forward.pid"
    local logfile="${EKS_DIR}/.controller-port-forward.log"
    local port i
    port="${GATEWAY_CONTROLLER_PORT:-9090}"
    if [[ -f "$pidfile" ]] && kill -0 "$(cat "$pidfile")" 2>/dev/null; then
        eks_wait_controller_ready && return 0
    fi
    eks_stop_port_forward_controller
    : >"$logfile"
    kubectl port-forward -n "${EKS_NAMESPACE}" \
        "svc/$(eks_controller_svc)" \
        "${port}:9090" \
        >>"$logfile" 2>&1 &
    echo $! >"$pidfile"
    disown
    eks_wait_controller_ready
}

# Controller Service exposes REST :9090 only (admin :9092 is pod-local, not on the Service).
eks_wait_controller_ready() {
    local port i user pass base_paths base status
    port="${GATEWAY_CONTROLLER_PORT:-9090}"
    user="${GATEWAY_MGMT_USER:-admin}"
    pass="${GATEWAY_MGMT_PASS:-admin}"
    # Prefer chart-aligned base first, then probe both (1.1.0=v0.9, 1.2+=v1).
    base_paths=("${GATEWAY_MGMT_API_BASE:-}" "/api/management/v0.9" "/api/management/v1")
    # Deduplicate empty / repeats while preserving order
    local -a uniq=()
    local b seen
    for b in "${base_paths[@]}"; do
        [[ -n "$b" ]] || continue
        seen=0
        for u in "${uniq[@]-}"; do
            [[ "$u" == "$b" ]] && { seen=1; break; }
        done
        [[ "$seen" -eq 0 ]] && uniq+=("$b")
    done
    base_paths=("${uniq[@]}")
    for i in $(seq 1 30); do
        for base in "${base_paths[@]}"; do
            status="$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 2 --max-time 5 \
                -u "${user}:${pass}" "http://127.0.0.1:${port}${base}/rest-apis" || true)"
            if [[ "${status}" == "200" ]]; then
                export GATEWAY_MGMT_API_BASE="${base}"
                return 0
            fi
        done
        sleep 1
    done
    echo "Controller not reachable via port-forward (REST :${port})" >&2
    local logfile="${EKS_DIR}/.controller-port-forward.log"
    [[ -f "$logfile" ]] && tail -20 "$logfile" >&2
    return 1
}

eks_stop_port_forward_controller() {
    local pidfile="${EKS_DIR}/.controller-port-forward.pid"
    local pid
    [[ -f "$pidfile" ]] || return 0
    pid="$(cat "$pidfile")"
    kill "$pid" 2>/dev/null || true
    sleep 1
    kill -0 "$pid" 2>/dev/null && kill -9 "$pid" 2>/dev/null || true
    rm -f "$pidfile"
}

eks_runtime_hostname() {
    local svc ip host
    svc="$(eks_runtime_svc)"
    ip=$(kubectl get svc -n "${EKS_NAMESPACE}" "$svc" \
        -o jsonpath='{.status.loadBalancer.ingress[0].hostname}' 2>/dev/null || true)
    if [[ -n "$ip" ]]; then
        echo "$ip"
        return 0
    fi
    ip=$(kubectl get svc -n "${EKS_NAMESPACE}" "$svc" \
        -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)
    [[ -n "$ip" ]] && echo "$ip"
}

eks_wait_runtime_lb() {
    local i host
    echo "Waiting for runtime LoadBalancer hostname..."
    for i in $(seq 1 60); do
        host="$(eks_runtime_hostname)"
        if [[ -n "$host" ]]; then
            echo "$host"
            return 0
        fi
        sleep 10
    done
    echo "Timed out waiting for LoadBalancer" >&2
    return 1
}
