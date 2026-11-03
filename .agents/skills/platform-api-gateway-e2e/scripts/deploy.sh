#!/usr/bin/env bash
# deploy.sh <collection> <handle> <version>
#   collection: rest-apis | llm-providers | llm-proxies | mcp-proxies | agent-proxies (any
#   platform-api collection with a /{id}/deployments sub-resource)
# Deploys the artifact's current definition to the gateway of <version> and polls until the
# gateway acks. Prints "deploymentId status" on stdout. Exit 1 on FAILED/timeout, 2 when
# platform-api refuses the deploy (the error body is printed to stderr).
set -euo pipefail
. "$(dirname "$0")/lib.sh"
need_work
COLL="${1:?collection}"; ID="${2:?handle}"; VER="${3:?version}"; GW=$(gw_handle "$VER")

out=$(api POST "/$COLL/$ID/deployments" "{\"name\":\"dep-$(date +%s)-$RANDOM\",\"base\":\"current\",\"gatewayId\":\"$GW\"}")
code=$(printf '%s\n' "$out" | http_code)
if [ "$code" != 201 ]; then
  printf '%s\n' "$out" | http_body >&2
  log "deploy $COLL/$ID -> $GW refused (HTTP $code)"; exit 2
fi
DEP=$(printf '%s\n' "$out" | http_body | json_get deploymentId)

status=""
for i in $(seq 1 75); do
  status=$(api GET "/$COLL/$ID/deployments/$DEP" | http_body | json_get status)
  case "$status" in DEPLOYED|FAILED) break ;; esac
  sleep 1
done
echo "$DEP $status"
log "deploy $COLL/$ID -> $GW: $status ($DEP)"
[ "$status" = DEPLOYED ] || {
  api GET "/$COLL/$ID/deployments/$DEP" | http_body >&2
  exit 1; }
