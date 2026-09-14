#!/bin/bash -e
# Delegates to performance-common JMeter server starter.

# Follow symlinks: $HOME/jmeter -> perf-ai-gateway-manual/jmeter -> .../ai-gateway-manual/jmeter
SCRIPT_DIR="$(cd -P "$(dirname "$0")" && pwd)"
PERF_ROOT="$(cd -P "${SCRIPT_DIR}/../.." && pwd)"
exec "${PERF_ROOT}/performance-common/distribution/scripts/jmeter/jmeter-server-start.sh" "$@"
