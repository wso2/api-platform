#!/bin/bash -e
# Format summary.csv into gateway/perf/README.md and open a GitHub PR.
#
# Required:
#   SUMMARY_CSV                 path to summary-<TEST_ID>.csv
#   GH_TOKEN or GITHUB_TOKEN    PAT with repo + pull_request (or `gh auth login`)
#
# Optional:
#   RESULTS_PR_REPO             default https://github.com/wso2/api-platform.git
#   RESULTS_PR_BASE             default main
#   RESULTS_PR_README           default gateway/perf/README.md
#   RESULTS_PR_BRANCH           default perf-results-<TEST_ID>
#   TEST_ID, GATEWAY_HELM_CHART_VERSION, GATEWAY_NODE_INSTANCE_TYPE,
#   GATEWAY_RUNTIME_CPU_LIMIT, GATEWAY_RUNTIME_MEM_LIMIT,
#   ROUTER_CONCURRENCY, GOMAXPROCS, GOGC, GOMEMLIMIT, RUN_PERF_OPTS
#
# Jenkins (after Step 15):
#   export PUBLISH_RESULTS_PR=1   # or call this script explicitly
#   SUMMARY_CSV=.../summary-${TEST_ID}.csv ./api-gateway-eks-perf/publish-results-pr.sh

set -euo pipefail

SUMMARY_CSV="${SUMMARY_CSV:?Set SUMMARY_CSV to the summary CSV path}"
[[ -f "${SUMMARY_CSV}" ]] || { echo "ERROR: SUMMARY_CSV not found: ${SUMMARY_CSV}" >&2; exit 1; }

RESULTS_PR_REPO="${RESULTS_PR_REPO:-https://github.com/wso2/api-platform.git}"
RESULTS_PR_BASE="${RESULTS_PR_BASE:-main}"
RESULTS_PR_README="${RESULTS_PR_README:-gateway/perf/README.md}"
TEST_ID="${TEST_ID:-$(date +%Y%m%d-%H%M%S)}"
RESULTS_PR_BRANCH="${RESULTS_PR_BRANCH:-perf-results-${TEST_ID}}"

export GH_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
if [[ -z "${GH_TOKEN}" ]] && ! gh auth status >/dev/null 2>&1; then
    echo "ERROR: set GH_TOKEN (or GITHUB_TOKEN), or run gh auth login." >&2
    exit 1
fi
command -v gh >/dev/null || { echo "ERROR: gh CLI required on the Jenkins slave." >&2; exit 1; }
command -v python3 >/dev/null || { echo "ERROR: python3 required." >&2; exit 1; }

RESULTS_PR_SLUG="$(
    python3 - <<PY
import re, os
u = os.environ.get("RESULTS_PR_REPO", "")
m = re.search(r"github\.com[:/]([^/]+/[^/.]+)", u)
print(m.group(1) if m else "")
PY
)"
[[ -n "${RESULTS_PR_SLUG}" ]] || {
    echo "ERROR: could not parse owner/repo from RESULTS_PR_REPO=${RESULTS_PR_REPO}" >&2
    exit 1
}

WORK="$(mktemp -d "${TMPDIR:-/tmp}/perf-results-pr.XXXXXX")"
cleanup() { rm -rf "${WORK}"; }
trap cleanup EXIT

echo "==> Sparse-clone ${RESULTS_PR_REPO}@${RESULTS_PR_BASE} ($(dirname "${RESULTS_PR_README}"))"
git clone --depth 1 --filter=tree:0 --sparse \
    --branch "${RESULTS_PR_BASE}" \
    "${RESULTS_PR_REPO}" "${WORK}/repo"
git -C "${WORK}/repo" sparse-checkout set --cone "$(dirname "${RESULTS_PR_README}")"

README_PATH="${WORK}/repo/${RESULTS_PR_README}"
mkdir -p "$(dirname "${README_PATH}")"
[[ -f "${README_PATH}" ]] || printf '%s\n' '# API Platform Gateway Performance Results' >"${README_PATH}"

