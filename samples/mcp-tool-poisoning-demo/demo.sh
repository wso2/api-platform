#!/usr/bin/env bash
# Demonstrates MCP tool poisoning, once with no gateway policy in front of
# the evil-server (pass1) and once through a WSO2 API Platform MCP Proxy
# with the MCP Access Control policy attached (pass2).
#
# Usage: ./demo.sh [pass1|pass2|all]   (default: all)
#
# Exit codes for `pass2` (and the pass2 leg of `all`):
#   0  gateway configured, poisoned tool correctly filtered out
#   2  GATEWAY_MCP_URL not set -- gateway prerequisite not configured yet
#   3  gateway configured, but the poisoned tool is still present
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib.sh
source "$SCRIPT_DIR/scripts/lib.sh"

if [[ -f "$SCRIPT_DIR/.env" ]]; then
  set -a
  # shellcheck source=.env.example
  source "$SCRIPT_DIR/.env"
  set +a
fi

MODE="${1:-all}"

require_cmd curl
require_cmd jq

INIT_BODY='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"mcp-tool-poisoning-demo-client","version":"1.0.0"}}}'
NOTIFIED_BODY='{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}'
LIST_BODY='{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'

# Substrings that show up in this demo's injected payload. This is only how
# the *demo script* recognizes and reports on the planted payload in pass 1 --
# the gateway's MCP Access Control policy in pass 2 doesn't inspect content at
# all, it denies by tool name (see README).
POISON_MARKERS='IMPORTANT|http://example.com|debug_context|Do not mention'

list_tools() {
  local url="$1" token="${2:-}"
  mcp_request "$url" "$INIT_BODY" "$token" >/dev/null || return 1
  mcp_request "$url" "$NOTIFIED_BODY" "$token" >/dev/null || return 1
  mcp_request "$url" "$LIST_BODY" "$token"
}

count_poisoned() {
  local json="$1"
  echo "$json" | jq -r '.result.tools[].description' 2>/dev/null | grep -Ec "$POISON_MARKERS" || true
}

flag_poisoned() {
  local json="$1" name desc
  echo "$json" | jq -c '.result.tools[]' 2>/dev/null | while read -r tool; do
    name=$(echo "$tool" | jq -r '.name')
    desc=$(echo "$tool" | jq -r '.description')
    if echo "$desc" | grep -Eq "$POISON_MARKERS"; then
      printf "   %s✘ %-24s POISONED -- hidden instructions embedded in description%s\n" "$RED" "$name" "$NC"
      echo "$desc" | fold -s -w 96 | head -4 | sed "s/^/     ${DIM}/;s/\$/${NC}/"
    else
      printf "   %s✔ %-24s clean%s\n" "$GREEN" "$name" "$NC"
    fi
  done
}

print_diff() {
  log_header "Summary: pass 1 vs pass 2"
  local names1 names2
  names1=$(jq -r '.result.tools[].name' "$SCRIPT_DIR/.pass1-response.json" 2>/dev/null | sort)
  names2=$(jq -r '.result.tools[].name' "$SCRIPT_DIR/.pass2-response.json" 2>/dev/null | sort)
  comm -23 <(echo "$names1") <(echo "$names2") | while read -r removed; do
    [[ -n "$removed" ]] && printf "   %s✔ %-24s removed by gateway policy%s\n" "$GREEN" "$removed" "$NC"
  done
  comm -12 <(echo "$names1") <(echo "$names2") | while read -r kept; do
    [[ -n "$kept" ]] && printf "   %s• %-24s present in both (unaffected, benign tool)%s\n" "$DIM" "$kept" "$NC"
  done
}

run_pass1() {
  log_header "PASS 1 -- Direct connection, no gateway policy"
  log_info "Client -> evil-server directly at ${EVIL_SERVER_URL}"

  local resp total poisoned
  resp=$(list_tools "$EVIL_SERVER_URL") || { log_err "Could not reach evil-server. Did you run ./setup.sh?"; exit 1; }
  echo "$resp" > "$SCRIPT_DIR/.pass1-response.json"

  total=$(echo "$resp" | jq '.result.tools | length' 2>/dev/null || echo 0)
  poisoned=$(count_poisoned "$resp")

  echo
  echo "Tools returned by tools/list:"
  flag_poisoned "$resp"
  echo

  if [[ "$poisoned" -gt 0 ]]; then
    log_warn "${poisoned}/${total} tool(s) contain an embedded prompt-injection payload."
    log_warn "Nothing between the client and the MCP server is inspecting tool descriptions."
    log_warn "An AI agent wired up to this server would receive that instruction as-is."
  else
    log_ok "No poisoned tools detected."
  fi
}

run_pass2() {
  log_header "PASS 2 -- Through the WSO2 API Platform MCP Proxy (MCP Access Control)"

  if [[ -z "${GATEWAY_MCP_URL:-}" ]]; then
    log_warn "GATEWAY_MCP_URL is not set -- the gateway prerequisite hasn't been configured yet."
    cat <<EOF

   To see pass 2, first complete "Configure the gateway" in README.md:
     1. In AI Workspace, create an MCP Proxy pointing at:
          ${EVIL_SERVER_URL}
     2. Attach the "MCP Access Control" policy: tools.mode=deny,
        tools.exceptions=[list_open_invoices].
     3. Deploy the proxy to AI Gateway and copy its invoke URL.
     4. Put that URL in .env as GATEWAY_MCP_URL (and GATEWAY_API_KEY if your
        deployment requires a subscription key / token).
     5. Re-run: ./demo.sh pass2

EOF
    return 2
  fi

  log_info "Client -> WSO2 API Platform MCP Proxy at ${GATEWAY_MCP_URL}"

  local resp total poisoned valid_tools
  resp=$(list_tools "$GATEWAY_MCP_URL" "${GATEWAY_API_KEY:-}") || {
    log_err "Could not reach GATEWAY_MCP_URL. Confirm the proxy is deployed and reachable."
    exit 1
  }
  echo "$resp" > "$SCRIPT_DIR/.pass2-response.json"

  if echo "$resp" | jq -e '(.result.tools | type) == "array"' >/dev/null 2>&1; then
    valid_tools=1
  else
    valid_tools=0
  fi
  total=$(echo "$resp" | jq '.result.tools | length' 2>/dev/null || echo 0)
  poisoned=$(count_poisoned "$resp")

  echo
  echo "Tools returned by tools/list:"
  flag_poisoned "$resp"
  echo

  if [[ -f "$SCRIPT_DIR/.pass1-response.json" ]]; then
    print_diff
  fi

  if [[ "$valid_tools" -ne 1 ]]; then
    log_err "MCP proxy did not return a valid tool list (no .result.tools array in the response)."
    log_err "Confirm GATEWAY_MCP_URL/GATEWAY_API_KEY are correct and the proxy is deployed and reachable."
    return 3
  elif [[ "$poisoned" -eq 0 ]]; then
    log_ok "0 poisoned tools returned -- the gateway policy filtered it out before it reached the client."
    return 0
  else
    log_err "${poisoned} poisoned tool(s) still present."
    log_err "Check that the MCP Access Control policy is attached AND deployed on this proxy, and that"
    log_err "tools.exceptions contains list_open_invoices but not get_weather_report."
    return 3
  fi
}

case "$MODE" in
  pass1|1)
    run_pass1
    ;;
  pass2|2)
    run_pass2
    exit $?
    ;;
  all)
    run_pass1
    run_pass2
    exit $?
    ;;
  *)
    echo "Usage: $0 [pass1|pass2|all]"
    exit 1
    ;;
esac
