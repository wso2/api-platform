#!/usr/bin/env bash
# run.sh --versions 1.0.0,1.1.0,1.2.0  (any published gateway releases, at most 4) --scenarios rest-api-basic,llm-provider-secret [--keep] [--reuse]
# One-shot: boot platform-api + request-info, start the gateways, run each scenario against every
# version, print the report, tear down (unless --keep). A scenario name may also be a path to a
# custom scenario script. --reuse keeps the existing platform-api DB and running services.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"; . "$HERE/lib.sh"
VERSIONS="1.0.0,1.1.0,1.2.0"; SCENARIOS="rest-api-basic,llm-provider-secret,mcp-proxy-upstream-path,kind-gating,llm-policies-flatten,mcp-spec-versions-fold,upstream-auth-types,llm-proxy-additional-providers,secret-rotation"; KEEP=false; REUSE=false
while [ $# -gt 0 ]; do
  case "$1" in
    --versions) VERSIONS="$2"; shift 2 ;; --scenarios) SCENARIOS="$2"; shift 2 ;;
    --keep) KEEP=true; shift ;; --reuse) REUSE=true; shift ;;
    *) die "unknown argument $1" ;;
  esac
done
VLIST=$(echo "$VERSIONS" | tr ',' ' ')

if [ "$REUSE" = true ] && curl -sf "$PAPI_URL/health" >/dev/null 2>&1; then
  log "reusing running platform-api"
else
  "$HERE/start-platform-api.sh" || exit 1
fi
# start-platform-api.sh created a fresh work dir; pick it up.
WORK=$(cat "$WORK_POINTER"); LOGS="$WORK/logs"; STATE="$WORK/state.env"
"$HERE/bootstrap.sh" || exit 1
"$HERE/start-backend.sh" || exit 1
for v in $VLIST; do "$HERE/start-gateway.sh" "$v" || exit 1; done

: > "$WORK/results.tsv"
for s in $(echo "$SCENARIOS" | tr ',' ' '); do
  f="$s"; [ -f "$f" ] || f="$HERE/../scenarios/$s.sh"
  [ -f "$f" ] || { log "unknown scenario $s"; continue; }
  log "=== scenario $(basename "$f" .sh)"
  bash "$f" $VLIST || true
done

"$HERE/report.sh"
if [ "$KEEP" = true ]; then
  log "stack kept running; tear down with: $HERE/teardown.sh"
else
  "$HERE/teardown.sh"
fi