META_FILE="${WORK}/meta.env"
{
    echo "TEST_ID=${TEST_ID}"
    echo "DATE_UTC=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "GATEWAY_HELM_CHART_VERSION=${GATEWAY_HELM_CHART_VERSION:-}"
    echo "GATEWAY_NODE_INSTANCE_TYPE=${GATEWAY_NODE_INSTANCE_TYPE:-}"
    echo "GATEWAY_RUNTIME_CPU_LIMIT=${GATEWAY_RUNTIME_CPU_LIMIT:-}"
    echo "GATEWAY_RUNTIME_MEM_LIMIT=${GATEWAY_RUNTIME_MEM_LIMIT:-}"
    echo "ROUTER_CONCURRENCY=${ROUTER_CONCURRENCY:-}"
    echo "GOMAXPROCS=${GOMAXPROCS:-}"
    echo "GOGC=${GOGC:-}"
    echo "GOMEMLIMIT=${GOMEMLIMIT:-}"
    echo "RUN_PERF_OPTS=${RUN_PERF_OPTS:-}"
} >"${META_FILE}"

echo "==> Format CSV → markdown and update ${RESULTS_PR_README}"
RESULTS_PR_REPO="${RESULTS_PR_REPO}" SUMMARY_CSV="${SUMMARY_CSV}" README_PATH="${README_PATH}" META_FILE="${META_FILE}" \
python3 <<'PY'
import csv, os
from pathlib import Path

csv_path = Path(os.environ["SUMMARY_CSV"])
readme_path = Path(os.environ["README_PATH"])
meta_path = Path(os.environ["META_FILE"])
meta = {}
for line in meta_path.read_text().splitlines():
    if "=" in line:
        k, _, v = line.partition("=")
        meta[k] = v

keep = [
    "Scenario Name",
    "Concurrent Users",
    "Throughput (Requests/sec)",
    "Average Response Time (ms)",
    "Error %",
    "90th Percentile of Response Time (ms)",
    "99th Percentile of Response Time (ms)",
    "# Samples",
]
short = {
    "Scenario Name": "Scenario",
    "Concurrent Users": "Users",
    "Throughput (Requests/sec)": "Throughput",
    "Average Response Time (ms)": "Avg Response Time(ms)",
    "Error %": "Err %",
    "90th Percentile of Response Time (ms)": "p90 (ms)",
    "99th Percentile of Response Time (ms)": "p99 (ms)",
    "# Samples": "Samples",
}

with csv_path.open(newline="") as f:
    rows = list(csv.DictReader(f))
if not rows:
    raise SystemExit("summary CSV has no data rows")

headers = [h for h in keep if h in rows[0]] or list(rows[0].keys())[:8]

def cell(v: str) -> str:
    return (v or "").strip().replace("|", "\\|")

lines = [
    f"### Test ID `{meta.get('TEST_ID', '')}`",
    "",
    f"- **UTC:** `{meta.get('DATE_UTC', '')}`",
]
if meta.get("GATEWAY_HELM_CHART_VERSION"):
    lines.append(f"- **Chart:** `{meta['GATEWAY_HELM_CHART_VERSION']}`")
if meta.get("GATEWAY_NODE_INSTANCE_TYPE"):
    lines.append(f"- **Gateway node:** `{meta['GATEWAY_NODE_INSTANCE_TYPE']}`")
cpu, mem = meta.get("GATEWAY_RUNTIME_CPU_LIMIT", ""), meta.get("GATEWAY_RUNTIME_MEM_LIMIT", "")
if cpu or mem:
    lines.append(f"- **Runtime resources:** CPU `{cpu or 'n/a'}` / mem `{mem or 'n/a'}`")
tune = [
    f"`{label}={meta[k]}`"
    for k, label in (
        ("ROUTER_CONCURRENCY", "ROUTER_CONCURRENCY"),
        ("GOMAXPROCS", "GOMAXPROCS"),
        ("GOGC", "GOGC"),
        ("GOMEMLIMIT", "GOMEMLIMIT"),
    )
    if meta.get(k)
]
if tune:
    lines.append("- **Tuning:** " + ", ".join(tune))
