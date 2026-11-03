#!/usr/bin/env bash
# Shared helpers for the platform-api-gateway-e2e skill. Source it; do not run it.
# bash 3.2 compatible (macOS). Every path and port can be overridden from the environment.

# Everything this skill creates (downloads, extracted gateways, DB, logs, state) lives in one temp
# work dir, created by start-platform-api.sh and recorded in $WORK_POINTER so later scripts find it.
# (not a ${:=} default: the braces of {GATEWAY_VERSION} would end the expansion early)
[ -n "${GW_DOWNLOAD_URL:-}" ] || GW_DOWNLOAD_URL='https://github.com/wso2/api-platform/releases/download/gateway/v{GATEWAY_VERSION}/wso2apip-api-gateway-{GATEWAY_VERSION}.zip'
: "${REQUEST_INFO_MODULE:=github.com/renuka-fernando/request-info@latest}"   # echo backend (go install)
: "${PAPI_HTTP_PORT:=9380}"                                       # curl -> platform-api (plain HTTP)
: "${PAPI_HTTPS_PORT:=9343}"                                      # gateways -> platform-api (wss/https only)
: "${BACKEND_PORT:=8090}"                                         # request-info
: "${E2E_ORG:=e2e-org}"                                           # organization handle
: "${E2E_ORG_REF:=e2e00000-0000-4000-8000-000000000001}"          # token "organization" claim (<=40 chars)

WORK_POINTER="${TMPDIR:-/tmp}/papi-gw-e2e.workdir"
if [ -n "${PAPI_GW_E2E_DIR:-}" ]; then WORK="$PAPI_GW_E2E_DIR"
elif [ -f "$WORK_POINTER" ] && [ -d "$(cat "$WORK_POINTER")" ]; then WORK="$(cat "$WORK_POINTER")"
else WORK=""; fi
LOGS="$WORK/logs"
STATE="$WORK/state.env"
PAPI_URL="http://localhost:${PAPI_HTTP_PORT}"
BASE="${PAPI_URL}/api/v0.9"
# Containers reach services on the host through host.docker.internal.
BACKEND_URL_FROM_GW="http://host.docker.internal:${BACKEND_PORT}"
[ -n "$WORK" ] && mkdir -p "$LOGS"

# need_work: scripts other than start-platform-api.sh require an existing work dir.
need_work() { [ -n "$WORK" ] && [ -d "$WORK" ] || die "no work dir — run start-platform-api.sh (or run.sh) first"; }

# new_workdir: a fresh temp dir per run. The gateway compose files bind-mount their configs, and
# some Docker runtimes (colima) only share $HOME with the VM, so the system temp dir is used only
# when a container can actually see it; otherwise a temp dir under $HOME is used.
new_workdir() {
  if [ -n "${PAPI_GW_E2E_DIR:-}" ]; then WORK="$PAPI_GW_E2E_DIR"; mkdir -p "$WORK"
  else
    WORK=$(mktemp -d "${TMPDIR:-/tmp}/papi-gw-e2e.XXXXXX")
    if docker info >/dev/null 2>&1; then
      echo ok > "$WORK/.mount-probe"
      if [ "$(docker run --rm -v "$WORK:/probe:ro" alpine:3.20 cat /probe/.mount-probe 2>/dev/null)" != ok ]; then
        rm -rf "$WORK"
        mkdir -p "$HOME/.cache"
        WORK=$(mktemp -d "$HOME/.cache/papi-gw-e2e.XXXXXX")
        printf '[e2e] %s\n' "docker cannot bind-mount the system temp dir (e.g. colima shares only \$HOME); using $WORK" >&2
      fi
      rm -f "$WORK/.mount-probe"
    fi
  fi
  echo "$WORK" > "$WORK_POINTER"
  LOGS="$WORK/logs"; STATE="$WORK/state.env"; mkdir -p "$LOGS"
}

log()  { printf '[e2e] %s\n' "$*" >&2; }
die()  { printf '[e2e] ERROR: %s\n' "$*" >&2; exit 1; }

# Repo root holding platform-api/ (the working tree under test): PAPI_REPO, else the checkout the
# shell is in, else the checkout this skill lives in.
repo_root() {
  if [ -n "${PAPI_REPO:-}" ]; then echo "$PAPI_REPO"; return; fi
  local top
  top=$(git rev-parse --show-toplevel 2>/dev/null || true)
  if [ -n "$top" ] && [ -f "$top/platform-api/cmd/main.go" ]; then echo "$top"; return; fi
  git -C "$(dirname "${BASH_SOURCE[0]}")" rev-parse --show-toplevel
}

# --- state ------------------------------------------------------------------
state_set() { # key value
  touch "$STATE"
  grep -v "^$1=" "$STATE" > "$STATE.tmp" 2>/dev/null || true
  printf '%s=%q\n' "$1" "$2" >> "$STATE.tmp"
  mv "$STATE.tmp" "$STATE"
}
state_load() { [ -f "$STATE" ] && . "$STATE"; return 0; }

