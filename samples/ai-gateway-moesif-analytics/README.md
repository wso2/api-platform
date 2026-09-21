# AI Gateway: Token Usage and Cost in Moesif

This sample runs the WSO2 AI Gateway with analytics published to
[Moesif](https://www.moesif.com). Every LLM request that passes through the gateway
appears there with the model that served it, its token counts, and its cost.

Two proxies send traffic to two models of very different price, so the gap between
request volume and actual spend is visible immediately.

The integration is one environment variable: the distribution ships with analytics
enabled and Moesif as the publisher, and reads your Application ID from the
environment.

> If you are looking for request rate, latency and traces instead, see
> [`ai-gateway-observability`](../ai-gateway-observability), which runs a self-hosted
> Prometheus and Grafana stack.

## Prerequisites

- Docker with the Compose plugin
- `curl` (or `wget`), `unzip`, `jq` and `openssl`
- A Moesif account (the free tier is sufficient)

On Windows, run the sample from a WSL2 shell with Docker Desktop's WSL integration
enabled.

## Configure your Application ID

1. Sign up or sign in at [Moesif](https://www.moesif.com/).
2. Get your **Collector Application ID**:
   - On a new account, the onboarding wizard shows it while you set up your first
     application. Copy it then.
   - On an existing account, select the account icon, then **Installation** or
     **API Keys**, and copy the **Collector Application ID** field.
3. Provide it to the sample:

   ```bash
   cp .env.example .env
   # add the ID to .env
   ```

   Or export it: `export MOESIF_APPLICATION_ID='<your-application-id>'`

This is the only Moesif credential the sample needs. `.env` is git-ignored, so the
ID you paste in is not committed. For other configuration options, see the
[Moesif documentation](https://www.moesif.com/docs).

## Run

```bash
./setup.sh
```

1. Downloads and extracts the AI Gateway distribution.
2. Confirms analytics is enabled with Moesif as the publisher.
3. Configures cost pricing for the `llm-cost` policy.
4. Starts a WireMock container as the model backend, and the gateway with your
   Application ID.
5. Registers the provider, two proxies, and an inbound API key on each.

It finishes by printing the two proxies it registered:

| Proxy | Endpoint | Model | API key |
|-------|----------|-------|---------|
| Assistant | `/assistant/chat/completions` | `gpt-4o-mini` | `demo-assistant-key` |
| Support | `/support/chat/completions` | `gpt-4.1` | `demo-support-key` |

Both share one provider, `mock-openai-provider`, pointing at the WireMock backend.
`gpt-4.1` costs roughly fifteen times more per token than `gpt-4o-mini`, which is
what makes the cost comparison worth looking at.

```bash
./load.sh
```

Repeats a fixed ten-request cycle for 60 seconds:

| Requests | Proxy | Outcome |
|----------|-------|---------|
| 4 | Assistant | 200 |
| 4 | Support | 200 |
| 1 | Assistant | 500, returned by the mock backend |
| 1 | Assistant | 401, invalid API key rejected at the gateway |

Pass a duration in seconds to change the run length: `./load.sh <seconds>`.

It prints the token totals per model when it finishes. Open
<https://www.moesif.com> to compare, selecting the time range you want.

## What to look for in Moesif

Moesif groups analytics into four dashboards in the left sidebar: Overview, APIs,
LLM and MCP. This sample publishes LLM proxy traffic, so open **LLM**.

The page is prebuilt, so there is nothing to configure. What each panel holds after
a run:

| Panel | What this sample puts in it |
|-------|-----------------------------|
| Token Usage, Estimated Cost | token totals and estimated spend for the selected time range |
| AI API Details | token usage and request count, per proxy |
| Traffic Share by Provider and Model | how requests divide between `gpt-4o-mini` and `gpt-4.1` |
| Cost Trend per Provider | spend accumulating across runs |
| Latency Trend | P95 latency per model |
| Average Error Rate, Error Type Breakdown | the 500s and 401s from the cycle above |

In **AI API Details** you can observe the split the sample is built to show: the
support proxy receives fewer requests than the assistant proxy, yet reports around
four times the tokens and far more of the cost. `gpt-4.1` returns longer responses
and costs about fifteen times more per token, so it accounts for roughly 80% of the
tokens and 98% of the spend while serving the smaller share of traffic.

Compare that against **Traffic Share by Provider and Model**, where the two models
sit much closer together. Request counts and spend tell different stories, and
per-proxy, per-model attribution is what lets you see the difference.

## Verify locally

```bash
./test.sh
```

Checks that:

1. Analytics is enabled with Moesif as a publisher.
2. The Application ID reached the gateway, and is not the placeholder the config
   falls back to. Without this the gateway reports healthy while publishing nothing.
3. Responses through both proxies carry token usage on the expected model.
4. Requests without an API key are rejected before reaching the model.

This covers everything up to the point where events leave the gateway. Open the LLM
dashboard in Moesif to confirm they arrived.

## Troubleshooting

The gateway writes its startup and error messages to the container log. Nothing extra
is configured for this; the logs are there by default:

```bash
cd wso2apip-ai-gateway-1.2.0
docker compose logs gateway-runtime        # router and policy engine
docker compose logs gateway-controller     # resource registration
cd ..
docker logs mock-llm-openai                # model backend
```

Add `-f` to follow a log as requests arrive, or `--tail 100` to see only the most
recent lines. The policy engine reports each policy it loads at startup, which is the
first place to look if cost or token fields are missing from the events.

## How it works

```
  ./load.sh ──► AI Gateway :8080 ──► mock model (WireMock)
                       │
                       │  reads usage from the response,
                       │  prices it, builds the analytics event
                       ▼
                    Moesif
```

No real model provider is involved. The backend is
[WireMock](https://wiremock.org), an HTTP server that returns canned responses
matched against the incoming request. The mappings in `wiremock/mappings/` match on
the `model` field and reply with an OpenAI-shaped chat completion carrying fixed
token counts, so every run produces the same numbers and the sample costs nothing to
run. One mapping returns a 500 when the prompt contains `FAIL`, which is where the
upstream failures in the cycle come from.

Three parts produce the token and cost fields:

- **Token counts** come from the model's own `usage` block. No configuration is
  required; a backend that returns no usage block produces events without tokens.
- **Cost** comes from the `llm-cost` policy attached to each proxy, which prices the
  response using the pricing file bundled with the distribution. The policy reads that
  file's path from `policy_configurations.llm_cost_v1`, which `setup.sh` appends to the
  gateway configuration. Attach the policy to a proxy rather than to the provider: only
  the proxy's event reaches analytics, so a cost calculated on the provider is not
  reported.
- **Delivery** is handled by the Moesif publisher, already enabled in the shipped
  configuration. It batches events and flushes on a short interval, so they appear
  within seconds.

## Containers

| Container | Role | Port |
|-----------|------|------|
| `gateway-controller` | control plane, where resources are registered | 9090 |
| `gateway-runtime` | router and policy engine, where traffic flows | 8080, 9901 |
| `mock-llm-openai` | WireMock model backend | 8082 |

Analytics are stored in Moesif, so the sample runs no local database or dashboard.

## Send a single request

```bash
curl -X POST http://localhost:8080/assistant/chat/completions \
  -H "Content-Type: application/json" \
  -H "api_key: demo-assistant-key" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}'
```

Use `gpt-4.1` to see a different cost on the event. The support proxy is at
`/support/chat/completions` with `demo-support-key`.

Ports, credentials, API keys and traffic duration are environment variables declared
at the top of `setup.sh` and `load.sh`.

## Try other policies

Both proxies carry `api-key-auth` and `llm-cost`. Adding another one is an edit to
the proxy definition plus a redeploy:

1. Add it under `policies` in `llm-proxy-assistant.yaml`.
2. Apply the change:

   ```bash
   curl -X PUT http://localhost:9090/api/management/v1/llm-proxies/assistant-proxy \
     -u admin:admin \
     -H "Content-Type: application/yaml" \
     --data-binary @llm-proxy-assistant.yaml
   ```

3. Run `./load.sh` again and compare the events in Moesif.

`build.yaml` in the extracted distribution lists every policy this gateway ships
with, including guardrails such as `content-length-guardrail`, `word-count-guardrail`
and `regex-guardrail`, and rate limits such as `token-based-ratelimit` and
`llm-cost-based-ratelimit`. For worked guardrail examples, see
[`request-path-guardrails`](../request-path-guardrails).

## Using a real model provider

Replace the `upstream` block in `llm-provider.yaml` with the provider's endpoint and
credential, and remove the WireMock container from `setup.sh`. The analytics
configuration is unchanged: the gateway reads the usage block from whatever the
provider returns and prices the model named in the response.

## Teardown

```bash
./teardown.sh            # remove the resources, containers and volumes
./teardown.sh --clean    # also remove the extracted distribution and archive
```

Events already delivered to Moesif are stored in your Moesif account and are not
removed by teardown.
