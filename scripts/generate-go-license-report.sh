#!/usr/bin/env bash
# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

set -euo pipefail

REPO_ROOT="$(pwd)"
REPORT_DIR="${1:-license-reports/go}"
GO_LICENSES_BIN="${GO_LICENSES_BIN:-go-licenses}"
export GOCACHE="${GOCACHE:-${TMPDIR:-/tmp}/api-platform-go-cache}"
export GOMODCACHE="${GOMODCACHE:-${TMPDIR:-/tmp}/api-platform-go-mod-cache}"
GO_LICENSES_GOWORK="${GO_LICENSES_GOWORK:-off}"
GO_LICENSES_GOFLAGS="${GO_LICENSES_GOFLAGS:--mod=mod}"
INTERNAL_LICENSE_PREFIXES="${INTERNAL_LICENSE_PREFIXES:-github.com/wso2/,github.com/policy-engine/,platform-api/}"

if [[ "${REPORT_DIR}" != /* ]]; then
    REPORT_DIR="${REPO_ROOT}/${REPORT_DIR}"
fi

if ! command -v "${GO_LICENSES_BIN}" >/dev/null 2>&1; then
    echo "go-licenses was not found. Install it with:" >&2
    echo "  go install github.com/google/go-licenses/v2@latest" >&2
    exit 127
fi

rm -rf "${REPORT_DIR}"
mkdir -p "${REPORT_DIR}"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/go-license-report.XXXXXX")"
trap 'rm -rf "${TMP_DIR}"' EXIT

SUMMARY_REPORT="${REPORT_DIR}/summary.md"
FAILED_MODULES="${REPORT_DIR}/failed-modules.txt"

COMPONENTS=(
    "gateway-controller|gateway/gateway-controller"
    "gateway-runtime|gateway/gateway-runtime/policy-engine"
    "gateway-builder|gateway/gateway-builder"
    "platform-api|platform-api"
)

overall_status=0

for component in "${COMPONENTS[@]}"; do
    report_name="${component%%|*}"
    module_dir="${component#*|}"
    go_mod="${module_dir}/go.mod"

    if [ ! -f "${go_mod}" ]; then
        overall_status=1
        echo "${module_dir}" >> "${FAILED_MODULES}"
        echo "Skipping ${report_name}; ${go_mod} was not found" >&2
        continue
    fi

    module_name="$(awk '/^module[[:space:]]+/ { print $2; exit }' "${go_mod}")"
    csv_report="${REPORT_DIR}/${report_name}-third-party-go-licenses.csv"
    raw_csv_report="${TMP_DIR}/${report_name}.raw.csv"
    log_report="${TMP_DIR}/${report_name}.log"
    module_backup_dir="${TMP_DIR}/${report_name}"
    mkdir -p "${module_backup_dir}"
    cp "${module_dir}/go.mod" "${module_backup_dir}/go.mod"
    if [ -f "${module_dir}/go.sum" ]; then
        cp "${module_dir}/go.sum" "${module_backup_dir}/go.sum"
        had_go_sum=1
    else
        had_go_sum=0
    fi

    echo "Generating ${report_name} Go license report for ${module_name} (${module_dir})"

    if (
        cd "${module_dir}"
        ignore_args=()
        while IFS= read -r std_package; do
            ignore_args+=(--ignore "${std_package}")
        done < <(GOWORK="${GO_LICENSES_GOWORK}" go list std)
        GOWORK="${GO_LICENSES_GOWORK}" GOFLAGS="${GO_LICENSES_GOFLAGS}" "${GO_LICENSES_BIN}" report "${ignore_args[@]+"${ignore_args[@]}"}" ./... > "${raw_csv_report}" 2> "${log_report}"
    ); then
        awk -F ',' -v prefixes="${INTERNAL_LICENSE_PREFIXES}" '
            BEGIN {
                split(prefixes, prefix_list, ",")
            }
            {
                for (idx in prefix_list) {
                    if (prefix_list[idx] != "" && index($1, prefix_list[idx]) == 1) {
                        next
                    }
                }
                print
            }
        ' "${raw_csv_report}" > "${csv_report}"
    else
        overall_status=1
        echo "${module_dir}" >> "${FAILED_MODULES}"
        cp "${log_report}" "${REPORT_DIR}/${report_name}.log"
        echo "Failed to generate ${report_name} Go license report for ${module_name}; see ${REPORT_DIR}/${report_name}.log" >&2
    fi

    cp "${module_backup_dir}/go.mod" "${module_dir}/go.mod"
    if [ "${had_go_sum}" -eq 1 ]; then
        cp "${module_backup_dir}/go.sum" "${module_dir}/go.sum"
    else
        rm -f "${module_dir}/go.sum"
    fi
done

{
    echo "# Third-party Go License Report"
    echo
    echo "- Generated at: $(date -u +'%Y-%m-%dT%H:%M:%SZ')"
    echo "- Components scanned: ${#COMPONENTS[@]}"
    echo "- Internal prefixes ignored: ${INTERNAL_LICENSE_PREFIXES}"
    echo
    echo "## Reports"
    echo
    for component in "${COMPONENTS[@]}"; do
        report_name="${component%%|*}"
        module_dir="${component#*|}"
        go_mod="${module_dir}/go.mod"
        if [ -f "${go_mod}" ]; then
            module_name="$(awk '/^module[[:space:]]+/ { print $2; exit }' "${go_mod}")"
            echo "- ${report_name}: ${report_name}-third-party-go-licenses.csv (${module_name})"
        else
            echo "- ${report_name}: missing ${go_mod}"
        fi
    done
    echo
    echo "## License Counts"
    echo
    for component in "${COMPONENTS[@]}"; do
        report_name="${component%%|*}"
        csv_report="${REPORT_DIR}/${report_name}-third-party-go-licenses.csv"
        if [ -f "${csv_report}" ]; then
            echo "### ${report_name}"
            awk -F ',' 'NF >= 3 { counts[$3]++ } END { for (license in counts) print "- " license ": " counts[license] }' "${csv_report}" | sort
            echo
        fi
    done
    if [ -f "${FAILED_MODULES}" ]; then
        echo
        echo "## Failed Modules"
        echo
        sed 's/^/- /' "${FAILED_MODULES}"
    fi
} > "${SUMMARY_REPORT}"

echo "Generated report directory: ${REPORT_DIR}"
exit "${overall_status}"
