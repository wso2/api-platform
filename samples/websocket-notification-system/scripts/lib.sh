#!/usr/bin/env bash
# Shared helpers sourced by setup.sh / demo.sh / teardown.sh
# Expects SCRIPT_DIR to already be set by the sourcing script.

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
BLUE=$'\033[0;34m'
BOLD=$'\033[1m'
NC=$'\033[0m'

NOTIFICATION_PORT="${NOTIFICATION_PORT:-8080}"
SERVER_PID_FILE="$SCRIPT_DIR/.server.pid"
SERVER_LOG_FILE="$SCRIPT_DIR/.server.log"

log_header() { printf "\n%s%s== %s ==%s\n" "$BOLD" "$BLUE" "$1" "$NC"; }
log_info()   { printf "%s%s%s %s\n" "$BLUE" "➜" "$NC" "$1"; }
log_ok()     { printf "%s%s%s %s\n" "$GREEN" "✔" "$NC" "$1"; }
log_warn()   { printf "%s%s%s %s\n" "$YELLOW" "⚠" "$NC" "$1"; }
log_err()    { printf "%s%s%s %s\n" "$RED" "✘" "$NC" "$1" >&2; }

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log_err "Required command '$1' not found. Please install it and re-run."
    exit 1
  fi
}