# --- versions & ports ---------------------------------------------------------
# Each gateway gets a port index (1..4) on first start, kept in state; host port = index*10000 +
# the release's default host port (router 8080 -> 18080, 28080, ...). 4 keeps every port < 65536.
ver_key()    { echo "$1" | tr '.-' '__'; }
ver_index() {
  state_load
  local k="GW_INDEX_$(ver_key "$1")" n
  eval "n=\${$k:-}"
  if [ -z "$n" ]; then
    n=$(grep -c '^GW_INDEX_' "$STATE" 2>/dev/null); n=$(( ${n:-0} + 1 ))
    [ "$n" -le 4 ] || die "at most 4 gateway versions per run"
    state_set "$k" "$n"
  fi
  echo "$n"
}
ver_dashed()   { echo "$1" | tr '.' '-'; }
gw_handle()    { echo "gw-$(ver_dashed "$1")"; }               # platform-api gateway handle
gw_project()   { echo "papi-gw-$(ver_dashed "$1")"; }          # docker compose project name
gw_dir()       { echo "$WORK/gw-$1"; }
# Registered version: major.minor for semver (the manifest later stores the exact version), the
# full date for CalVer (YYYY.MM.DD), which platform-api accepts as is.
gw_reg_version() {
  case "$1" in [0-9][0-9][0-9][0-9].*) echo "$1" ;; *) echo "$1" | cut -d. -f1-2 ;; esac
}
gw_port()      { echo $(( $(ver_index "$1") * 10000 + $2 )); }  # gw_port <ver> <default host port>
gw_router()    { echo "http://localhost:$(gw_port "$1" 8080)"; }
# Admin health moved between releases (/health, /api/admin/v0.9/health, /api/admin/v1/health).
gw_healthy() {
  local p; p=$(gw_port "$1" 9094)
  for hp in /api/admin/v1/health /api/admin/v0.9/health /health; do
    curl -sf "http://localhost:$p$hp" >/dev/null 2>&1 && return 0
  done
  return 1
}
# Gateway management API base (basic auth admin/admin): /api/management/v1, /api/management/v0.9 or /.
gw_mgmt_base() {
  local b="http://localhost:$(gw_port "$1" 9090)" mp
  for mp in /api/management/v1 /api/management/v0.9 ""; do
    [ "$(curl -s -o /dev/null -w '%{http_code}' -u admin:admin "$b$mp/rest-apis")" = 200 ] && { echo "$b$mp"; return 0; }
  done
  echo "$b"
}

# --- auth ---------------------------------------------------------------------
b64url() { printf '%s' "$1" | base64 | tr '+/' '-_' | tr -d '=\n'; }
E2E_SCOPES="ap:organization:manage ap:project:manage ap:gateway:manage ap:gateway_custom_policy:manage ap:rest_api:manage ap:application:manage ap:subscription:manage ap:subscription_plan:manage ap:llm_template:manage ap:llm_provider:manage ap:llm_proxy:manage ap:mcp_proxy:manage ap:agent_proxy:manage ap:secret:manage ap:api_key:read ap:api_key:all:manage"
# platform-api runs with internal_token + skip_validation, so an unsigned token is accepted.
mint_token() {
  local h p
  h=$(b64url '{"alg":"none","typ":"JWT"}')
  p=$(b64url "{\"sub\":\"e2e-user\",\"username\":\"e2e-user\",\"email\":\"e2e@example.com\",\"organization\":\"${E2E_ORG_REF}\",\"scope\":\"${E2E_SCOPES}\"}")
  printf '%s.%s.' "$h" "$p"
}
TOKEN="$(mint_token)"

# --- HTTP -----------------------------------------------------------------------
# api METHOD PATH [JSON]  -> prints body, then a final line "HTTP <code>".
api() {
  local m="$1" p="$2" d="${3:-}"
  if [ -n "$d" ]; then
    curl -sS -X "$m" "$BASE$p" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
      --data "$d" -w '\nHTTP %{http_code}\n'
  else
    curl -sS -X "$m" "$BASE$p" -H "Authorization: Bearer $TOKEN" -w '\nHTTP %{http_code}\n'
  fi
}
http_code() { tail -1 | awk '{print $2}'; }       # api ... | http_code
http_body() { sed '$d'; }                          # api ... | http_body
# json_get FIELD.PATH  (stdin JSON)  e.g. json_get token ; json_get list.0.id
json_get() {
  python3 -c '
import json,sys
v=json.load(sys.stdin)
for k in sys.argv[1].split("."):
    if k=="": continue
    v=v[int(k)] if isinstance(v,list) else v.get(k) if isinstance(v,dict) else None
    if v is None: break
print("" if v is None else (json.dumps(v) if isinstance(v,(dict,list)) else v))' "$1"
}

