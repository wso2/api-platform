# AI Gateway: Metrics and Tracing with a Ready-Made Dashboard

This sample runs the WSO2 AI Gateway with its full observability stack switched on
(Prometheus, Grafana, an OpenTelemetry collector and Jaeger), pointed at two LLM
proxies backed by a mock model. Generate a minute of traffic, and you get a live
dashboard showing request rate, latency and errors per proxy, plus a complete trace of
any single request through the gateway. No API key, no cloud account, nothing to
configure by hand.

## Prerequisites

- Docker with the Compose plugin
- `curl` (or `wget`), `unzip`, `jq` and `openssl`

On Windows, run these from a WSL2 shell with Docker Desktop's WSL integration enabled.

## Getting started

```bash
./setup.sh
```

1. Downloads and extracts the AI Gateway distribution.
2. Enables the Prometheus endpoints and tracing, and provisions the Grafana dashboard.
3. Starts a WireMock container standing in for the OpenAI API.
4. Starts the gateway together with Prometheus, Grafana, Jaeger and the OTel collector.
5. Waits for the gateway to report healthy, then puts the mock on its network.
6. Registers two proxies, `assistant-proxy` and `support-proxy`, each with an inbound
   API key. Only the provider behind `support-proxy` has a token budget.

Credentials, certificates and the environment file the stack needs are generated in
step 4, so there is nothing to configure beforehand.

```bash
./load.sh
```

1. Checks the gateway is running, and stops if it is not.
2. Sends a request every 0.25 seconds for 60 seconds, alternating between the two
   proxies. Pass a different duration as `./load.sh 120`.
3. Cycles through a fixed pattern every ten requests: one answered slowly, one failing
   upstream, one with an invalid key, and seven ordinary ones. The same every run.
4. Counts every response by status code and prints the totals.

Then open the two URLs the scripts print:

| URL | What you see |
|-----|--------------|
| <http://localhost:3000> | **Grafana**: the AI Gateway Overview dashboard, live (admin / admin) |
| <http://localhost:16686> | **Jaeger**: pick the `router` service, open any trace |

## What to look for

**In Grafana**, the dashboard opens on the AI Gateway Overview: four tiles showing peak
values, and six charts.

- *Request rate per proxy*: how much traffic each proxy is handling.
- *Gateway processing time per route*: how long the gateway itself takes, as a typical
  time (p50) and a slow-tail time (p95).
- *End-to-end latency*: how long the whole call takes, including the model.
- *Responses by status class*: how many requests succeeded (2xx) against how many were
  rejected (4xx) or failed (5xx).
- *Policy rejections by policy*: requests the gateway blocked, and which rule blocked
  them. Bad keys throughout, the token budget from part-way through the run.
- *Upstream failures*: requests the model backend itself failed.

**In Jaeger**, open a trace to see one request broken into its steps. The charts show
totals across all traffic; a trace shows where a single slow request lost its time.

## Verify from the terminal

`test.sh` checks the pipeline end to end: the metrics endpoints respond, Prometheus is
scraping all three of them, both proxies report per-proxy metrics, Grafana loaded the
dashboard, and Jaeger stored traces.

```bash
./test.sh
```

Expected output:

```
══════════════════════════════════════════════════
 Pre-flight checks
══════════════════════════════════════════════════
[INFO] Checking gateway health at http://localhost:9094/health ...
[PASS] Gateway is healthy.

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

```
  ./load.sh ──► Gateway :8080 ──► mock LLM (WireMock)
                     │
        ┌────────────┴────────────┐
        │                         │
   Prometheus scrapes        gateway pushes
   metrics every 15s         traces (OpenTelemetry)
        │                         │
        ▼                         ▼
   Grafana :3000             Jaeger :16686
```

- **Metrics** answer how much traffic, how fast, and how often broken. Token usage is
  not among them; that goes to analytics (Moesif), not metrics.
- **Traces** answer where a single request spent its time.

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

`--clean` makes the next `./setup.sh` download and set up the gateway from scratch,
instead of reusing the copy already on disk.