if meta.get("RUN_PERF_OPTS"):
    lines.append(f"- **RUN_PERF_OPTS:** `{meta['RUN_PERF_OPTS']}`")
lines += [
    "",
    "| " + " | ".join(short.get(h, h) for h in headers) + " |",
    "| " + " | ".join("---" for _ in headers) + " |",
]
for row in rows:
    lines.append("| " + " | ".join(cell(row.get(h, "")) for h in headers) + " |")
lines.append("")
lines.append("---")
lines.append("")

new_section = "\n".join(lines)
start, end = "<!-- PERF_RESULTS_START -->", "<!-- PERF_RESULTS_END -->"
text = readme_path.read_text() if readme_path.exists() else "# API Platform Gateway Performance Results\n"

# Append (newest first): insert this run after START, keep prior runs below.
if start in text and end in text:
    pre, rest = text.split(start, 1)
    existing, post = rest.split(end, 1)
    existing = existing.strip("\n")
    # Skip if this TEST_ID was already published (idempotent re-run).
    tid = meta.get("TEST_ID", "")
    if tid and f"### Test ID `{tid}`" in existing:
        print(f"Test ID {tid} already in README — leaving unchanged")
    else:
        combined = new_section + (existing + "\n" if existing else "")
        text = pre.rstrip() + "\n\n" + start + "\n" + combined + end + "\n" + post.lstrip("\n")
        readme_path.write_text(text)
        print(f"Appended results for {tid} to {readme_path} ({len(rows)} row(s))")
else:
    if not text.strip().startswith("#"):
        text = "# API Platform Gateway Performance Results\n\n" + text
    text = text.rstrip() + "\n\n" + start + "\n" + new_section + end + "\n"
    readme_path.write_text(text)
    print(f"Created results block in {readme_path} ({len(rows)} row(s))")
PY

cd "${WORK}/repo"
git config user.name "${GIT_AUTHOR_NAME:-jenkins-perf}"
git config user.email "${GIT_AUTHOR_EMAIL:-jenkins-perf@users.noreply.github.com}"
git checkout -B "${RESULTS_PR_BRANCH}"
git add "${RESULTS_PR_README}"
if git diff --cached --quiet; then
    echo "No README changes — nothing to publish."
    exit 0
fi

git commit -m "$(cat <<EOF
docs(perf): append Jenkins results ${TEST_ID}

Append API Gateway EKS summary for ${TEST_ID} to ${RESULTS_PR_README}.
EOF
)"

echo "==> Push ${RESULTS_PR_BRANCH} → ${RESULTS_PR_SLUG}"
git push -u origin "HEAD:${RESULTS_PR_BRANCH}"

echo "==> Open PR against ${RESULTS_PR_BASE}"
PR_URL="$(
    gh pr list --repo "${RESULTS_PR_SLUG}" --head "${RESULTS_PR_BRANCH}" --json url -q '.[0].url' 2>/dev/null || true
)"
if [[ -z "${PR_URL}" ]]; then
    PR_URL="$(gh pr create \
        --repo "${RESULTS_PR_SLUG}" \
        --base "${RESULTS_PR_BASE}" \
        --head "${RESULTS_PR_BRANCH}" \
        --title "(Automated PR): Performance test results for jenkins perf job ${TEST_ID}" \
        --body "$(cat <<EOF
## Summary
- Appends formatted API Gateway EKS results for \`${TEST_ID}\` to \`${RESULTS_PR_README}\`.
- Newest run is inserted at the top of the results block; prior runs are kept.

## Notes
- Table is a readable subset of \`summary.csv\` (Scenario, Users, TPS, latency, errors, percentiles).
- Results live between \`<!-- PERF_RESULTS_START -->\` and \`<!-- PERF_RESULTS_END -->\`.
EOF
)" )"
fi

echo "PR: ${PR_URL}"
if [[ -n "${JENKINS_JOB_WORKSPACE:-}" ]]; then
    echo "${PR_URL}" >"${JENKINS_JOB_WORKSPACE}/results-pr-url.txt"
fi
