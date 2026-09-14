#!/bin/bash -e
# Build summary.csv from JMeter results (trimmed column set for API Gateway EKS).

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
MANUAL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
# shellcheck source=env.example
source "${SCRIPT_DIR}/env.example"
# shellcheck source=../lib/common.sh
source "${MANUAL_DIR}/lib/common.sh"
require_bash4

PERF_HOME="${PERF_HOME:-${HOME}/perf-api-gateway-manual}"
JMETER_DIR="${PERF_HOME}/jmeter"
RESULTS_DIR="${JMETER_DIR}/results"
WARMUP="${WARMUP:-}"
JTL_SPLITTER_JAR="${JTL_SPLITTER_JAR:-${PERF_ROOT}/performance-common/components/jtl-splitter/target/jtl-splitter-0.4.6-SNAPSHOT.jar}"
CREATE_SUMMARY="${PERF_ROOT}/performance-common/distribution/scripts/jmeter/create-summary-csv.sh"

export JTL_SPLITTER_JAR

if [[ -z "$WARMUP" && -f "${RESULTS_DIR}/test-metadata.json" ]]; then
    WARMUP=$(jq -r '.warmup_time // empty' "${RESULTS_DIR}/test-metadata.json")
fi
WARMUP="${WARMUP:-300}"

echo "==> Results: ${RESULTS_DIR}"
echo "==> Warmup: ${WARMUP}s"
[[ -d "$RESULTS_DIR" ]] || { echo "No results dir. Run a perf test first."; exit 1; }

ensure_jtl_for_split() {
    local run_dir="$1"
    if [[ -f "${run_dir}/results.jtl" ]]; then
        printf '%s\n' "${run_dir}/results.jtl"
        return 0
    fi
    if [[ -f "${run_dir}/jtls.zip" ]]; then
        echo "  extracting results.jtl from ${run_dir}/jtls.zip ..." >&2
        unzip -p "${run_dir}/jtls.zip" results.jtl >"${run_dir}/results.jtl"
        printf '%s\n' "${run_dir}/results.jtl"
        return 0
    fi
    return 1
}

echo "==> Splitting JTLs and writing *-measurement-summary.json ..."
split_count=0
while IFS= read -r run_dir; do
    [[ -f "${run_dir}/results-measurement-summary.json" ]] && continue
    jtl=$(ensure_jtl_for_split "$run_dir") || continue
    echo "  split: ${jtl} (large files may take several minutes)" >&2
    "${PERF_HOME}/jtl-splitter/jtl-splitter.sh" -m 2g -- \
        -f "$jtl" -d -t "$WARMUP" -u SECONDS -s
    split_count=$((split_count + 1))
done < <(find "$RESULTS_DIR" \( -name 'jtls.zip' -o -name 'results.jtl' \) | xargs -I{} dirname {} | sort -u)

summary_json_count=$(find "$RESULTS_DIR" -name 'results-measurement-summary.json' | wc -l | tr -d ' ')
echo "==> Found ${summary_json_count} measurement summary JSON file(s) (split ops: ${split_count})"
if [[ "$summary_json_count" -eq 0 ]]; then
    echo "ERROR: No results-measurement-summary.json under ${RESULTS_DIR}." >&2
    echo "  Ensure a perf test completed and jtl-splitter JAR exists (mvn package in performance-common)." >&2
    exit 1
fi

cd "$JMETER_DIR"
rm -f summary.csv summary.full.csv

# Path layout: .../<scenario>/<heap>_heap/<users>_users/<msize>B/<rsize>B_response/<sleep>ms_sleep
# create-summary-csv.sh locates the scenario directory by counting back one level per -c/-r
# pair, so every level below the scenario must be declared here even though the Python step
# below keeps only a subset. Omitting any pair shifts the window and yields the wrong
# Scenario Name plus N/A for Concurrent Users.
"$BASH" "$CREATE_SUMMARY" \
    -n "API Gateway" \
    -d "$RESULTS_DIR" \
    -j 1 \
    -p jmeter \
    -o summary.full.csv \
    -c "Heap Size" -r '([0-9]+[a-zA-Z])_heap' \
    -c "Concurrent Users" -r '([0-9]+)_users' \
    -c "Message Size (Bytes)" -r '^([0-9]+)B$' \
    -c "Response Size (Bytes)" -r '([0-9]+)B_response' \
    -c "Back-end Service Delay (ms)" -r '([0-9]+)ms_sleep'

python3 - <<'PY'
import csv
from pathlib import Path

src = Path("summary.full.csv")
dst = Path("summary.csv")
keep = [
    "Scenario Name",
    "Concurrent Users",
    "Throughput (Requests/sec)",
    "Average Response Time (ms)",
    "# Samples",
    "Error Count",
    "Error %",
    "Average Users in the System",
    "Standard Deviation of Response Time (ms)",
    "Minimum Response Time (ms)",
    "Maximum Response Time (ms)",
    "75th Percentile of Response Time (ms)",
    "90th Percentile of Response Time (ms)",
    "95th Percentile of Response Time (ms)",
    "98th Percentile of Response Time (ms)",
    "99th Percentile of Response Time (ms)",
    "99.9th Percentile of Response Time (ms)",
    "Received (KB/sec)",
    "Sent (KB/sec)",
]
with src.open(newline="") as f:
    reader = csv.DictReader(f)
    missing = [c for c in keep if c not in (reader.fieldnames or [])]
    if missing:
        raise SystemExit(f"summary.full.csv missing columns: {missing}")
    rows = [{c: row.get(c, "") for c in keep} for row in reader]
with dst.open("w", newline="") as f:
    writer = csv.DictWriter(f, fieldnames=keep)
    writer.writeheader()
    writer.writerows(rows)
print(f"==> Trimmed {len(rows)} row(s) -> summary.csv ({len(keep)} columns)")
PY

rm -f summary.full.csv

data_rows=$(($(wc -l <summary.csv) - 1))
echo "==> Wrote ${JMETER_DIR}/summary.csv (${data_rows} data row(s))"
if [[ "$data_rows" -eq 0 ]]; then
    echo "WARNING: CSV has headers only. Check results-measurement-summary.json files." >&2
    exit 1
fi
