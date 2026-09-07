#!/bin/bash -e
# JMeter EC2 setup: Java 17, JMeter tarball, PERF_HOME symlinks, payloads.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=env.example
source "${SCRIPT_DIR}/env.example"
# shellcheck source=../lib/common.sh
source "${MANUAL_DIR}/lib/common.sh"

install_java17_and_maven
install_linux_packages unzip bc jq

if ! command -v jmeter >/dev/null; then
    JMETER_TGZ="${JMETER_TGZ:-${HOME}/apache-jmeter-5.6.3.tgz}"
    [[ -f "$JMETER_TGZ" ]] || {
        echo "Download JMeter 5.6.3 to ${JMETER_TGZ} or install jmeter on PATH" >&2
        exit 1
    }
    tar -xzf "$JMETER_TGZ" -C "$HOME"
    JMETER_HOME="${HOME}/apache-jmeter-5.6.3"
    cp "${PERF_ROOT}/performance-common/distribution/scripts/jmeter/user.properties" \
        "${JMETER_HOME}/bin/user.properties"
    grep -q 'apache-jmeter-5.6.3/bin' "${HOME}/.bashrc" 2>/dev/null || \
        echo 'export PATH=${HOME}/apache-jmeter-5.6.3/bin:$PATH' >>"${HOME}/.bashrc"
    export PATH="${HOME}/apache-jmeter-5.6.3/bin:$PATH"
fi

# Always refresh user.properties from perf repo so log tuning and RMI settings stay in sync.
JMETER_HOME="${JMETER_HOME:-$(dirname "$(dirname "$(command -v jmeter)")")}"
if [[ -n "${JMETER_HOME}" && -f "${JMETER_HOME}/bin/user.properties" ]]; then
    cp "${PERF_ROOT}/performance-common/distribution/scripts/jmeter/user.properties" \
        "${JMETER_HOME}/bin/user.properties"
fi

COMMON_SCRIPTS="${PERF_ROOT}/performance-common/distribution/scripts"
mkdir -p "$PERF_HOME"
ln -sfn "${MANUAL_DIR}/jmeter" "${PERF_HOME}/jmeter"
ln -sfn "${COMMON_SCRIPTS}/payloads" "${PERF_HOME}/payloads"
ln -sfn "${COMMON_SCRIPTS}/jtl-splitter" "${PERF_HOME}/jtl-splitter"
ln -sfn "${COMMON_SCRIPTS}/common" "${PERF_HOME}/common"
ln -sfn "${PERF_HOME}/jmeter" "${HOME}/jmeter"
ln -sfn "${PERF_HOME}/payloads" "${HOME}/payloads"
ln -sfn "${PERF_HOME}/jtl-splitter" "${HOME}/jtl-splitter"

if [[ "${SKIP_PAYLOAD_GENERATION:-true}" != "true" ]]; then
    (
        cd "$PERF_HOME"
        ./payloads/generate-payloads.sh -a -s 1024 -s 10240
    )
    cp "${PERF_HOME}"/ai_*.json "${HOME}/" 2>/dev/null || true
fi

echo "JMeter setup OK. PERF_HOME=${PERF_HOME}"
echo "Next: source env.jmeter && ./run-scenario.sh -g api-gateway ..."
