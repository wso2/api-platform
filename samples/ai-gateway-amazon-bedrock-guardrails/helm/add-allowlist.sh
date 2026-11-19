#!/usr/bin/env bash
# The chart renders a fixed config.toml with no field for root-level keys, so this adds the guardrail
# allowlist at the top of config.toml in an unpacked copy of the chart.
# Needs SUBST_VARS and the variables it names, which scripts/common.sh exports.
# Usage: ./helm/add-allowlist.sh <unpacked-chart-folder>
set -euo pipefail
TEMPLATE="${1:?Pass the unpacked chart folder.}/templates/gateway/gateway-config.yaml"
ALLOWLIST="$(envsubst "${SUBST_VARS}" < "$(dirname "${BASH_SOURCE[0]}")/allowlist.toml" | sed 's/^/    /')"
export ALLOWLIST
grep -q '^  config\.toml: |' "${TEMPLATE}" || { echo "No config.toml in ${TEMPLATE}" >&2; exit 1; }
awk '{ print } /^  config\.toml: \|-?$/ && !done { print ENVIRON["ALLOWLIST"]; print ""; done = 1 }' \
  "${TEMPLATE}" > "${TEMPLATE}.new"
mv "${TEMPLATE}.new" "${TEMPLATE}"
