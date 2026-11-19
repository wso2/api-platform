#!/usr/bin/env bash
# Forwards the gateway's management API (9090) and traffic port (8443) to this machine. Keep it running
# while you deploy and test. Press Ctrl+C to stop.
# Usage: ./scripts/port-forward.sh
set -euo pipefail
# shellcheck source=scripts/common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

trap 'kill $(jobs -p) 2>/dev/null || true' EXIT
kubectl -n "${NAMESPACE}" port-forward "svc/${RELEASE}-controller" 9090:9090 &
controller_pid=$!
kubectl -n "${NAMESPACE}" port-forward "svc/${RELEASE}-gateway-runtime" 8443:8443 &
runtime_pid=$!

# Stop when either port forward exits. This polls instead of using wait -n, which the bash 3.2 that
# ships with macOS doesn't have.
while kill -0 "${controller_pid}" 2>/dev/null && kill -0 "${runtime_pid}" 2>/dev/null; do
  sleep 2
done
echo "A port forward stopped, for example because a pod restarted. Run ./scripts/port-forward.sh again." >&2
exit 1
