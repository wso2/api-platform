#!/bin/bash -e
# Distributed API Gateway perf runner (plain / header / jwt scenarios).

PERF_GATEWAY_TYPE="${PERF_GATEWAY_TYPE:-api-gateway}"
_cli_gateway_type=""
_forward_args=()
while [[ $# -gt 0 ]]; do
    case "$1" in
    -g)
        PERF_GATEWAY_TYPE="$2"
        _cli_gateway_type="$2"
        shift 2
        ;;
    *)
        _forward_args+=("$1")
        shift
        ;;
    esac
done
set -- "${_forward_args[@]}"
export PERF_GATEWAY_TYPE

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
MANUAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
if [[ -f "${SCRIPT_DIR}/env.jmeter" ]]; then
    # shellcheck source=env.jmeter
    source "${SCRIPT_DIR}/env.jmeter"
else
    # shellcheck source=env.example
    source "${SCRIPT_DIR}/env.example"
fi
# CLI -g wins over PERF_GATEWAY_TYPE in env.jmeter
if [[ -n "${_cli_gateway_type}" ]]; then
    export PERF_GATEWAY_TYPE="${_cli_gateway_type}"
fi
# shellcheck source=lib/gateway-profile.sh
source "${SCRIPT_DIR}/lib/gateway-profile.sh"
load_jmeter_gateway_profile "${SCRIPT_DIR}"
# shellcheck source=../lib/common.sh
source "${MANUAL_DIR}/lib/common.sh"
require_bash4

if [[ "${PERF_GATEWAY_TYPE}" != "api-gateway" ]]; then
    echo "ERROR: performance-test-scripts supports -g api-gateway only" >&2
    exit 1
fi
PERF_HOME="${PERF_HOME:-${HOME}/perf-api-gateway-manual}"
export PERF_HOME PERF_ROOT
export PERF_LOCAL=true
export PERF_LOCAL_SCRIPTS="${SCRIPT_DIR}"
export SKIP_JTL_SPLIT="${SKIP_JTL_SPLIT:-true}"
export SKIP_PAYLOAD_GENERATION="${SKIP_PAYLOAD_GENERATION:-true}"
export AI_CHAT_COMPLETION_MIN_RESPONSE_BYTES=279

: "${GATEWAY_HOST:?Set GATEWAY_HOST in env.jmeter}"
: "${BACKEND_HOST:?Set BACKEND_HOST in env.jmeter}"

# api-gateway EC2 perf: docker Netty on :8688; jar backend on host uses :3000
if [[ "${BACKEND_IMPL:-jar}" == "docker" || "${BACKEND_IMPL:-jar}" == "compose" ]]; then
    export BACKEND_PORT="${BACKEND_PORT:-8688}"
else
    export BACKEND_PORT="${BACKEND_PORT:-3000}"
fi

echo "==> ${PERF_GATEWAY_TYPE}: JMeter -> ${GATEWAY_HOST}:8080 (backend ${BACKEND_HOST}:${BACKEND_PORT}, impl=${BACKEND_IMPL:-jar})"

mkdir -p "$PERF_HOME"
[[ -d "${PERF_HOME}/jmeter" ]] || "${SCRIPT_DIR}/setup-jmeter.sh"

cd "${PERF_HOME}/jmeter"
common_dir="${PERF_ROOT}/performance-common/distribution/scripts/common"
perf_test_common="${PERF_ROOT}/performance-common/distribution/scripts/jmeter/perf-test-common.sh"

api_keys_file_name="api-keys.csv"

