#!/bin/bash -e
# Pre-generate Bearer tokens for api_api_jwt_get (Asgardeo OAuth on Jenkins/EKS).
#   export JWT_OAUTH_TOKEN_URL='https://api.asgardeo.io/t/<client>/oauth2/token'
#   export JWT_OAUTH_CLIENT_ID='...'
#   export JWT_OAUTH_CLIENT_SECRET='...'
#   JWT_REGENERATE=1 ./generate-jwt-tokens.sh
#
# Single token (JWKS/JWT cache test):
#   JWT_SINGLE_TOKEN=1 ./generate-jwt-tokens.sh

set -euo pipefail

_out="${JWT_TOKENS_FILE:-${HOME}/jwt-tokens.csv}"
_jwt_count="${JWT_TOKEN_COUNT:-500}"
if [[ "${JWT_SINGLE_TOKEN:-}" == "1" ]]; then
    _jwt_count=1
fi

_fetch_oauth_token() {
    local token_url="${JWT_OAUTH_TOKEN_URL:?Set JWT_OAUTH_TOKEN_URL}"
    local client_id="${JWT_OAUTH_CLIENT_ID:?Set JWT_OAUTH_CLIENT_ID}"
    local client_secret="${JWT_OAUTH_CLIENT_SECRET:?Set JWT_OAUTH_CLIENT_SECRET}"
    curl -sf --connect-timeout 15 --location "${token_url}" \
        --header 'Content-Type: application/x-www-form-urlencoded' \
        --data-urlencode 'grant_type=client_credentials' \
        --data-urlencode "client_id=${client_id}" \
        --data-urlencode "client_secret=${client_secret}" \
        | python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])'
}

_fetch_mock_token() {
    local token_url="$1"
    curl -sf --connect-timeout 10 "${token_url}"
}

_fetch_mock_tokens_remote() {
    local jwt_count="$1"
    local token_url="$2"
    local out="$3"
    local gw_user="${GATEWAY_SCP_USER:-${GATEWAY_SSH%%@*}}"
    gw_user="${gw_user:-ec2-user}"
    local gw_host="${GATEWAY_PRIVATE_IP:-${GATEWAY_HOST:?Set GATEWAY_HOST or GATEWAY_PRIVATE_IP}}"
    local ssh_opts=(-o StrictHostKeyChecking=no -o ConnectTimeout=10)

    if [[ -n "${GATEWAY_SSH_KEY:-}" ]]; then
        [[ -f "${GATEWAY_SSH_KEY}" ]] || {
            echo "ERROR: GATEWAY_SSH_KEY not found: ${GATEWAY_SSH_KEY}" >&2
            return 1
        }
        ssh_opts+=(-i "${GATEWAY_SSH_KEY}")
    fi

    if ! ssh "${ssh_opts[@]}" "${gw_user}@${gw_host}" \
        "curl -sf 'http://127.0.0.1:8088/jwks' >/dev/null || (cd ~/perf/ai-gateway-manual/api-gateway && ./start-mock-jwks.sh)" \
        2>/dev/null; then
        echo "ERROR: Could not reach gateway or start mock-jwks (${gw_user}@${gw_host})" >&2
        return 1
    fi

    : >"${out}"
    if [[ "${jwt_count}" -eq 1 ]]; then
        token=$(ssh "${ssh_opts[@]}" "${gw_user}@${gw_host}" "curl -sf '${token_url}'") || return 1
        echo "Bearer ${token}" >>"${out}"
        return 0
    fi

    echo "==> Fetching ${jwt_count} mock tokens via SSH ..."
    ssh "${ssh_opts[@]}" "${gw_user}@${gw_host}" bash -s -- "${jwt_count}" "${token_url}" <<'REMOTE' >"${out}"
set -euo pipefail
count=$1
url=$2
for ((i = 1; i <= count; i++)); do
    token=$(curl -sf "${url}")
    printf 'Bearer %s\n' "${token}"
done
REMOTE
}

generate_jwt_tokens() {
    local out="$_out"
    local jwt_count="$_jwt_count"
    local i token

    : >"${out}"

    if [[ -n "${JWT_OAUTH_TOKEN_URL:-}" ]]; then
        if [[ "${jwt_count}" -eq 1 ]]; then
            echo "Generating 1 OAuth token at ${out} (all threads reuse it) ..."
        else
            echo "Generating ${jwt_count} OAuth tokens at ${out} (${JWT_OAUTH_TOKEN_URL}) ..."
        fi
        for ((i = 1; i <= jwt_count; i++)); do
            token=$(_fetch_oauth_token) || break
            echo "Bearer ${token}" >>"${out}"
        done
    else
        local issuer="http://172.17.0.1:8088/token"
        local token_url="http://127.0.0.1:8088/token?issuer=${issuer}"
        if [[ "${jwt_count}" -eq 1 ]]; then
            echo "Generating 1 mock JWT at ${out} (all threads reuse it) ..."
        else
            echo "Generating ${jwt_count} mock JWT tokens at ${out} (issuer=${issuer}) ..."
        fi
        if [[ -n "${GATEWAY_HOST:-}" ]] && curl -sf --connect-timeout 3 \
            "http://${GATEWAY_HOST}:8088/token?issuer=${issuer}" >/dev/null 2>&1; then
            echo "==> Fetching mock tokens via HTTP ${GATEWAY_HOST}:8088"
            token_url="http://${GATEWAY_HOST}:8088/token?issuer=${issuer}"
            for ((i = 1; i <= jwt_count; i++)); do
                token=$(_fetch_mock_token "${token_url}") || break
                echo "Bearer ${token}" >>"${out}"
            done
        else
            echo "==> Port 8088 not reachable — fetching mock tokens via SSH"
            _fetch_mock_tokens_remote "${jwt_count}" "${token_url}" "${out}" || true
        fi
    fi

    local lines
    lines=$(wc -l <"${out}" | tr -d ' ')
    if [[ "${lines}" -lt 1 ]]; then
        echo "ERROR: Failed to generate JWT tokens." >&2
        if [[ -n "${JWT_OAUTH_TOKEN_URL:-}" ]]; then
            echo "  Check JWT_OAUTH_TOKEN_URL, JWT_OAUTH_CLIENT_ID, JWT_OAUTH_CLIENT_SECRET" >&2
        else
            echo "  Set JWT_OAUTH_TOKEN_URL, JWT_OAUTH_CLIENT_ID, JWT_OAUTH_CLIENT_SECRET (Asgardeo OAuth)." >&2
        fi
        return 1
    fi
    echo "==> Wrote ${lines} JWT tokens to ${out}"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    generate_jwt_tokens
fi
