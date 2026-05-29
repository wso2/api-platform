#!/bin/sh
set -eu

print_ok() {
  tput setaf 2
  echo "✔  $1"
  tput sgr0
}

print_info() {
  echo "-->  $1"
}

print_title() {
  echo
  tput bold
  tput setaf 2
  echo "=== $1 ==="
  tput sgr0
  echo
}

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

print_title "Teardown"
print_info "Removing mock LLM container..."
docker rm -f mock-llm-openai 2>/dev/null || true
print_ok "mock-llm-openai removed"

print_info "Stopping WSO2 AI Gateway stack..."
cd "${SCRIPT_DIR}/wso2apip-ai-gateway-1.1.0/"
docker compose down -v
print_ok "Teardown complete."