_parse_jmeter_users() {
    local prev="" u=""
    while [[ $# -gt 0 ]]; do
        if [[ "$prev" == "-u" ]]; then
            echo "$1"
            return 0
        fi
        prev="$1"
        shift
    done
    return 0
}

_jmeter_users="$(_parse_jmeter_users "$@")"
export API_KEY_START="${API_KEY_START:-1}"
# API_KEY_COUNT comes from env.jmeter (pregenerated once on gateway). Not overridden per run.

function start_netty_backend_local() {
    local _heap="$1"
    local sleep_time="$2"
    local backend_flags="$3"
    local response_size="${rsize:-10240}"
    # EKS / in-cluster mock: gateway reaches ClusterIP; no SSH to a backend EC2.
    if [[ "${BACKEND_IN_CLUSTER:-}" == "1" || -z "${BACKEND_SSH:-}" ]]; then
        echo "Skipping remote backend start (BACKEND_IN_CLUSTER=${BACKEND_IN_CLUSTER:-0}, BACKEND_SSH=${BACKEND_SSH:-<empty>})"
        return 0
    fi
    local ssh_opts=(-o StrictHostKeyChecking=no -o ConnectTimeout=10)
    [[ -f "${BACKEND_SSH_KEY}" ]] && ssh_opts+=(-i "${BACKEND_SSH_KEY}")
    if [[ "${BACKEND_IMPL:-jar}" == "compose" || "${BACKEND_IMPL:-jar}" == "docker" ]]; then
        echo "Ensuring container backend is up (impl=${BACKEND_IMPL:-jar}, port=${BACKEND_PORT}) — skip jar reconfigure"
        ssh "${ssh_opts[@]}" "${BACKEND_SSH}" \
            "cd ${PERF_ROOT}/ai-gateway-manual/backend && \
             [[ -f env.backend ]] && source env.backend; \
             export BACKEND_IMPL='${BACKEND_IMPL:-jar}' BACKEND_PORT='${BACKEND_PORT}' NETTY_PORT='${BACKEND_PORT}'; \
             ./start-backend.sh" \
            || echo "WARNING: backend container start failed"
        return 0
    fi
    echo "Reconfiguring backend: delay=${sleep_time}ms response_size=${response_size} impl=${BACKEND_IMPL:-jar} port=${BACKEND_PORT}"
    ssh "${ssh_opts[@]}" "${BACKEND_SSH}" \
        "cd ${PERF_ROOT}/ai-gateway-manual/backend && \
         [[ -f env.backend ]] && source env.backend; \
         export BACKEND_IMPL='${BACKEND_IMPL:-jar}' BACKEND_PORT='${BACKEND_PORT}' NETTY_PORT='${BACKEND_PORT}' \
         MOCK_BACKEND_DELAY='${sleep_time}' MOCK_RESPONSE_SIZE='${response_size}' NETTY_HEAP='${NETTY_HEAP:-4g}'; \
         ./reconfigure-backend.sh -d ${sleep_time} -r ${response_size}" \
        || echo "WARNING: backend SSH reconfigure failed — ensure backend/reconfigure-backend.sh is on backend EC2"
}
export -f start_netty_backend_local

. "${common_dir}/common.sh"
. "${perf_test_common}"

# High-TPS runs: skip per-cell GC logs (saves GiB when disk is tight). Set SKIP_JMETER_GC_LOG=0 to enable.
if [[ "${SKIP_JMETER_GC_LOG:-true}" == "true" ]]; then
    function jmeter_gc_log_args() { :; }
fi

function collect_server_metrics() { return 0; }
function write_server_metrics() { return 0; }
function download_file() { return 0; }
export -f collect_server_metrics write_server_metrics download_file

# shellcheck source=generate-jwt-tokens.sh
source "${SCRIPT_DIR}/generate-jwt-tokens.sh"

function initialize() {
    if [[ "${JWT_SINGLE_TOKEN:-}" == "1" ]]; then
        export JWT_TOKEN_COUNT=1
        generate_jwt_tokens || exit 1
    elif [[ ! -s "${HOME}/jwt-tokens.csv" || "${JWT_REGENERATE:-}" == "1" ]]; then
        generate_jwt_tokens || exit 1
    fi

    [[ "${jmeter_servers:-1}" -le 1 ]] && return 0

    local key_files=()
    local f
    for f in "${HOME}"/jwt-tokens*.csv; do
        [[ -f "$f" ]] && key_files+=("$f")
    done
    if [[ ${#key_files[@]} -eq 0 ]]; then
        echo "ERROR: No jwt-tokens*.csv in ${HOME}. Set JWT_OAUTH_* or run generate-jwt-tokens.sh" >&2
        exit 1
    fi

    local host
    for host in "${jmeter_ssh_hosts[@]}"; do
        echo "Copying JWT tokens to ${host}..."
        scp -o StrictHostKeyChecking=no "${key_files[@]}" "${host}:${HOME}/"
    done
}
export -f initialize

script_dir="$(pwd -P)"

# shellcheck source=gateway-scenarios.sh
source "${MANUAL_DIR}/jmeter/gateway-scenarios.sh"

function before_execute_test_scenario() {
    local service_host="${GATEWAY_HOST}"
    local response_size=${rsize:-10240}
    local keys_file="${HOME}/${api_keys_file_name}"
    local request_body_mode=""
    local request_body=""

    if [[ ${scenario[host_type]} == "backend" ]]; then
        service_host="${BACKEND_HOST}"
    elif [[ -n "${scenario_api_key_file[${scenario[name]}]:-}" ]]; then
        keys_file="${HOME}/${scenario_api_key_file[${scenario[name]}]}"
        [[ -f "$keys_file" ]] || keys_file="${PERF_HOME}/${scenario_api_key_file[${scenario[name]}]}"
        [[ -f "$keys_file" ]] || {
        echo "Missing ${keys_file}. Regenerate JWT tokens (JWT_REGENERATE=1)." >&2
            exit 1
        }
        local key_lines
        key_lines=$(wc -l <"$keys_file" | tr -d ' ')
        if [[ -n "${users:-}" && "${users}" =~ ^[0-9]+$ && "${key_lines}" -lt "${users}" ]]; then
            echo "WARNING: ${keys_file} has ${key_lines} keys but test uses ${users} users (keys will recycle)."
            echo "         Pre-generate more: ./pregenerate-api-keys.sh ${users}  (one-time on gateway + fetch)"
        fi
        if [[ "${scenario_auth_header[${scenario[name]}]:-}" == "Authorization" && ! -s "$keys_file" ]]; then
            echo "ERROR: ${keys_file} is empty. Regenerate with JWT_REGENERATE=1 ./run-scenario.sh ..."
            exit 1
        fi
    fi

    jmeter_params+=("host=$service_host" "port=${scenario[port]}" "path=${scenario[path]}")
    if [[ -n "${GATEWAY_DUAL_PORTS:-}" ]]; then
        jmeter_params+=("dualGateway=1")
        jmeter_params+=("dualGatewayHosts=$(IFS=,; echo "${jmeter_hosts[*]}")")
        jmeter_params+=("dualGatewayPorts=${GATEWAY_DUAL_PORTS}")
    fi
    jmeter_params+=("protocol=${scenario[protocol]}")
    jmeter_params+=("resourceSuffixes=${scenario[resource_suffixes]:-}")
    jmeter_params+=("method=${scenario[method]:-POST}")

    request_body_mode="${scenario[request_body_mode]:-}"
    if [[ "${scenario[method]:-GET}" == "POST" ]]; then
        request_body="${scenario[request_body]:-{\"messages\":[{\"role\":\"user\",\"content\":\"perf-post\"}]}}"
        jmeter_params+=("requestBody=${request_body}")
    fi
    if [[ ${scenario[host_type]} == "gateway" ]]; then
        jmeter_params+=("response_size=${response_size}")
        if [[ -n "${scenario_api_key_file[${scenario[name]}]:-}" ]]; then
            jmeter_params+=("tokens=${keys_file}")
            jmeter_params+=("authHeaderName=${scenario_auth_header[${scenario[name]}]:-X-API-Key}")
        fi
    fi

  if [[ -n "${rsize}" && "${rsize}" -lt "${AI_CHAT_COMPLETION_MIN_RESPONSE_BYTES}" ]]; then
        echo "==> Response ${response_size}B < ${AI_CHAT_COMPLETION_MIN_RESPONSE_BYTES}B — Netty echo mode (tiny GET body, weather-like)"
        scenario[backend_flags]="--port ${BACKEND_PORT}"
    else
        scenario[backend_flags]="--port ${BACKEND_PORT} --ai-chat-completion-response --ai-chat-completion-response-size ${response_size}"
    fi
}

function after_execute_test_scenario() { return 0; }

print_load_plan() {
    local users="" servers=1
    while [[ $# -gt 0 ]]; do
        case "$1" in
        -u) users="$2"; shift 2 ;;
        -n) servers="$2"; shift 2 ;;
        *) shift ;;
        esac
    done
    [[ -z "$users" || "$servers" -le 1 ]] && return 0
    if ! [[ "$users" =~ ^[0-9]+$ && "$servers" =~ ^[0-9]+$ ]]; then
        return 0
    fi
    local per=$((users / servers)) rem=$((users % servers))
    if [[ "$rem" -ne 0 ]]; then
        echo "ERROR: -u ${users} must divide evenly by -n ${servers}" >&2
        exit 1
    fi
    echo ""
    echo "==> Gateway type: ${PERF_GATEWAY_TYPE}"
    if [[ -n "${GATEWAY_DUAL_PORTS:-}" ]]; then
        IFS=',' read -ra _dual_ports <<< "${GATEWAY_DUAL_PORTS}"
        local n_ports=${#_dual_ports[@]}
        local ix _port _base _end
        if [[ "$servers" -ge "$n_ports" ]]; then
            for ix in $(seq 0 $((servers - 1))); do
                _port="${_dual_ports[$ix]:-${_dual_ports[0]}}"
                echo "==> Engine ${ix}: ${GATEWAY_HOST}:${_port} (${per} threads)"
            done
            echo "==> Scaled gateway: ${users} TOTAL users => ${per} threads × ${servers} engines (one port per engine)"
        else
            local _ppe=$(( (n_ports + servers - 1) / servers ))
            for ix in $(seq 0 $((servers - 1))); do
                _base=$((ix * _ppe))
                _end=$((_base + _ppe - 1))
                [[ "$_end" -ge "$n_ports" ]] && _end=$((n_ports - 1))
                if [[ "$_base" -lt "$n_ports" ]]; then
                    echo "==> Engine ${ix}: ${GATEWAY_HOST}:${_dual_ports[_base]}–${_dual_ports[_end]} (${per} threads, round-robin)"
                fi
            done
            echo "==> Scaled gateway: ${users} TOTAL users => ${per} threads × ${servers} engines (${n_ports} ports total)"
        fi
        echo "    Single JMeter client on jmeter1; combined summary on this host"
    else
        echo "==> Gateway target: ${GATEWAY_HOST}:8080"
        echo "==> Load plan: ${users} TOTAL users => ${per} threads × ${servers} engines"
    fi
    echo "    Gateway sees HTTP from ${servers} JMeter host IPs (not ${users}×${servers})"
    if [[ "${PERF_GATEWAY_TYPE}" == "api-gateway" && "${users}" -gt 1000 ]]; then
        echo "WARNING: api-gateway plain baseline is known to plateau around ~20-25k TPS at 1000 users."
        echo "         Higher users can trigger ext_proc overflow (HTTP 500 / JMeter code 1000)."
    fi
    echo ""
}

print_load_plan "$@"

prune_old_result_backups() {
    local keep="${1:-2}"
    local d
    for d in $(ls -dt results.backup-* 2>/dev/null | tail -n +$((keep + 1))); do
        echo "==> Removing old backup: ${d}"
        rm -rf "${d}"
    done
    for d in $(ls -t results.zip.backup-* 2>/dev/null | tail -n +$((keep + 1))); do
        echo "==> Removing old backup: ${d}"
        rm -f "${d}"
    done
}

preflight_jmeter_disk() {
    local avail_kb min_free_kb=2097152
    avail_kb=$(df "${PERF_HOME}" 2>/dev/null | awk 'NR==2 {print $4}')
    if [[ -z "${avail_kb}" ]]; then
        avail_kb=$(df / | awk 'NR==2 {print $4}')
    fi
    echo "==> Disk free: $(( avail_kb / 1024 )) MiB on $(df / | awk 'NR==2 {print $1}')"
    if [[ -d "${PERF_HOME}/jmeter/results" ]]; then
        echo "==> Current results/: $(du -sh "${PERF_HOME}/jmeter/results" 2>/dev/null | awk '{print $1}')"
    fi
    prune_old_result_backups 1
    avail_kb=$(df / | awk 'NR==2 {print $4}')
    if [[ "${avail_kb}" -lt "${min_free_kb}" ]]; then
        echo "WARNING: < 2 GiB free — deleting results/ and old backups (no archive)"
        rm -rf "${PERF_HOME}/jmeter/results" "${PERF_HOME}/jmeter/results.backup-"* 2>/dev/null || true
        rm -f "${PERF_HOME}/jmeter/results.zip" "${PERF_HOME}/jmeter/results.zip.backup-"* 2>/dev/null || true
        avail_kb=$(df / | awk 'NR==2 {print $4}')
    fi
    if [[ "${avail_kb}" -lt 524288 ]]; then
        echo "ERROR: < 512 MiB free on JMeter-1. Run: cd ${SCRIPT_DIR} && ./recover-stuck-test.sh --aggressive" >&2
        exit 1
    fi
    if [[ "${avail_kb}" -lt 1048576 ]]; then
        echo "WARNING: < 1 GiB free after cleanup — long runs may fail; consider ./recover-stuck-test.sh --aggressive or expand EBS"
    fi
}

backup_previous_results() {
    local stamp
    stamp=$(date +%Y%m%d-%H%M%S)
    local avail_kb
    avail_kb=$(df / | awk 'NR==2 {print $4}')
    if [[ -d results ]]; then
        if [[ "${avail_kb}" -lt 2097152 ]]; then
            echo "==> Disk low (< 2 GiB) — deleting results/ instead of backup"
            rm -rf results
        else
            echo "Backing up existing results/ -> results.backup-${stamp}"
            mv results "results.backup-${stamp}"
        fi
    fi
    if [[ -f results.zip ]]; then
        if [[ "${avail_kb}" -lt 2097152 ]]; then
            rm -f results.zip
        else
            echo "Backing up existing results.zip -> results.zip.backup-${stamp}"
            mv results.zip "results.zip.backup-${stamp}"
        fi
    fi
    rm -f test-duration.json test-metadata.json
}

preflight_jmeter_disk
backup_previous_results

test_scenarios
