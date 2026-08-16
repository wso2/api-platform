#!/usr/bin/env bash
# Merge collected server-side coverage and render reports.
#
# Input: the per-variant per-service counter dirs a `-coverage` suite run leaves under
# COVERAGE_OUT (default suites/it/coverage-out). Rerunnable: consumes only what is on
# disk, so a failed suite's partial harvest still reports.
#
# Outputs, all under COVERAGE_OUT:
#   merged/                     covdata pods merged across variants
#   coverage.txt                one text profile, generated code filtered out (the denominator)
#   coverage-controller.txt     the controller's slice of coverage.txt
#   coverage-policyengine.txt   the policy-engine + policies slice
#   coverage-controller.html    per-module HTML (rendered from inside each module so
#   coverage-policyengine.html  sources resolve; policy-module sources render only when
#                               their exact versions are present in the module cache)
set -euo pipefail

here="$(cd "$(dirname "$0")/.." && pwd)"
out="${COVERAGE_OUT:-$here/suites/it/coverage-out}"
controller_mod="$here/../../gateway/gateway-controller"
pe_mod="$here/../../gateway/gateway-runtime/policy-engine"

die() { echo "coverage-report: $*" >&2; exit 1; }

[ -d "$out" ] || die "no coverage output at $out — run the suite with -coverage first"

# Every <block>/<service> dir that actually holds counters. covmeta without covcounters
# means the process never flushed; covdata would fail on it, so require both.
inputs=""
while IFS= read -r dir; do
  if ls "$dir"/covmeta.* >/dev/null 2>&1 && ls "$dir"/covcounters.* >/dev/null 2>&1; then
    inputs="${inputs:+$inputs,}$dir"
  fi
done < <(find "$out" -mindepth 2 -maxdepth 2 -type d | sort)
[ -n "$inputs" ] || die "no counter dirs under $out (want <block>/<service>/covmeta.* + covcounters.*)"

echo "merging: $inputs"
rm -rf "$out/merged" && mkdir -p "$out/merged"
go tool covdata merge -i="$inputs" -o "$out/merged"
go tool covdata textfmt -i="$out/merged" -o "$out/coverage.raw.txt"

# The denominator: product code only. Generated code is dropped here to MATCH what a reader
# expects the number to mean — never-instrumentable stubs at a forced 0% only understate it.
grep -v -E '^github\.com/wso2/api-platform/gateway/gateway-controller/pkg/api/(admin|management)/' "$out/coverage.raw.txt" \
  | grep -v -E '^github\.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pythonbridge/proto/' \
  | grep -v -E '^github\.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/cmd/policy-engine/(plugin_registry|build_info)\.go' \
  > "$out/coverage.txt"

# Guard: a report with no executed statement is a pipeline failure wearing a green suite.
covered=$(awk 'NR>1 && $NF>0 {n++} END {print n+0}' "$out/coverage.txt")
[ "$covered" -gt 0 ] || die "coverage.txt contains no executed statements — collection is broken"

split_profile() { # prefix-regex -> file
  { head -1 "$out/coverage.txt"; grep -E "$1" "$out/coverage.txt" || true; } > "$2"
}
split_profile '^github\.com/wso2/api-platform/gateway/gateway-controller/' "$out/coverage-controller.txt"
split_profile '^github\.com/wso2/(api-platform/gateway/gateway-runtime/policy-engine|gateway-controllers/policies)/' "$out/coverage-policyengine.txt"
# HTML needs sources; the policy modules' exact versions live only inside the image build
# (build.yaml pins ranges), so the renderable slice is the policy-engine module alone.
split_profile '^github\.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/' "$out/coverage-policyengine-module.txt"

summary() { # file label — profile lines end "<numstmt> <hitcount>"
  awk 'NR>1 {total+=$(NF-1); if ($NF>0) covered+=$(NF-1)}
       END {if (total>0) printf "  %-16s %6.1f%% of statements (%d/%d)\n", "'"$2"'", 100*covered/total, covered, total}' "$1"
}
echo "statement coverage (generated code excluded):"
summary "$out/coverage-controller.txt"   "controller"
summary "$out/coverage-policyengine.txt" "policy-engine"
summary "$out/coverage.txt"              "combined"

# HTML per module, from inside each module so the profile's import paths resolve to sources.
(cd "$controller_mod" && go tool cover -html="$out/coverage-controller.txt" -o "$out/coverage-controller.html") \
  || echo "coverage-report: controller HTML failed (sources not resolvable?)" >&2
(cd "$pe_mod" && go tool cover -html="$out/coverage-policyengine-module.txt" -o "$out/coverage-policyengine.html") \
  || echo "coverage-report: policy-engine HTML failed (sources not resolvable?)" >&2
echo "note: policy-module sources are not renderable on the host (their exact versions exist only in the image build); their numbers are complete in coverage.txt and the summary above"

echo "reports under $out"
