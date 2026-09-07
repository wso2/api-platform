# shellcheck shell=bash
# Load env.jmeter.<ai|api> (or .example) after PERF_GATEWAY_TYPE is set.

load_jmeter_gateway_profile() {
    local script_dir="$1"
    local profile="${script_dir}/env.jmeter.${PERF_GATEWAY_TYPE%-gateway}"
    if [[ -f "$profile" ]]; then
        # shellcheck source=/dev/null
        source "$profile"
    elif [[ -f "${profile}.example" ]]; then
        # shellcheck source=/dev/null
        source "${profile}.example"
    else
        echo "Missing ${profile} (and no .example). Copy env.jmeter.api.example to env.jmeter.api" >&2
        exit 1
    fi
}