# wait_for <seconds> <description> <command...>
wait_for() {
  local secs="$1" what="$2"; shift 2
  local i=0
  while [ "$i" -lt "$secs" ]; do
    if "$@" >/dev/null 2>&1; then log "$what: ready after ${i}s"; return 0; fi
    sleep 1; i=$((i+1))
  done
  log "$what: not ready after ${secs}s"; return 1
}

# Pid listening on a TCP port (empty if none).
port_pid() { { lsof -nP -t -iTCP:"$1" -sTCP:LISTEN 2>/dev/null || true; } | head -1; }

# --- scenario helpers ---------------------------------------------------------------
# ensure_created COLLECTION JSON   -> 201 created, 409 reused; anything else fails.
ensure_created() {
  local out code; out=$(api POST "/$1" "$2"); code=$(printf '%s\n' "$out" | http_code)
  case "$code" in 201) log "created $1/$(printf '%s' "$2" | json_get id)";; 409) log "reusing $1/$(printf '%s' "$2" | json_get id)";;
    *) printf '%s\n' "$out" >&2; die "create $1 failed (HTTP $code)";; esac
}
# ensure_secret HANDLE VALUE   (multipart). An existing secret is updated to VALUE.
ensure_secret() {
  local code; code=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$BASE/secrets" -H "Authorization: Bearer $TOKEN" \
    -F "id=$1" -F "displayName=$1" -F "value=$2" -F type=GENERIC)
  if [ "$code" = 409 ]; then
    code=$(curl -sS -o /dev/null -w '%{http_code}' -X PUT "$BASE/secrets/$1" -H "Authorization: Bearer $TOKEN" \
      -F "displayName=$1" -F "value=$2")
  fi
  case "$code" in 200|201) log "secret '$1' ready";; *) die "create/update secret $1 failed (HTTP $code)";; esac
}
# echo_get FIELD  (stdin: invoke.sh output) -> field of the request request-info received,
#   e.g. URL.Path, URL.RawQuery, Header.Authorization.0, Body, Method
echo_get()  { http_body | json_get "Request.$1" 2>/dev/null || true; }
# check SCENARIO VERSION CHECK EXPECTED ACTUAL  -> PASS/FAIL line, appended to $WORK/results.tsv
check() {
  local r=FAIL; [ "$4" = "$5" ] && r=PASS
  printf '%s\t%s\t%s\t%s\texpected=%s actual=%s\n' "$1" "$2" "$3" "$r" "$4" "$5" | tee -a "$WORK/results.tsv"
  [ "$r" = PASS ]
}
# invoke_until SECS FIELD EXPECTED VER METHOD PATH [BODY] [curl args...]
# Re-invokes until the echoed FIELD equals EXPECTED or SECS elapse; prints the last invoke output.
# Needed after a REdeploy: the DEPLOYED ack arrives a few seconds before the router serves the
# updated config, so the first call can still hit the previous version.
invoke_until() {
  local secs="$1" field="$2" want="$3" i=0 out; shift 3
  while :; do
    out=$("$(dirname "${BASH_SOURCE[0]}")/invoke.sh" "$@")
    [ "$(printf '%s\n' "$out" | echo_get "$field")" = "$want" ] && break
    [ "$i" -ge "$secs" ] && break
    sleep 1; i=$((i+1))
  done
  printf '%s\n' "$out"
}
# ver_below VER MIN -> true when LTS gateway release VER is older than LTS release MIN (semver,
# numeric per field). Not meaningful across release channels (LTS vs date-named STS).
ver_below() {
  python3 - "$1" "$2" <<'PY'
import sys
t=lambda v: tuple(int(x) for x in (v.split("-")[0].split(".")+["0","0"])[:3])
sys.exit(0 if t(sys.argv[1]) < t(sys.argv[2]) else 1)
PY
}
# gw_find VER PATH KEY -> first value of KEY anywhere in the gateway management API response for
# PATH (e.g. mcp-proxies/e2e-mcp), "<absent>" when missing. Shows what the gateway actually holds.
gw_find() {
  curl -s -u admin:admin "$(gw_mgmt_base "$1")/$2" | python3 -c '
import json,sys
def f(o,k):
    if isinstance(o,dict):
        if k in o: return o[k]
        o=list(o.values())
    if isinstance(o,list):
        for v in o:
            r=f(v,k)
            if r is not None: return r
try: r=f(json.load(sys.stdin),sys.argv[1])
except Exception: r=None
print("<absent>" if r is None else (json.dumps(r) if isinstance(r,(dict,list)) else r))' "$3"
}
# warned VER FIELD -> number of platform-api "adapted for older gateway" warnings for FIELD on VER.
warned() {
  local n; n=$(grep "adapted for older gateway" "$LOGS/platform-api.log" 2>/dev/null | grep -F "field=$2 " | grep -cE "gatewayVersion=$(printf '%s' "$1" | sed 's/[.]/\\./g')( |$)")
  echo "${n:-0}"
}
