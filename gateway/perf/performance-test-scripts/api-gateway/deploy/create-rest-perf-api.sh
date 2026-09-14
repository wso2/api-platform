#!/bin/bash -e
# Deploy RestApi for API Gateway EKS perf (plain, set-headers, jwt-auth only).

script_dir=$(dirname "$0")
controller_host="${GATEWAY_CONTROLLER_HOST:-127.0.0.1}"
controller_port="${GATEWAY_CONTROLLER_PORT:-9090}"
mgmt_user="${GATEWAY_MGMT_USER:-admin}"
mgmt_pass="${GATEWAY_MGMT_PASS:-admin}"
mgmt_base="${GATEWAY_MGMT_API_BASE:-/api/management/v1}"
upstream_url="${MOCK_BACKEND_URL:-http://127.0.0.1:${BACKEND_PORT:-8688}/v1}"
api_name=""
api_mode="plain"
api_version="1.0.0"
route_count="8"
chat_path="/chat/completions"
enable_ratelimit="0"
operations_mode="both"
ratelimit_requests="${RATELIMIT_REQUESTS:-1000000000}"
ratelimit_duration="${RATELIMIT_DURATION:-24h}"

function usage() {
    echo ""
    echo "Usage: $0 -n <api_name> [-m <api_mode>] [-r <route_count>] [-l] [-o get|both] [-h]"
    echo ""
    echo "-n: API context prefix (e.g. api-plain). Gateway path: /<name>/<version>${chat_path}"
    echo "-m: plain | add_headers | jwt_auth"
    echo "-r: Route count (default 8): 1 = base only; 8 = base + /r1-/r7"
    echo "-l: Attach basic-ratelimit v1 (high quota for perf)"
    echo "-o: get (GET only) | both (GET+POST, default)"
    echo ""
}

while getopts "n:m:r:lo:h" opt; do
    case "${opt}" in
    n) api_name=${OPTARG} ;;
    m) api_mode=${OPTARG} ;;
    r) route_count=${OPTARG} ;;
    l) enable_ratelimit="1" ;;
    o) operations_mode=${OPTARG} ;;
    h) usage; exit 0 ;;
    \?) usage; exit 1 ;;
    esac
done

if [[ ! "$route_count" =~ ^[0-9]+$ ]] || [[ "$route_count" -lt 1 ]]; then
    echo "Invalid -r ${route_count}. Use a positive integer (e.g. 1, 8)."
    exit 1
fi

resource_suffixes=("")
if [[ "$route_count" -gt 1 ]]; then
    for ((i = 1; i < route_count; i++)); do
        resource_suffixes+=("/r${i}")
    done
fi

if [[ "$operations_mode" != "get" && "$operations_mode" != "both" ]]; then
    echo "Invalid -o ${operations_mode}. Use get or both."
    exit 1
fi

if [[ -z $api_name ]]; then
    echo "Please provide -n <api_name>"
    exit 1
fi

if [[ $api_mode != "plain" && $api_mode != "add_headers" && $api_mode != "jwt_auth" ]]; then
    echo "Invalid -m. Use: plain, add_headers, jwt_auth"
    exit 1
fi

metadata_name="perf-${api_name}"
api_context="/${api_name}/${api_version}"
mgmt_url="http://${controller_host}:${controller_port}${mgmt_base}/rest-apis"
auth="${mgmt_user}:${mgmt_pass}"
yaml_file="/tmp/${metadata_name}-$$.yaml"
curl_timeout_args=(--connect-timeout "${GATEWAY_CURL_CONNECT_TIMEOUT:-5}" --max-time "${GATEWAY_CURL_MAX_TIME:-30}")

cat >"$yaml_file" <<EOF
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: ${metadata_name}
spec:
  displayName: ${metadata_name}
  version: v1.0
  context: ${api_context}
  upstream:
    main:
      url: ${upstream_url}
  policies:
EOF

append_jwt_auth_policy() {
    local km_name="${JWT_KEYMANAGER_NAME:-test}"
    cat >>"$yaml_file" <<EOF
    - name: jwt-auth
      version: v1
      params:
        issuers:
          - ${km_name}
EOF
}

append_set_headers_policy() {
    cat >>"$yaml_file" <<EOF
    - name: set-headers
      version: v1
      params:
        request:
          headers:
            - name: x-perf-mode
              value: ${api_mode}
            - name: x-perf-api
              value: ${api_name}
        response:
          headers:
            - name: x-perf-gateway
              value: api-gateway
EOF
}

append_operation_ratelimit_policies() {
    if [[ "$enable_ratelimit" != "1" ]]; then
        return
    fi
    cat >>"$yaml_file" <<EOF
      policies:
        - name: basic-ratelimit
          version: v1
          params:
            limits:
              - requests: ${ratelimit_requests}
                duration: "${ratelimit_duration}"
EOF
}

case "$api_mode" in
plain) ;;
add_headers) append_set_headers_policy ;;
jwt_auth) append_jwt_auth_policy ;;
esac

cat >>"$yaml_file" <<EOF
  operations:
EOF

for suffix in "${resource_suffixes[@]}"; do
    cat >>"$yaml_file" <<EOF
    - method: GET
      path: ${chat_path}${suffix}
EOF
    append_operation_ratelimit_policies
    if [[ "$operations_mode" == "both" ]]; then
        cat >>"$yaml_file" <<EOF
    - method: POST
      path: ${chat_path}${suffix}
EOF
        append_operation_ratelimit_policies
    fi
done

get_code=$(curl -s "${curl_timeout_args[@]}" -o /dev/null -w "%{http_code}" -u "$auth" "${mgmt_url}/${metadata_name}")
if [[ "$get_code" == "200" ]]; then
    echo "Updating RestApi ${metadata_name} (mode=${api_mode}, routes=${route_count}, context=${api_context})..."
    http_code=$(curl -s "${curl_timeout_args[@]}" -o /tmp/create-rest-api-response.json -w "%{http_code}" \
        -u "$auth" -X PUT "${mgmt_url}/${metadata_name}" -H "Content-Type: application/yaml" --data-binary @"$yaml_file")
else
    echo "Creating RestApi ${metadata_name} (mode=${api_mode}, routes=${route_count}, context=${api_context})..."
    http_code=$(curl -s "${curl_timeout_args[@]}" -o /tmp/create-rest-api-response.json -w "%{http_code}" \
        -u "$auth" -X POST "$mgmt_url" -H "Content-Type: application/yaml" --data-binary @"$yaml_file")
fi

rm -f "$yaml_file"

if [[ "$http_code" != "200" && "$http_code" != "201" && "$http_code" != "202" ]]; then
    echo "Failed to deploy RestApi. HTTP ${http_code}"
    cat /tmp/create-rest-api-response.json
    exit 1
fi

echo "Deployed ${metadata_name} at ${api_context}${chat_path}"
