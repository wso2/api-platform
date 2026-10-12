#!/usr/bin/env bash
# invoke.sh <version> <METHOD> <path> [json-body] [extra curl args...]
# Calls the gateway router of <version> with Host: localhost (the vhost the gateways are
# registered with). Prints the response body, then "HTTP <code>". request-info echoes the request
# it received as {"Request":{"Method","Header","URL":{"Path","RawQuery"},"Body",...}}.
set -euo pipefail
. "$(dirname "$0")/lib.sh"
need_work
VER="${1:?version}"; M="${2:?method}"; P="${3:?path}"; D="${4:-}"; shift 3; [ $# -gt 0 ] && shift
args=(-sS -X "$M" "$(gw_router "$VER")$P" -H 'Host: localhost' -w '\nHTTP %{http_code}\n' --max-time 20)
[ -n "$D" ] && args+=(-H 'Content-Type: application/json' --data "$D")
# A route can lag the DEPLOYED ack by a second or two: retry 404s for up to INVOKE_WAIT seconds.
i=0
while :; do
  out=$(curl "${args[@]}" "$@" || true)
  code=$(printf '%s\n' "$out" | tail -1 | awk '{print $2}')
  [ "$code" = 404 ] && [ "$i" -lt "${INVOKE_WAIT:-15}" ] || break
  sleep 1; i=$((i+1))
done
printf '%s\n' "$out"
