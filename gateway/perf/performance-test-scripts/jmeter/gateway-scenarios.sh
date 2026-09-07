# API Gateway EKS perf — three baseline scenarios only.
# Must match deploy-apis-eks-minimal.sh (api-plain, api-header, api-jwt).
# Default 8 routes: base + /r1-/r7 (PERF_API_ROUTE_COUNT must match deploy -r).

function _perf_api_route_suffixes() {
    local routes="${1:-${PERF_API_ROUTE_COUNT:-8}}"
    local i
    local s=""
    for ((i = 1; i < routes; i++)); do
        s="${s},/r${i}"
    done
    echo "${s}"
}

function load_perf_gateway_scenarios() {
    unset test_scenario0 test_scenario1 test_scenario2 test_scenario 2>/dev/null || true
    unset scenario_api_key_file scenario_auth_header 2>/dev/null || true
    declare -gA scenario_api_key_file
    declare -gA scenario_auth_header

    if [[ "${PERF_GATEWAY_TYPE:-api-gateway}" != "api-gateway" ]]; then
        echo "ERROR: performance-test-scripts supports api-gateway only (got: ${PERF_GATEWAY_TYPE})" >&2
        exit 1
    fi

    local backend_port="${BACKEND_PORT:-8688}"
    local backend_flags="--port ${backend_port}"

    scenario_api_key_file=(
        [api_api_jwt_get]="jwt-tokens.csv"
    )
    scenario_auth_header=(
        [api_api_jwt_get]="Authorization"
    )

    declare -gA test_scenario0=(
        [name]="api_api_plain_get"
        [display_name]="API Gateway Plain GET"
        [description]="RestApi plain GET — no policies (api-plain)."
        [jmx]="api-api-test-gateway-plain.jmx"
        [protocol]="http"
        [path]="/api-plain/1.0.0/chat/completions"
        [resource_suffixes]="$(_perf_api_route_suffixes "${PERF_API_ROUTE_COUNT:-8}")"
        [host_type]="gateway"
        [port]="8080"
        [use_apim]=false
        [use_backend]=true
        [backend_flags]="${backend_flags}"
        [skip]=false
        [method]="GET"
    )
    declare -gA test_scenario1=(
        [name]="api_api_header_get"
        [display_name]="API Gateway Header Policy GET"
        [description]="set-headers policy GET (api-header)."
        [jmx]="api-api-test-gateway-plain.jmx"
        [protocol]="http"
        [path]="/api-header/1.0.0/chat/completions"
        [resource_suffixes]="$(_perf_api_route_suffixes "${PERF_API_ROUTE_COUNT:-8}")"
        [host_type]="gateway"
        [port]="8080"
        [use_apim]=false
        [use_backend]=true
        [backend_flags]="${backend_flags}"
        [skip]=false
        [method]="GET"
    )
    declare -gA test_scenario2=(
        [name]="api_api_jwt_get"
        [display_name]="API Gateway JWT GET"
        [description]="jwt-auth policy GET (api-jwt)."
        [jmx]="api-api-test-gateway-jwt-plain.jmx"
        [protocol]="http"
        [path]="/api-jwt/1.0.0/chat/completions"
        [resource_suffixes]="$(_perf_api_route_suffixes "${PERF_API_ROUTE_COUNT:-8}")"
        [host_type]="gateway"
        [port]="8080"
        [use_apim]=false
        [use_backend]=true
        [backend_flags]="${backend_flags}"
        [skip]=false
        [method]="GET"
    )

    declare -gA test_scenario=(
        [test_scenario0]=1
        [test_scenario1]=1
        [test_scenario2]=1
    )
}

load_perf_gateway_scenarios
