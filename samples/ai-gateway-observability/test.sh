#!/usr/bin/env bash
# Verifies the observability pipeline end to end: the gateway exposes metrics,
# Prometheus is actually scraping them, the metrics carry the per-proxy labels the
# dashboard needs, Grafana has the dashboard, and Jaeger has traces.
#
# Run ./setup.sh and ./load.sh first; with no traffic there is nothing to prove.
set -uo pipefail

HEALTH_URL="${HEALTH_URL:-http://localhost:9094/health}"
CONTROLLER_METRICS="${CONTROLLER_METRICS:-http://localhost:9011/metrics}"
POLICY_ENGINE_METRICS="${POLICY_ENGINE_METRICS:-http://localhost:9003/metrics}"
ENVOY_METRICS="${ENVOY_METRICS:-http://localhost:9901/stats/prometheus}"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://localhost:9092}"
JAEGER_URL="${JAEGER_URL:-http://localhost:16686}"
GRAFANA_URL="${GRAFANA_URL:-http://localhost:3000}"
GRAFANA_AUTH="${GRAFANA_AUTH:-admin:admin}"

# Proxy names from llm-proxy-assistant.yaml and llm-proxy-support.yaml.
EXPECTED_PROXIES=("assistant-proxy" "support-proxy")

GREEN="\033[0;32m"; RED="\033[0;31m"; BLUE="\033[0;34m"; NC="\033[0m"
pass() { printf '%b[PASS]%b %s\n' "${GREEN}" "${NC}" "$*"; }
fail() { printf '%b[FAIL]%b %s\n' "${RED}" "${NC}" "$*"; FAILURES=$(( FAILURES + 1 )); }
info() { printf '%b[INFO]%b %s\n' "${BLUE}" "${NC}" "$*"; }

FAILURES=0

echo ""
echo "══════════════════════════════════════════════════"
echo " Pre-flight checks"
echo "══════════════════════════════════════════════════"

if ! command -v jq >/dev/null 2>&1; then
  printf '%b[ERROR]%b jq is required (macOS: brew install jq, Debian/Ubuntu: apt install jq)\n' "${RED}" "${NC}" >&2; exit 1
fi

info "Checking gateway health at ${HEALTH_URL} ..."
if ! curl -sf --connect-timeout 5 --max-time 10 "${HEALTH_URL}" >/dev/null 2>&1; then
  printf '%b[ERROR]%b Gateway is not running. Run ./setup.sh first.\n' "${RED}" "${NC}" >&2; exit 1
fi
pass "Gateway is healthy."

# ---------------------------------------------------------------------------
# Test 1: every component exposes a metrics endpoint
# ---------------------------------------------------------------------------
echo ""
echo "══════════════════════════════════════════════════"
echo " Test 1: Metrics endpoints respond"
echo "══════════════════════════════════════════════════"

# Asserts an endpoint answers HTTP 200, recording a pass or a failure.
check_endpoint() {
  local label="$1" url="$2"
  local status
  status=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 --max-time 15 "${url}")
  if [[ "${status}" == "200" ]]; then
    pass "${label}: HTTP 200 (${url})"
  else
    fail "${label}: expected HTTP 200, got ${status} (${url})"
  fi
}

check_endpoint "Gateway controller" "${CONTROLLER_METRICS}"
check_endpoint "Policy engine"      "${POLICY_ENGINE_METRICS}"
check_endpoint "Envoy router"       "${ENVOY_METRICS}"

# ---------------------------------------------------------------------------
# Test 2: Prometheus is scraping all of them
# ---------------------------------------------------------------------------
echo ""
echo "══════════════════════════════════════════════════"
echo " Test 2: Prometheus scrape targets are up"
echo "══════════════════════════════════════════════════"

TARGETS=$(curl -s --connect-timeout 5 --max-time 15 "${PROMETHEUS_URL}/api/v1/targets" 2>/dev/null)
if [[ -z "${TARGETS}" ]] || ! jq -e '.data.activeTargets' >/dev/null 2>&1 <<< "${TARGETS}"; then
  fail "Could not read scrape targets from ${PROMETHEUS_URL}/api/v1/targets"
else
  DOWN=$(jq -r '.data.activeTargets[] | select(.health != "up") | "\(.labels.job) (\(.scrapeUrl)): \(.lastError)"' <<< "${TARGETS}")
  UP_COUNT=$(jq -r '[.data.activeTargets[] | select(.health == "up")] | length' <<< "${TARGETS}")
  if [[ -z "${DOWN}" ]]; then
    pass "All ${UP_COUNT} scrape targets are up."
  else
    fail "Some scrape targets are down:"
    printf '        %s\n' "${DOWN}"
  fi
fi

# ---------------------------------------------------------------------------
# Test 3: the metrics carry the per-proxy label the dashboard groups by
#
# Only the request_headers phase records api_name/api_version; the later phases
# report empty strings. Every per-proxy panel therefore filters on that phase.
# ---------------------------------------------------------------------------
echo ""
echo "══════════════════════════════════════════════════"
echo " Test 3: Per-proxy request metrics exist"
echo "══════════════════════════════════════════════════"

