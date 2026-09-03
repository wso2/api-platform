#!/bin/sh
set -eu

# ---------------------------------------------------------------------------
# rest-to-mcp: expose sample-service as MCP tools through the WSO2 AI Gateway.
#
#   sample-service (REST)
#        ^ HTTP
#   generated MCP server   <- built here by arazzo-mcp-gen
#        ^ MCP
#   WSO2 AI Gateway        <- auth, rate limiting, observability
#        ^ MCP
#   client.py
# ---------------------------------------------------------------------------

if command -v tput >/dev/null 2>&1 && [ -n "${TERM:-}" ] && tput setaf 2 >/dev/null 2>&1; then
  GREEN="$(tput setaf 2)"; YELLOW="$(tput setaf 3)"; RED="$(tput setaf 1)"
  BOLD="$(tput bold)"; RESET="$(tput sgr0)"
else
  GREEN=""; YELLOW=""; RED=""; BOLD=""; RESET=""
fi
print_ok()    { echo "${GREEN}OK  $1${RESET}"; }
print_info()  { echo "--> $1"; }
print_warn()  { echo "${YELLOW}!   $1${RESET}"; }
print_error() { echo "${RED}x   $1${RESET}"; }
print_title() { echo ""; echo "${BOLD}${GREEN}=== $1 ===${RESET}"; echo ""; }

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

if [ ! -f "${SCRIPT_DIR}/.env" ] && [ -f "${SCRIPT_DIR}/.env.example" ]; then
  cp "${SCRIPT_DIR}/.env.example" "${SCRIPT_DIR}/.env"
  print_ok "Created .env from .env.example"
fi
[ -f "${SCRIPT_DIR}/.env" ] && . "${SCRIPT_DIR}/.env"

MGMT_PORT="${MGMT_PORT:-9090}"
HEALTH_PORT="${HEALTH_PORT:-9094}"
TRAFFIC_PORT="${TRAFFIC_PORT:-8443}"
# Host port for the generated MCP server. Inside its container it always
# listens on 5000, which is the port mcp.yaml refers to.
MCP_PORT="${MCP_PORT:-5050}"
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin}"
MAX_RETRIES="${MAX_RETRIES:-45}"
RETRY_INTERVAL=2

GW_VERSION=1.2.0
GW_ZIP="${SCRIPT_DIR}/wso2apip-ai-gateway-${GW_VERSION}.zip"
GW_DIR="${SCRIPT_DIR}/wso2apip-ai-gateway-${GW_VERSION}"
GW_URL="https://github.com/wso2/api-platform/releases/download/ai-gateway/v${GW_VERSION}/wso2apip-ai-gateway-${GW_VERSION}.zip"

ARAZZO_VERSION=0.1.0
BIN_DIR="${SCRIPT_DIR}/bin"
GEN="${BIN_DIR}/arazzo-mcp-gen"

NETWORK=rest-to-mcp
BACKEND=sample-service
MCP_SERVER=rest-to-mcp-server

# --------------------------------------------------------------------------
print_title "Checking prerequisites"

for tool in docker curl unzip; do
  command -v "$tool" >/dev/null 2>&1 || { print_error "$tool is required but not installed."; exit 1; }
done
if ! docker info >/dev/null 2>&1; then
  print_error "Docker is not running. Start your Docker runtime (Docker Desktop, Rancher Desktop, colima) and try again."
  exit 1
fi
print_ok "docker, curl and unzip are available"

# --------------------------------------------------------------------------
print_title "Installing arazzo-mcp-gen"

if [ -x "$GEN" ]; then
  print_ok "Already installed at ${GEN}"
else
  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$(uname -m)" in
    arm64|aarch64) ARCH=arm64 ;;
    x86_64|amd64)  ARCH=amd64 ;;
    *) print_error "Unsupported architecture: $(uname -m)"; exit 1 ;;
  esac
  ASSET="arazzo-mcp-gen-${ARAZZO_VERSION}-${OS}-${ARCH}.zip"
  print_info "Downloading ${ASSET}..."
  mkdir -p "$BIN_DIR"
  curl -fsSL "https://github.com/wso2/arazzo-mcp-generator/releases/download/${ARAZZO_VERSION}/${ASSET}" \
    -o "${BIN_DIR}/${ASSET}"
  unzip -oq "${BIN_DIR}/${ASSET}" -d "$BIN_DIR"
  mv "${BIN_DIR}/arazzo-mcp-gen-${OS}-${ARCH}" "$GEN"
  chmod +x "$GEN"
  # The macOS binaries are unsigned, so Gatekeeper blocks them until the
  # quarantine flag is cleared.
  if [ "$OS" = "darwin" ]; then
    xattr -d com.apple.quarantine "$GEN" 2>/dev/null || true
  fi
  print_ok "Installed to ${GEN}"
fi

# --------------------------------------------------------------------------
print_title "Starting the REST backend (sample-service)"

docker network create "$NETWORK" >/dev/null 2>&1 || true
docker rm -f "$BACKEND" >/dev/null 2>&1 || true
print_info "Building sample-service..."
docker build -q -t sample-service-image "${SCRIPT_DIR}/../sample-service" >/dev/null
docker run -d --name "$BACKEND" --network "$NETWORK" -p 8090:8080 sample-service-image >/dev/null || {
  print_error "Could not start sample-service."
  echo "  If the error above mentions a port bind, something already holds 8090."
  echo "  Find it with:  docker ps | grep 8090"
  exit 1
}

