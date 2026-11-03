#!/usr/bin/env bash
# Scenario rest-api-basic: a REST API proxies to request-info with its context stripped and the
# query string kept.  Usage: rest-api-basic.sh <version>...
set -uo pipefail
S="$(cd "$(dirname "$0")/../scripts" && pwd)"; . "$S/lib.sh"
N=rest-api-basic
ensure_created rest-apis "{\"id\":\"e2e-rest\",\"displayName\":\"e2e-rest\",\"context\":\"/e2e\",\"version\":\"v1\",\"projectId\":\"default\",\"upstream\":{\"main\":{\"url\":\"$BACKEND_URL_FROM_GW/rest\"}}}"
for v in "$@"; do
  "$S/deploy.sh" rest-apis e2e-rest "$v" >/dev/null || { check $N "$v" deploy DEPLOYED FAILED; continue; }
  out=$(invoke_until 20 URL.Path /rest/hello "$v" GET '/e2e/hello?x=1')
  check $N "$v" http-status 200 "$(printf '%s\n' "$out" | http_code)"
  check $N "$v" upstream-path /rest/hello "$(printf '%s\n' "$out" | echo_get URL.Path)"
  check $N "$v" query x=1 "$(printf '%s\n' "$out" | echo_get URL.RawQuery)"
done