QUERY='sum by (api_name) (policy_engine_requests_total{phase="request_headers"})'
RESULT=$(curl -s --connect-timeout 5 --max-time 15 --get "${PROMETHEUS_URL}/api/v1/query" --data-urlencode "query=${QUERY}" 2>/dev/null)

if ! jq -e '.status == "success"' >/dev/null 2>&1 <<< "${RESULT}"; then
  fail "Prometheus query failed. Is Prometheus running on ${PROMETHEUS_URL}?"
else
  PROXIES=$(jq -r '.data.result[] | select(.metric.api_name != "" and .metric.api_name != null) | "\(.metric.api_name)=\(.value[1])"' <<< "${RESULT}")
  # Both proxies must report. The dashboard's per-proxy panels are the point of the
  # sample, so one proxy answering while the other is silent is a failure, not a pass.
  MISSING=""
  for proxy in "${EXPECTED_PROXIES[@]}"; do
    grep -q "^${proxy}=" <<< "${PROXIES}" || MISSING="${MISSING} ${proxy}"
  done
  if [[ -z "${PROXIES}" ]]; then
    fail "No per-proxy request counts yet. Run ./load.sh to generate traffic."
  elif [[ -n "${MISSING}" ]]; then
    fail "Missing request counts for:${MISSING}"
    echo "        Found instead:"
    while IFS= read -r line; do printf '        %s\n' "${line}"; done <<< "${PROXIES}"
  else
    pass "Per-proxy request counts found for both proxies:"
    while IFS= read -r line; do printf '        %s\n' "${line}"; done <<< "${PROXIES}"
  fi
fi

# ---------------------------------------------------------------------------
# Test 4: Grafana provisioned the dashboard
# ---------------------------------------------------------------------------
echo ""
echo "══════════════════════════════════════════════════"
echo " Test 4: Grafana dashboard is provisioned"
echo "══════════════════════════════════════════════════"

SEARCH=$(curl -s --connect-timeout 5 --max-time 15 -u "${GRAFANA_AUTH}" "${GRAFANA_URL}/api/search?query=AI%20Gateway" 2>/dev/null)
if jq -e 'type == "array" and length > 0' >/dev/null 2>&1 <<< "${SEARCH}"; then
  TITLE=$(jq -r '.[0].title' <<< "${SEARCH}")
  URI=$(jq -r '.[0].url' <<< "${SEARCH}")
  pass "Dashboard '${TITLE}' is loaded: ${GRAFANA_URL}${URI}"
else
  fail "Dashboard not found in Grafana. Check: docker compose logs grafana"
fi

# ---------------------------------------------------------------------------
# Test 5: traces reached Jaeger
# ---------------------------------------------------------------------------
echo ""
echo "══════════════════════════════════════════════════"
echo " Test 5: Traces are visible in Jaeger"
echo "══════════════════════════════════════════════════"

SERVICES=$(curl -s --connect-timeout 5 --max-time 15 "${JAEGER_URL}/api/services" 2>/dev/null)
SERVICE_LIST=$(jq -r '.data[]? // empty' <<< "${SERVICES}" 2>/dev/null | grep -v '^jaeger' || true)

if [[ -z "${SERVICE_LIST}" ]]; then
  fail "Jaeger knows no gateway services yet. Run ./load.sh, then retry."
else
  info "Services reporting traces: $(echo "${SERVICE_LIST}" | tr '\n' ' ')"
  TRACED=0
  for service in ${SERVICE_LIST}; do
    COUNT=$(curl -s --connect-timeout 5 --max-time 15 --get "${JAEGER_URL}/api/traces" \
      --data-urlencode "service=${service}" \
      --data-urlencode "limit=1" 2>/dev/null | jq -r '.data | length' 2>/dev/null)
    if [[ "${COUNT:-0}" -gt 0 ]]; then
      pass "Trace found for service '${service}': ${JAEGER_URL}/search?service=${service}"
      TRACED=1
      break
    fi
  done
  [[ "${TRACED}" -eq 1 ]] || fail "No traces stored yet. Run ./load.sh, then retry."
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "══════════════════════════════════════════════════"
if [[ "${FAILURES}" -eq 0 ]]; then
  printf '%b[PASS]%b  Observability pipeline is working end to end.\n' "${GREEN}" "${NC}"
  echo ""
  echo "  Grafana : ${GRAFANA_URL}   (admin / admin)"
  echo "  Jaeger  : ${JAEGER_URL}"
  echo "══════════════════════════════════════════════════"
  echo ""
  exit 0
else
  printf '%b[FAIL]%b  %s check(s) failed.\n' "${RED}" "${NC}" "${FAILURES}"
  echo ""
  echo "Troubleshooting:"
  echo "  - Generate traffic first : ./load.sh"
  echo "  - Are all containers up  : docker ps"
  echo "  - Gateway logs           : cd wso2apip-ai-gateway-1.2.0 && docker compose logs"
  echo "  - Start from clean       : ./teardown.sh && ./setup.sh"
  echo "══════════════════════════════════════════════════"
  echo ""
  exit 1
fi
