# AI Gateway: Metrics and Tracing with a Ready-Made Dashboard

This sample runs the WSO2 AI Gateway with its full observability stack switched on
(Prometheus, Grafana, an OpenTelemetry collector and Jaeger), pointed at two LLM
proxies backed by a mock model. Generate a minute of traffic, and you get a live
dashboard showing request rate, latency and errors per proxy, plus a complete trace of
any single request through the gateway. No API key, no cloud account, nothing to
configure by hand.

The point: once a gateway sits between your applications and an LLM provider, you can
operate that traffic the same way you operate everything else, with standard
Prometheus metrics and OpenTelemetry traces that plug into whatever stack you already
run.

## Prerequisites

- Docker with the Compose plugin
- `curl` (or `wget`), `unzip`, `jq` and `openssl`
- Roughly 2 GB of free memory for the containers

On Windows, run these from a WSL2 shell with Docker Desktop's WSL integration enabled.

The gateway's own `scripts/setup.sh` bcrypts the admin password using `htpasswd` when
it is installed, and falls back to a throwaway `httpd` container otherwise, so Docker
alone is enough.

## Getting started

```bash
./setup.sh     # downloads the gateway, starts everything, registers the proxies
./load.sh      # ~1 minute of mixed traffic
```

Then open the two URLs the scripts print:

| URL | What you see |
|-----|--------------|
| <http://localhost:3000> | **Grafana**: the AI Gateway Overview dashboard, live (admin / admin) |
| <http://localhost:16686> | **Jaeger**: pick the `router` service, open any trace |

## What to look for

**In Grafana**, the dashboard opens on the AI Gateway Overview. Four tiles across the
top carry the peaks over the window (requests/sec, worst p95 end-to-end, gateway
faults/sec, policy rejections/sec), and six charts break them down:

- *Request rate per proxy*: two lines, `assistant-proxy` and `support-proxy`.
- *Gateway processing time per route*: p50 and p95 of the time the policy engine itself
  spent on a request. This is the gateway's own overhead.
- *End-to-end latency*: p50 and p95 of the full round trip, including the model. The gap
  between this and the chart above is time spent waiting on the upstream.
- *Responses by status class*: 2xx alongside the 4xx and 5xx that `load.sh` mixes in.
- *Policy rejections by policy*: `api-key-auth` rejecting bad keys throughout, and
  `token-based-ratelimit` appearing part-way through the run when the support proxy
  spends its token budget.
- *Upstream failures*: 5xx responses coming back from the model backend.

**In Jaeger**, open a trace and you get one request's journey as a waterfall. The charts
above give these times in aggregate; a trace gives them for a single call, which is what
you want when one request was slow and you need to know where the time went.

## Verify from the terminal

`test.sh` proves the pipeline rather than describing it: the metrics endpoints respond,
Prometheus is scraping all three of them, the metrics carry the per-proxy labels the
dashboard groups by, Grafana loaded the dashboard, and Jaeger stored traces.

```bash
./test.sh
```

Expected output:

```
══════════════════════════════════════════════════
 Test 1: Metrics endpoints respond
══════════════════════════════════════════════════
[PASS] Gateway controller: HTTP 200 (http://localhost:9011/metrics)
[PASS] Policy engine: HTTP 200 (http://localhost:9003/metrics)
[PASS] Envoy router: HTTP 200 (http://localhost:9901/stats/prometheus)
...
[PASS]  Observability pipeline is working end to end.
```

## How it works

- **Metrics**: the gateway exposes Prometheus endpoints; Prometheus scrapes them every
  15 seconds and Grafana charts them. They answer how much traffic, how fast, how often
  broken. Token usage is not among them; that goes to analytics (Moesif), not metrics.
- **Traces**: the gateway exports OpenTelemetry spans to Jaeger. They answer where a
  single request spent its time.

Both ship with the gateway; `setup.sh` switches them on, so there is nothing to
configure by hand.

The two proxies differ only in a token budget on `/support`. That is what makes the
per-proxy panels worth watching: one keeps serving while the other starts returning 429.

## What's running

| Container | Role | Port |
|-----------|------|------|
| `gateway-controller` | Control plane, where proxies are registered | 9090 (API), 9011 (metrics) |
| `gateway-runtime` | Envoy router + policy engine, where traffic flows | 8080 (HTTP), 9901 (Envoy admin), 9003 (metrics) |
| `mock-llm-openai` | WireMock standing in for the OpenAI API | 8082 |
| `prometheus` | Scrapes and stores the metrics | 9092 |
| `grafana` | Charts them | 3000 |
| `otel-collector` | Receives spans from the gateway | 4317 / 4318 |
| `jaeger` | Stores and displays traces | 16686 |

## Send your own request

```bash
curl -X POST http://localhost:8080/assistant/chat/completions \
  -H "Content-Type: application/json" \
  -H "api_key: demo-assistant-key" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}'
```

Watch it land on the dashboard, then find its trace in Jaeger. The support proxy is at
`/support/chat/completions` with `demo-support-key`.

Ports, credentials, keys and traffic duration are all environment variables at the top
of `setup.sh` and `load.sh`. Override any of them before running.

## Teardown

```bash
./teardown.sh            # delete the proxies, stop the containers, drop the volumes
./teardown.sh --clean    # also remove the extracted distribution and the zip
```

Use `--clean` before a fresh run if you want to be certain the config edits above are
reapplied to a pristine distribution.
