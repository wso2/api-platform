#!/usr/bin/env bash
set -euo pipefail

# ---------------- containers ----------------
ROUTER_CTN="gateway-router-1"
CONTROLLER_CTN="gateway-gateway-controller-1"
POLICY_CTN="gateway-policy-engine-1"

# ---------------- API endpoint ----------------
API_MGR_URL="http://localhost:9090/apis"

# ---------------- params ----------------
TOTAL="${1:-50}"
OUT="${2:-stats.csv}"

# ---------------- CSV header ----------------
echo "api_count,router_cpu_pct,router_mem_used,controller_cpu_pct,controller_mem_used,policy_cpu_pct,policy_mem_used,http_code" > "$OUT"

# ---------------- helper: docker stats ----------------
get_stats() {
  local ctn="$1"
  docker stats --no-stream --format "{{.CPUPerc}},{{.MemUsage}}" "$ctn" 2>/dev/null \
  | awk -F',' '{
      split($2, a, " / ");
      gsub(/^[ \t]+|[ \t]+$/, "", $1);
      gsub(/^[ \t]+|[ \t]+$/, "", a[1]);
      print $1 "," a[1]
    }'
}

# ---------------- main loop ----------------
for i in $(seq 1 "$TOTAL"); do
  echo "Creating API $i ..."

  api_name="weather-api-$(printf "%04d" "$i")"
  api_context="/weather/v1.0/$i"

  resp_file="$(mktemp)"

  http_code=$(
    curl -s -o "$resp_file" -w "%{http_code}" \
      -X POST "$API_MGR_URL" \
      -u admin:admin \
      -H "Content-Type: application/yaml" \
      --data-binary @- <<EOF
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: ${api_name}
spec:
  displayName: ${api_name}
  version: v1.0
  context: ${api_context}
  upstream:
    main:
      url: http://netty:8688
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active1
    - method: GET
      path: /alerts/active2
    - method: GET
      path: /alerts/active3
    - method: GET
      path: /alerts/active4
    - method: GET
      path: /alerts/active5
    - method: GET
      path: /alerts/active6
    - method: GET
      path: /alerts/active7
EOF
  )

  if [[ "$http_code" == "409" ]]; then
    echo "❌ 409 Conflict while creating API $i"
    echo "Response:"
    cat "$resp_file"
    echo
    rm -f "$resp_file"
    break
  fi

  rm -f "$resp_file"

  # ---- capture stats ----
  router_line="$(get_stats "$ROUTER_CTN" || echo "NA,NA")"
  ctrl_line="$(get_stats "$CONTROLLER_CTN" || echo "NA,NA")"
  pol_line="$(get_stats "$POLICY_CTN" || echo "NA,NA")"

  router_cpu="${router_line%%,*}"
  router_mem="${router_line#*,}"

  ctrl_cpu="${ctrl_line%%,*}"
  ctrl_mem="${ctrl_line#*,}"

  pol_cpu="${pol_line%%,*}"
  pol_mem="${pol_line#*,}"

  echo "$i,$router_cpu,$router_mem,$ctrl_cpu,$ctrl_mem,$pol_cpu,$pol_mem,$http_code" >> "$OUT"

  if [[ "$http_code" != "200" && "$http_code" != "201" && "$http_code" != "202" ]]; then
    echo "❌ API create failed at count=$i (HTTP $http_code). Stopping."
    break
  fi
done

echo "✅ Done. CSV written to: $OUT"
