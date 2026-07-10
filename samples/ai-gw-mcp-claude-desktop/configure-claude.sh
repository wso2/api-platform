#!/bin/sh
set -eu

# Initialise tput colors if available
if command -v tput >/dev/null 2>&1 && [ -n "${TERM:-}" ] && tput setaf 2 >/dev/null 2>&1; then
  GREEN="$(tput setaf 2)"
  YELLOW="$(tput setaf 3)"
  RED="$(tput setaf 1)"
  BOLD="$(tput bold)"
  RESET="$(tput sgr0)"
else
  GREEN=""; YELLOW=""; RED=""; BOLD=""; RESET=""
fi

print_ok()    { echo "${GREEN}✔  $1${RESET}"; }
print_info()  { echo "-->  $1"; }
print_warn()  { echo "${YELLOW}⚠   $1${RESET}"; }
print_error() { echo "${RED}✖  $1${RESET}"; }
print_title() { echo ""; echo "${BOLD}${GREEN}=== $1 ===${RESET}"; echo ""; }

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ ! -f "${SCRIPT_DIR}/.env" ] && [ -f "${SCRIPT_DIR}/.env.example" ]; then
  cp "${SCRIPT_DIR}/.env.example" "${SCRIPT_DIR}/.env"
fi
if [ -f "${SCRIPT_DIR}/.env" ]; then
  . "${SCRIPT_DIR}/.env"
fi

BEARER_TOKEN="${BEARER_TOKEN:-}"
TRAFFIC_PORT="${TRAFFIC_PORT:-8443}"
MCP_URL="https://localhost:${TRAFFIC_PORT}/reading-list/mcp"

if [ -z "$BEARER_TOKEN" ]; then
  print_error "BEARER_TOKEN is not set. Copy .env.example to .env and try again."
  exit 1
fi

print_title "Configuring Claude Desktop"

# ── Detect Claude Desktop config path ────────────────────────────────────────
OS="$(uname -s)"
case "$OS" in
  Darwin)
    CLAUDE_CONFIG_DIR="${HOME}/Library/Application Support/Claude"
    RESTART_CMD="pkill -x 'Claude' 2>/dev/null || true; sleep 1; open -a Claude"
    ;;
  Linux)
    CLAUDE_CONFIG_DIR="${HOME}/.config/Claude"
    RESTART_CMD=""
    ;;
  *)
    print_warn "Unsupported OS '${OS}'. Locate claude_desktop_config.json manually."
    CLAUDE_CONFIG_DIR=""
    RESTART_CMD=""
    ;;
esac

CLAUDE_CONFIG="${CLAUDE_CONFIG_DIR}/claude_desktop_config.json"

if [ -z "$CLAUDE_CONFIG_DIR" ] || [ ! -d "$CLAUDE_CONFIG_DIR" ]; then
  print_error "Claude Desktop config directory not found at: ${CLAUDE_CONFIG_DIR}"
  echo ""
  echo "Is Claude Desktop installed? Download from: https://claude.ai/download"
  echo ""
  echo "If installed in a non-standard location, manually add this to claude_desktop_config.json:"
  print_manual_config() { true; }
  _print_snippet
  exit 1
fi

print_info "Found Claude Desktop config directory: ${CLAUDE_CONFIG_DIR}"

# ── Backup existing config ────────────────────────────────────────────────────
if [ -f "$CLAUDE_CONFIG" ]; then
  BACKUP="${CLAUDE_CONFIG}.bak.$(date +%Y%m%d_%H%M%S)"
  cp "$CLAUDE_CONFIG" "$BACKUP"
  print_ok "Backed up existing config to: ${BACKUP}"
else
  print_info "No existing config found — creating new one."
  echo '{}' > "$CLAUDE_CONFIG"
fi

# ── Merge in the MCP server entry ─────────────────────────────────────────────
print_info "Merging reading-list MCP server into claude_desktop_config.json..."

python3 - "$CLAUDE_CONFIG" "$MCP_URL" "$BEARER_TOKEN" << 'PYEOF'
import json, sys

config_path = sys.argv[1]
mcp_url     = sys.argv[2]
token       = sys.argv[3]

with open(config_path) as f:
    config = json.load(f)

config.setdefault("mcpServers", {})
config["mcpServers"]["reading-list"] = {
    "command": "npx",
    "args": [
        "mcp-remote@latest",
        mcp_url,
        "--header",
        f"Authorization: Bearer {token}"
    ],
    "env": {
        "NODE_TLS_REJECT_UNAUTHORIZED": "0"
    }
}

with open(config_path, "w") as f:
    json.dump(config, f, indent=2)

print(json.dumps(config, indent=2))
PYEOF

print_ok "claude_desktop_config.json updated"

# ── Restart Claude Desktop ─────────────────────────────────────────────────────
if [ -n "$RESTART_CMD" ]; then
  print_info "Restarting Claude Desktop..."
  eval "$RESTART_CMD"
  print_ok "Claude Desktop restarted"
else
  print_warn "Please restart Claude Desktop manually to apply the new MCP server."
fi

echo ""
print_ok "Done. Try these prompts in Claude Desktop:"
echo ""
echo "  \"What books are on my reading list?\""
echo "  \"Add 'The Pragmatic Programmer' by Andy Hunt with status to_read\""
echo "  \"Delete book with ID 3 from my reading list\""
echo ""
print_warn "The Bearer token expires in 2099 — no renewal needed for this demo."
echo ""
