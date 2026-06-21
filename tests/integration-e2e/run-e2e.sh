#!/usr/bin/env bash
# Combined platform-api + gateway end-to-end scenario.
#
# Phase 1: bring up the DB + control plane (platform-api), then via the
#          platform-api REST API create a project, a REST API (upstream =
#          sample-backend), a gateway and a registration token.
# Phase 2: start the gateway data plane (gateway-controller + gateway-runtime)
#          with that token, deploy the API, and assert a request through the
#          gateway ingress reaches the sample backend.
#
# Usage: ./run-e2e.sh [up|down]   (default: full run + teardown)
set -uo pipefail

cd "$(dirname "$0")"
# DB selection: E2E_DB = postgres (default) | sqlite | sqlserver
DB="${E2E_DB:-postgres}"
case "$DB" in
  postgres)  COMPOSE_FILE=docker-compose.yaml ;;
  sqlite)    COMPOSE_FILE=docker-compose.sqlite.yaml ;;
  sqlserver) COMPOSE_FILE=docker-compose.sqlserver.yaml ;;
  *) echo "unknown E2E_DB=$DB (want postgres|sqlite|sqlserver)"; exit 2 ;;
esac
PROJECT=apip-e2e
COMPOSE="docker compose -p $PROJECT -f $COMPOSE_FILE"
PA=https://localhost:9243
CURL="curl -sk --max-time 20"
echo "E2E database backend: $DB ($COMPOSE_FILE)"

# Services to start in phase 1 (the DB service name differs per backend).
case "$DB" in
  postgres)  PHASE1="postgres platform-api sample-backend" ;;
  sqlserver) PHASE1="sqlserver platform-api sample-backend" ;;
  sqlite)    PHASE1="platform-api sample-backend" ;;
esac

log()  { printf '\n\033[1;34m== %s\033[0m\n' "$*"; }
ok()   { printf '\033[1;32mPASS\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31mFAIL\033[0m %s\n' "$*"; FAILED=1; }
FAILED=0

ING="http://localhost:${GW_HTTP_PORT:-18080}/e2e/"   # gateway 1 ingress
ING2="http://localhost:${GW2_HTTP_PORT:-18081}/e2e/" # gateway 2 ingress (multi-gateway)
ingress_status() { $CURL -H 'Host: localhost' "${1:-$ING}" -o /dev/null -w '%{http_code}' 2>/dev/null; }
# wait_ingress <url> <wanted-status>: poll the ingress until it returns the
# wanted status (up to ~90s); echoes the last observed status.
wait_ingress() {
  local url="$1" want="$2" s=""
  for _ in $(seq 1 45); do s=$(ingress_status "$url"); [ "$s" = "$want" ] && { echo "$s"; return 0; }; sleep 2; done
  echo "$s"; return 1
}
# Multi-gateway is exercised on the postgres stack (the only one wired with a
# second gateway); it is DB-independent, so once is enough.
MULTI=""; [ "$DB" = "postgres" ] && MULTI=1

teardown() { log "Teardown"; GATEWAY_REGISTRATION_TOKEN=x $COMPOSE down -v --remove-orphans >/dev/null 2>&1 || true; }
[ "${1:-run}" = "down" ] && { teardown; exit 0; }
# E2E_KEEP=1 leaves the stack running for inspection.
[ -z "${E2E_KEEP:-}" ] && trap teardown EXIT

# ---------------------------------------------------------------- Phase 1
log "Phase 1: start control plane ($PHASE1)"
GATEWAY_REGISTRATION_TOKEN=placeholder $COMPOSE up -d $PHASE1

log "Wait for platform-api /health"
for i in $(seq 1 120); do
  $CURL "$PA/health" 2>/dev/null | grep -q ok && { ok "platform-api healthy"; break; }
  sleep 2
  [ "$i" = 120 ] && { fail "platform-api never became healthy"; $COMPOSE logs platform-api | tail -30; exit 1; }
done

