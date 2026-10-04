#!/usr/bin/env bash
#
# Mints an access token for the sample, signed with the key pair in keys/
# that setup.sh generated.
#
#   ./token.sh                      token for the default subject
#   ./token.sh alice@example.com    token for a named subject
#
# Other scripts reuse it by sourcing this file and calling mint_token:
#
#   source ./token.sh
#   TOKEN=$(mint_token alice@example.com)
#
# A real deployment would get tokens from an identity provider. The sample
# signs its own so it needs no extra service.

TOKEN_ISSUER="${TOKEN_ISSUER:-https://mcp-observability-sample.local}"
TOKEN_AUDIENCE="${TOKEN_AUDIENCE:-mcp-observability-sample}"
TOKEN_TTL="${TOKEN_TTL:-3600}"

TOKEN_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOKEN_KEY="${TOKEN_KEY:-${TOKEN_DIR}/keys/private.pem}"

# Base64 as used in tokens: the URL-safe alphabet, with padding removed.
token_b64url() { openssl base64 -A | tr '+/' '-_' | tr -d '='; }

# mint_token <subject>
mint_token() {
  local subject="${1:?a subject is required}" now exp header payload signing signature

  if [[ ! -f "${TOKEN_KEY}" ]]; then
    echo "No signing key at ${TOKEN_KEY}. Run ./setup.sh first." >&2
    return 1
  fi

  now=$(date +%s)
  exp=$(( now + TOKEN_TTL ))

  header=$(printf '{"alg":"RS256","typ":"JWT","kid":"sample-key"}' | token_b64url)
  payload=$(printf '{"iss":"%s","sub":"%s","aud":"%s","iat":%s,"exp":%s}' \
    "${TOKEN_ISSUER}" "${subject}" "${TOKEN_AUDIENCE}" "${now}" "${exp}" | token_b64url)

  signing="${header}.${payload}"
  signature=$(printf '%s' "${signing}" | openssl dgst -sha256 -sign "${TOKEN_KEY}" | token_b64url)

  echo "${signing}.${signature}"
}

# Run directly rather than sourced.
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  set -euo pipefail
  mint_token "${1:-alice@example.com}"
fi