n=0
until curl -sf -o /dev/null "http://localhost:8090/health"; do
  n=$((n + 1))
  [ "$n" -ge 15 ] && { print_error "sample-service did not become healthy."; exit 1; }
  sleep 1
done
print_ok "sample-service is up on http://localhost:8090 (plain REST, no MCP)"

# --------------------------------------------------------------------------
print_title "Generating the MCP server from the Arazzo spec"

print_info "Validating arazzo/ ..."
"$GEN" validate -f ./arazzo || {
  print_error "Arazzo spec failed validation. Fix the errors above and re-run."
  exit 1
}

print_info "Generating and building the image..."
"$GEN" mcp-server generate -f ./arazzo -p 5000 -o ./generated | tee "${SCRIPT_DIR}/.generate.log"

IMAGE="$(grep -oE '^.*Image: +[a-z0-9._-]+' "${SCRIPT_DIR}/.generate.log" | sed -E 's/.*Image: +//' | tail -1)"
if [ -z "${IMAGE:-}" ]; then
  IMAGE=sample-service-echo-workflows-mcp-server
  print_warn "Could not read the image name from the output; assuming ${IMAGE}"
fi
print_ok "Built image: ${IMAGE}"
print_info "Generated code is in ./generated - mcp_server.py is worth a look."

docker rm -f "$MCP_SERVER" >/dev/null 2>&1 || true
docker run -d --name "$MCP_SERVER" --network "$NETWORK" \
  -p "${MCP_PORT}:5000" "$IMAGE" >/dev/null || {
  print_error "Could not start the MCP server."
  echo "  If the error above mentions a port bind, ${MCP_PORT} is already in use."
  echo "  Change MCP_PORT in ${SCRIPT_DIR}/.env and re-run, or free the port."
  echo "  Find what holds it with:  docker ps | grep ${MCP_PORT}"
  exit 1
}
sleep 3
print_ok "MCP server running on http://localhost:${MCP_PORT}/mcp"

# --------------------------------------------------------------------------
print_title "Starting the WSO2 AI Gateway"

if [ -f "$GW_ZIP" ]; then
  print_info "Distribution zip already present, skipping download."
else
  print_info "Downloading the AI Gateway ${GW_VERSION} distribution..."
  curl -fsSL "$GW_URL" -o "$GW_ZIP"
fi
unzip -oq "$GW_ZIP" -d "$SCRIPT_DIR"
# Passing the credentials through keeps the distribution's setup script
# from stopping to prompt for them.
[ -f "${GW_DIR}/scripts/setup.sh" ] && (cd "$GW_DIR" && \
  ADMIN_USERNAME="$ADMIN_USERNAME" ADMIN_PASSWORD="$ADMIN_PASSWORD" \
  sh scripts/setup.sh >/dev/null)
(cd "$GW_DIR" && docker compose up -d)

print_info "Waiting for the gateway controller..."
n=0
until [ "$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:${HEALTH_PORT}/health")" = "200" ]; do
  n=$((n + 1))
  if [ "$n" -ge "$MAX_RETRIES" ]; then
    print_error "Gateway did not become healthy. Check: cd ${GW_DIR} && docker compose logs"
    exit 1
  fi
  sleep "$RETRY_INTERVAL"
done
print_ok "Gateway controller is healthy"

GW_NETWORK="$(docker network ls --filter name=gateway-network --format '{{.Name}}' | head -1)"
if [ -n "$GW_NETWORK" ]; then
  docker network connect "$GW_NETWORK" "$MCP_SERVER" 2>/dev/null || true
  print_ok "MCP server joined the gateway network (${GW_NETWORK})"
else
  print_warn "Could not find the gateway network - routing may fail."
fi

# --------------------------------------------------------------------------
print_title "Registering the MCP proxy"

AUTH="Authorization: Basic $(printf %s "${ADMIN_USERNAME}:${ADMIN_PASSWORD}" | base64 | tr -d '\r\n')"
MGMT="http://localhost:${MGMT_PORT}/api/management/v1"

curl -s -X DELETE "${MGMT}/mcp-proxies/sample-service-mcp-v1.0" -H "$AUTH" >/dev/null 2>&1 || true
curl -sf -X POST "${MGMT}/mcp-proxies" \
  -H "Content-Type: application/yaml" -H "$AUTH" \
  --data-binary @"${SCRIPT_DIR}/mcp.yaml" >/dev/null || {
    print_error "Failed to register the MCP proxy. Is the gateway management API up on ${MGMT_PORT}?"
    exit 1
  }
print_ok "MCP proxy registered at /sample-service"

print_info "Waiting for the route to go live..."
n=0
until [ "$(curl -sk -o /dev/null -w '%{http_code}' -X POST \
  "https://localhost:${TRAFFIC_PORT}/sample-service/mcp" \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')" = "200" ]; do
  n=$((n + 1))
  if [ "$n" -ge 20 ]; then
    print_warn "Route did not answer 200 yet. It may still come up - try test.sh."
    break
  fi
  sleep "$RETRY_INTERVAL"
done

# --------------------------------------------------------------------------
print_title "Ready"
echo "  Run the tests   : ./test.sh"
echo "  Run the client  : python3 -m venv .venv && . .venv/bin/activate"
echo "                    python3 -m pip install -r requirements.txt && python3 client.py"
echo "  Tear it all down: ./teardown.sh"
echo ""
echo "  Gateway MCP endpoint : https://localhost:${TRAFFIC_PORT}/sample-service/mcp"
echo "  Generated MCP server : http://localhost:${MCP_PORT}/mcp"
echo "  REST backend         : http://localhost:8090"
echo ""
