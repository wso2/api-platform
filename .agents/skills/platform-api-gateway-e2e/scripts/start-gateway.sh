#!/usr/bin/env bash
# start-gateway.sh <version>   e.g. 1.0.0, 1.1.0, 1.2.0 (any published gateway release)
# Downloads the release zip from GitHub into the work dir, registers the gateway in platform-api,
# mints a registration token, shifts the release's host ports by <index>*10000, wires the control
# plane and starts gateway-controller + gateway-runtime. Re-running rotates the token and
# recreates the containers.
set -euo pipefail
. "$(dirname "$0")/lib.sh"
need_work
VER="${1:?usage: start-gateway.sh <gateway version, e.g. 1.2.0>}"
IDX=$(ver_index "$VER"); GW=$(gw_handle "$VER"); PROJ=$(gw_project "$VER"); DIR=$(gw_dir "$VER")
docker info >/dev/null 2>&1 || die "docker daemon not reachable"

# 1. Download + unpack (once per work dir).
if [ ! -f "$DIR/docker-compose.yaml" ]; then
  url=$(printf '%s' "$GW_DOWNLOAD_URL" | sed "s/{GATEWAY_VERSION}/$VER/g")
  zip="$WORK/downloads/wso2apip-api-gateway-$VER.zip"; mkdir -p "$WORK/downloads"
  log "downloading $url"
  curl -fsSL -o "$zip" "$url" || die "download failed: $url (is $VER a published gateway release?)"
  rm -rf "$WORK/unzip-$VER" "$DIR"; mkdir -p "$WORK/unzip-$VER"
  unzip -q "$zip" -d "$WORK/unzip-$VER"
  compose=$(find "$WORK/unzip-$VER" -maxdepth 2 -name docker-compose.yaml | head -1)
  [ -n "$compose" ] || die "no docker-compose.yaml in $zip"
  mv "$(dirname "$compose")" "$DIR"; rm -rf "$WORK/unzip-$VER"
  # - "H:C"  ->  - "<IDX*10000+H>:C"   (only host ports clash between stacks)
  python3 - "$DIR/docker-compose.yaml" "$IDX" <<'PY'
import re,sys
p,idx=sys.argv[1],int(sys.argv[2])
s=open(p).read()
s=re.sub(r'(-\s*")(\d+):(\d+)"', lambda m: f'{m.group(1)}{idx*10000+int(m.group(2))}:{m.group(3)}"', s)
open(p,"w").write(s)
PY
fi

# 2. Register the gateway. Deployments are shaped for the stored version, which the controller's
#    manifest replaces with the exact build version (1.0.0 sends none -> stored as 1.0.0).
REG=$(gw_reg_version "$VER")
body="{\"id\":\"$GW\",\"displayName\":\"Gateway $VER\",\"functionalityType\":\"${GW_FUNCTIONALITY:-regular}\",\"version\":\"$REG\",\"endpoints\":[\"http://localhost\"]}"
out=$(api POST /gateways "$body"); code=$(printf '%s\n' "$out" | http_code)
case "$code" in
  201) log "gateway '$GW' registered (version $REG)" ;;
  409) log "gateway '$GW' already registered — reusing" ;;
  *) printf '%s\n' "$out" >&2; die "register gateway failed (HTTP $code)" ;;
esac

# 3. Registration token (plaintext is returned only here).
out=$(api POST "/gateways/$GW/tokens")
[ "$(printf '%s\n' "$out" | http_code)" = 201 ] || { printf '%s\n' "$out" >&2; die "token rotation failed"; }
GW_TOKEN=$(printf '%s\n' "$out" | http_body | json_get token)
[ -n "$GW_TOKEN" ] || die "no token in response"

# 4. Control-plane wiring. Releases with scripts/setup.sh (1.2.0+) read config only through
#    {{ env }} tokens in api-platform.env, which setup.sh creates along with the required admin
#    user and the AES key. Older releases interpolate GATEWAY_CONTROLPLANE_HOST /
#    GATEWAY_REGISTRATION_TOKEN from the compose .env. Gateways dial wss/https only.
CP_HOST="host.docker.internal:${PAPI_HTTPS_PORT}"
cd "$DIR"
if [ -x scripts/setup.sh ]; then
  if [ ! -f api-platform.env ]; then
    COMPOSE_PROJECT_NAME="$PROJ" ADMIN_USERNAME=admin ADMIN_PASSWORD=admin ./scripts/setup.sh > "$LOGS/gw-$VER-setup.log" 2>&1 \
      || { cat "$LOGS/gw-$VER-setup.log"; die "setup.sh failed"; }
  fi
  grep -v '^APIP_GW_CONTROLLER_CONTROLPLANE_' api-platform.env > api-platform.env.tmp || true
  { cat api-platform.env.tmp
    echo "APIP_GW_CONTROLLER_CONTROLPLANE_HOST=$CP_HOST"
    echo "APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN=$GW_TOKEN"
    echo "APIP_GW_CONTROLLER_CONTROLPLANE_INSECURE_SKIP_VERIFY=true"; } > api-platform.env
  rm -f api-platform.env.tmp
else
  printf 'GATEWAY_CONTROLPLANE_HOST=%s\nGATEWAY_REGISTRATION_TOKEN=%s\n' "$CP_HOST" "$GW_TOKEN" > .env
fi

# 5. Start only the two core services (observability profiles stay off). Volumes are dropped first
#    so the controller never carries deployments from an earlier platform-api database.
docker compose -p "$PROJ" down -v --remove-orphans > /dev/null 2>&1 || true
docker compose -p "$PROJ" up -d --force-recreate gateway-controller gateway-runtime > "$LOGS/gw-$VER-compose.log" 2>&1 \
  || { cat "$LOGS/gw-$VER-compose.log"; die "docker compose up failed"; }

wait_for 300 "gateway $VER controller health" gw_healthy "$VER" \
  || { docker compose -p "$PROJ" logs --tail 40 gateway-controller; die "gateway $VER controller unhealthy"; }

is_active() { api GET "/gateways/$GW" | http_body | json_get isActive | grep -qi true; }
wait_for 60 "gateway $VER connected to platform-api" is_active \
  || { docker compose -p "$PROJ" logs --tail 40 gateway-controller; die "gateway $VER did not connect (see controller log above)"; }

has_exact_version() { api GET "/gateways/$GW" | http_body | json_get version | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$'; }
wait_for 30 "gateway $VER manifest" has_exact_version || log "manifest version not reported yet"
stored=$(api GET "/gateways/$GW" | http_body | json_get version)
log "gateway $VER up: router $(gw_router "$VER"), controller REST http://localhost:$(gw_port "$VER" 9090), platform-api stored version '$stored'"