log "Login (admin/admin)"
TOKEN=$($CURL -X POST "$PA/api/portal/v1/auth/login" -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin"}' | jq -r '.token // empty')
[ -n "$TOKEN" ] && ok "got JWT" || { fail "login failed"; exit 1; }
AUTH=(-H "Authorization: Bearer $TOKEN")

log "Create project"
PROJ=$($CURL -X POST "$PA/api/v1/projects" "${AUTH[@]}" -H 'Content-Type: application/json' \
  -d '{"name":"e2e-proj","description":"e2e"}')
echo "$PROJ" | jq . 2>/dev/null || echo "$PROJ"
PROJECT_ID=$(echo "$PROJ" | jq -r '.id // .uuid // empty')
[ -n "$PROJECT_ID" ] && ok "project=$PROJECT_ID" || { fail "create project"; exit 1; }

log "Create REST API (upstream -> sample-backend)"
API=$($CURL -X POST "$PA/api/v1/rest-apis" "${AUTH[@]}" -H 'Content-Type: application/json' -d "{
  \"name\":\"E2E API\",\"context\":\"/e2e\",\"version\":\"v1\",\"projectId\":\"$PROJECT_ID\",
  \"upstream\":{\"main\":{\"url\":\"http://sample-backend:9080\"}}
}")
echo "$API" | jq . 2>/dev/null || echo "$API"
API_ID=$(echo "$API" | jq -r '.id // .handle // .uuid // empty')
[ -n "$API_ID" ] && ok "api=$API_ID" || { fail "create api"; exit 1; }

log "Create gateway"
GW=$($CURL -X POST "$PA/api/v1/gateways" "${AUTH[@]}" -H 'Content-Type: application/json' -d '{
  "name":"e2e-gw","displayName":"E2E GW","vhost":"localhost","functionalityType":"regular"
}')
echo "$GW" | jq . 2>/dev/null || echo "$GW"
GW_ID=$(echo "$GW" | jq -r '.id // .uuid // empty')
[ -n "$GW_ID" ] && ok "gateway=$GW_ID" || { fail "create gateway"; exit 1; }

log "Rotate gateway token"
TOK=$($CURL -X POST "$PA/api/v1/gateways/$GW_ID/tokens" "${AUTH[@]}" -H 'Content-Type: application/json' -d '{}')
echo "$TOK" | jq 'del(.token)' 2>/dev/null || true
REG_TOKEN=$(echo "$TOK" | jq -r '.token // .apiKey // .value // .registrationToken // empty')
[ -n "$REG_TOKEN" ] && ok "got registration token" || { fail "rotate token"; echo "$TOK"; exit 1; }

if [ -n "$MULTI" ]; then
  log "Create a second gateway + token (multi-gateway)"
  GW2=$($CURL -X POST "$PA/api/v1/gateways" "${AUTH[@]}" -H 'Content-Type: application/json' -d '{
    "name":"e2e-gw2","displayName":"E2E GW2","vhost":"localhost","functionalityType":"regular"
  }')
  GW2_ID=$(echo "$GW2" | jq -r '.id // .uuid // empty')
  TOK2=$($CURL -X POST "$PA/api/v1/gateways/$GW2_ID/tokens" "${AUTH[@]}" -H 'Content-Type: application/json' -d '{}')
  REG_TOKEN_2=$(echo "$TOK2" | jq -r '.token // empty')
  { [ -n "$GW2_ID" ] && [ -n "$REG_TOKEN_2" ]; } && ok "gateway2=$GW2_ID" || { fail "create gateway2"; exit 1; }
fi

# Attach the gateway and deploy the API *before* starting the controller, so the
# controller's initial sync-on-connect picks up the deployment (avoids a race
# where the controller connects and syncs an empty deployment set first).
log "Attach gateway to API + deploy"
$CURL -X POST "$PA/api/v1/rest-apis/$API_ID/gateways" "${AUTH[@]}" -H 'Content-Type: application/json' \
  -d "[{\"gatewayId\":\"$GW_ID\"}]" | jq -c '{count}' 2>/dev/null || true
DEP=$($CURL -X POST "$PA/api/v1/rest-apis/$API_ID/deployments" "${AUTH[@]}" -H 'Content-Type: application/json' \
  -d "{\"base\":\"current\",\"gatewayId\":\"$GW_ID\",\"name\":\"dep1\"}")
echo "$DEP" | jq -c '{deploymentId, status}' 2>/dev/null || echo "$DEP"
DEPLOY_ID=$(echo "$DEP" | jq -r '.deploymentId // empty')
[ "$(echo "$DEP" | jq -r '.status // empty')" = "DEPLOYED" ] && ok "API deployed" || { fail "deploy"; exit 1; }

# ---------------------------------------------------------------- Phase 2
log "Phase 2: start gateway data plane with the registration token"
GATEWAY_REGISTRATION_TOKEN="$REG_TOKEN" $COMPOSE up -d gateway-controller gateway-runtime

log "Assert traffic through the gateway ingress (deployed -> 200)"
RESP=$(wait_ingress "$ING" 200)
BODY=$($CURL -H 'Host: localhost' "$ING" 2>/dev/null)
echo "ingress status=$RESP body=$(echo "$BODY" | head -c 160)"
if [ "$RESP" = "200" ]; then ok "gateway served the API (200)"; else
  fail "gateway did not serve the API (status $RESP)"; $COMPOSE logs gateway-controller | tail -25
fi

log "Negative routing: a path outside the API context is not served"
NEG=$($CURL -H 'Host: localhost' "http://localhost:${GW_HTTP_PORT:-18080}/no-such-api/ping" -o /dev/null -w '%{http_code}' 2>/dev/null)
[ "$NEG" = "404" ] && ok "unmapped path returns 404" || fail "unmapped path: want 404, got $NEG"

# ---------------------------------------------------------------- Undeploy / redeploy
if [ -n "$DEPLOY_ID" ]; then
  log "Undeploy the API (deployed -> 404 at the data plane)"
  $CURL -X POST "$PA/api/v1/rest-apis/$API_ID/deployments/$DEPLOY_ID/undeploy?gatewayId=$GW_ID" "${AUTH[@]}" \
    -H 'Content-Type: application/json' | jq -c '{status}' 2>/dev/null || true
  RESP=$(wait_ingress "$ING" 404)
  [ "$RESP" = "404" ] && ok "gateway stopped serving after undeploy (404)" || { fail "still served after undeploy (status $RESP)"; $COMPOSE logs gateway-controller | tail -20; }

  log "Redeploy the API (404 -> 200 again)"
  DEP2=$($CURL -X POST "$PA/api/v1/rest-apis/$API_ID/deployments" "${AUTH[@]}" -H 'Content-Type: application/json' \
    -d "{\"base\":\"current\",\"gatewayId\":\"$GW_ID\",\"name\":\"dep2\"}")
  echo "$DEP2" | jq -c '{deploymentId, status}' 2>/dev/null || echo "$DEP2"
  RESP=$(wait_ingress "$ING" 200)
  [ "$RESP" = "200" ] && ok "gateway serves the API again after redeploy (200)" || { fail "not served after redeploy (status $RESP)"; $COMPOSE logs gateway-controller | tail -20; }
else
  fail "no deployment id captured; skipping undeploy/redeploy"
fi

# ---------------------------------------------------------------- Multi-gateway
if [ -n "$MULTI" ]; then
  log "Multi-gateway: deploy the same API to a second gateway"
  $CURL -X POST "$PA/api/v1/rest-apis/$API_ID/gateways" "${AUTH[@]}" -H 'Content-Type: application/json' \
    -d "[{\"gatewayId\":\"$GW2_ID\"}]" | jq -c '{count}' 2>/dev/null || true
  DEPG2=$($CURL -X POST "$PA/api/v1/rest-apis/$API_ID/deployments" "${AUTH[@]}" -H 'Content-Type: application/json' \
    -d "{\"base\":\"current\",\"gatewayId\":\"$GW2_ID\",\"name\":\"dep-gw2\"}")
  echo "$DEPG2" | jq -c '{deploymentId, status}' 2>/dev/null || echo "$DEPG2"
  DEPLOY_ID_2=$(echo "$DEPG2" | jq -r '.deploymentId // empty')
  [ "$(echo "$DEPG2" | jq -r '.status // empty')" = "DEPLOYED" ] && ok "API deployed to gateway 2" || fail "deploy to gateway 2"

  log "Start the second gateway data plane"
  GATEWAY_REGISTRATION_TOKEN="$REG_TOKEN" GATEWAY_REGISTRATION_TOKEN_2="$REG_TOKEN_2" \
    $COMPOSE up -d gateway-controller-2 gateway-runtime-2

  log "Both gateways serve the same API (fan-out)"
  R1=$(wait_ingress "$ING" 200); R2=$(wait_ingress "$ING2" 200)
  { [ "$R1" = "200" ] && [ "$R2" = "200" ]; } && ok "both gateways serve the API (gw1=$R1 gw2=$R2)" || { fail "fan-out: gw1=$R1 gw2=$R2"; $COMPOSE logs gateway-controller-2 | tail -20; }

  log "Undeploy from gateway 2 only — gateway 1 must keep serving (per-gateway isolation)"
  $CURL -X POST "$PA/api/v1/rest-apis/$API_ID/deployments/$DEPLOY_ID_2/undeploy?gatewayId=$GW2_ID" "${AUTH[@]}" \
    -H 'Content-Type: application/json' | jq -c '{status}' 2>/dev/null || true
  R2=$(wait_ingress "$ING2" 404); R1=$(ingress_status "$ING")
  { [ "$R2" = "404" ] && [ "$R1" = "200" ]; } && ok "undeploy isolated to gateway 2 (gw2=$R2, gw1=$R1)" || fail "isolation broken: gw2=$R2 gw1=$R1"
fi

[ $FAILED -eq 0 ] && { log "RESULT: E2E PASSED"; exit 0; } || { log "RESULT: E2E FAILED"; exit 1; }
