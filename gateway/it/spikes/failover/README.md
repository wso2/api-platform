# Stage 0 failover probes

Hand-written Envoy configs that answer the make-or-break questions for the two
model-failover mechanisms, with no gateway code involved.

Results: [`specs/001-model-failover-policy/poc-results.md`](../../../../specs/001-model-failover-policy/poc-results.md)

## Running

```bash
# Option A — Override Host on the Envoy we ship today
A_FAIL=true C_FAIL=true ENVOY_CONFIG=a1-ordering.yaml docker compose up -d
curl -s localhost:10000/ -H "x-failover-targets: 172.28.0.11:8080,172.28.0.13:8080,172.28.0.12:8080" -d '{}'

# Option A — internal listener addressing (the decisive probe)
ENVOY_CONFIG=a4-internal-listener.yaml docker compose up -d --force-recreate
curl -s localhost:10000/ -H "x-failover-targets: envoy://int-provider-a/,envoy://int-provider-b/" -d '{}'

# Option B — priority_group, needs Envoy 1.40.0
ENVOY_TAG=v1.40.0 ENVOY_CONFIG=b1-refresh-on.yaml docker compose up -d --force-recreate
curl -s localhost:10000/ -H "x-failover-groups: c,a,b" -d '{}'

# Probe D — per-attempt upstream filter (the question the IT run left open)
ENVOY_TAG=v1.40.0 A_FAIL=true ENVOY_CONFIG=d1-upstream-filter.yaml docker compose up -d --force-recreate
curl -s localhost:10000/chat/completions -d '{}'          # D1: does it run per attempt, can it mutate :path?
ENVOY_TAG=v1.40.0 A_FAIL=true ENVOY_CONFIG=d2-attempt-indexed-rewrite.yaml docker compose up -d --force-recreate envoy
curl -s localhost:10000/chat/completions -d '{}'          # D2: the validated pattern
docker compose logs envoy | grep PROBE

docker compose down
```

Discover the address strings Envoy matches against by running with `ENVOY_LOG=trace`
and grepping for `Looking up`.

| File | Probes |
|---|---|
| `a1-ordering.yaml` | A1 ordering, A2 traversal, A3 overrun |
| `a4-internal-listener.yaml` | A4 internal-listener addressing |
| `b1-priority-group.yaml` | template; `REFRESH_FLAG` substituted by the variants below |
| `b1-refresh-on.yaml` | B1 walking, B2 override, B4 overrun |
| `b3-refresh-off.yaml` | B3 missing `refresh_cluster_on_retry` |
| `c-body-a.yaml` / `c-body-b.yaml` | C body-mutation preservation |

`backend.py` is a fake provider: it echoes the body and auth header it received, and
returns 500 when `ALWAYS_FAIL=true`.
