#!/bin/bash -e
# Deploy only the 3 baseline RestApis on EKS and remove all others.
#
#   perf-api-plain   -> api_api_plain_get
#   perf-api-header  -> api_api_header_get
#   perf-api-jwt     -> api_api_jwt_get
#
# Usage:
#   source env.eks && ./deploy-apis-eks-minimal.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=eks-common.sh
source "${SCRIPT_DIR}/eks-common.sh"
eks_load_env
eks_require_cmd kubectl curl aws python3
eks_kubecontext

cleanup() { eks_stop_port_forward_controller; }
trap cleanup EXIT

keep_apis=(perf-api-plain perf-api-header perf-api-jwt)

echo "==> Port-forward controller :${GATEWAY_CONTROLLER_PORT}"
eks_port_forward_controller

export GATEWAY_CONTROLLER_HOST="127.0.0.1"
export GATEWAY_CONTROLLER_PORT
export GATEWAY_MGMT_USER="${GATEWAY_MGMT_USER:-admin}"
export GATEWAY_MGMT_PASS="${GATEWAY_MGMT_PASS:-admin}"
# After port-forward, eks_wait_controller_ready may have auto-detected the live base.
export GATEWAY_MGMT_API_BASE="${GATEWAY_MGMT_API_BASE:-/api/management/v1}"
export MOCK_BACKEND_URL
export JWT_KEYMANAGER_NAME
export RATELIMIT_REQUESTS
export RATELIMIT_DURATION

echo "==> Management API: ${GATEWAY_MGMT_API_BASE}"
mgmt_url="http://${GATEWAY_CONTROLLER_HOST}:${GATEWAY_CONTROLLER_PORT}${GATEWAY_MGMT_API_BASE}/rest-apis"
auth="${GATEWAY_MGMT_USER}:${GATEWAY_MGMT_PASS}"
DEPLOY_DIR="${API_GATEWAY_DIR}/deploy"

echo "==> Upstream: ${MOCK_BACKEND_URL}"

list_json=$(curl -sf -u "$auth" "$mgmt_url")
api_names=()
while IFS= read -r name; do
    [[ -n "$name" ]] && api_names+=("$name")
done < <(python3 -c "
import json, sys
data = json.load(sys.stdin)
apis = data.get('apis', data.get('items', []))
for api in apis:
    name = api.get('metadata', {}).get('name') or api.get('name')
    if name:
        print(name)
" <<<"$list_json")

api_count=0
for _n in "${api_names[@]-}"; do
    [[ -n "$_n" ]] && api_count=$((api_count + 1))
done
echo "==> Before: ${api_count} RestApi(s)"
deleted=0
for name in "${api_names[@]-}"; do
    [[ -n "$name" ]] || continue
    keep_it=0
    for k in "${keep_apis[@]}"; do
        [[ "$name" == "$k" ]] && keep_it=1 && break
    done
    [[ "$keep_it" -eq 1 ]] && continue
    code=$(curl -s -o /tmp/delete-rest-api-eks.json -w "%{http_code}" \
        -u "$auth" -X DELETE "${mgmt_url}/${name}")
    if [[ "$code" == "200" || "$code" == "204" ]]; then
        echo "    deleted ${name}"
        deleted=$((deleted + 1))
    else
        echo "    FAILED ${name} (HTTP ${code})" >&2
        cat /tmp/delete-rest-api-eks.json >&2
        exit 1
    fi
done
echo "==> Deleted ${deleted} RestApi(s)"

echo "==> Deploy 3 RestApis (8 paths × GET+POST = 16 operations: base + /r1-/r7)"
"${DEPLOY_DIR}/create-rest-perf-api.sh" -n api-plain -m plain -r 8 -o both
"${DEPLOY_DIR}/create-rest-perf-api.sh" -n api-header -m add_headers -r 8 -o both
"${DEPLOY_DIR}/create-rest-perf-api.sh" -n api-jwt -m jwt_auth -r 8 -o both

echo "==> Waiting for routes to propagate..."
sleep 8

remaining=$(curl -sf -u "$auth" "$mgmt_url" | python3 -c "
import json, sys
data = json.load(sys.stdin)
apis = data.get('apis', data.get('items', []))
for api in apis:
    print(api.get('metadata', {}).get('name') or api.get('name'))
")
echo "==> After:"
echo "$remaining" | sed 's/^/    /'

echo "==> Done."
