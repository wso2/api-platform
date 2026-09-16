# AI Gateway: Token Usage and Cost in Moesif

This sample runs the WSO2 AI Gateway with analytics published to
[Moesif](https://www.moesif.com). Every LLM request that passes through the gateway
appears there with the model that served it, its token counts, and its cost.

Two proxies send traffic to two models of very different price, so the gap between
request volume and actual spend is visible immediately.

The integration is one environment variable: the distribution ships with analytics
enabled and Moesif as the publisher, and reads your Application ID from the
environment.

> For request rate, latency and traces on a self-hosted stack, see
> [`ai-gateway-observability`](../ai-gateway-observability). Metrics report how fast
> and how often; analytics report who used what, and at what cost. Token usage
> appears only in analytics.

## Prerequisites

- Docker with the Compose plugin
- `curl` (or `wget`), `unzip`, `jq` and `openssl`
- A Moesif account (the free tier is sufficient)

On Windows, run the sample from a WSL2 shell with Docker Desktop's WSL integration
enabled.

## Configure your Application ID

1. Sign in at <https://www.moesif.com>.
2. Copy the **Application ID** from **Settings → Installation**.
3. Provide it to the sample:

   ```bash
   cp .env.example .env
   # add the ID to .env
   ```

   Or export it: `export MOESIF_APPLICATION_ID='<your-application-id>'`

The Application ID is a write-only collector key. It is not a management API key,
and this sample does not require one. `.env` is git-ignored.

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

```bash
./load.sh
```

Sends a request every 0.25 seconds for 60 seconds: `gpt-4o-mini` through
`/assistant`, `gpt-4.1` through `/support`, with a fixed proportion of upstream
failures and rejected keys. Pass a duration to change the run length:
`./load.sh 120`. The token totals it prints are what Moesif should report.

Then open <https://www.moesif.com>.

## What to look for in Moesif

Moesif groups analytics into four dashboards, listed in the left sidebar:

| Dashboard | Covers |
|-----------|--------|
| Overview | all traffic across the platform |
| APIs | REST API proxies |
| LLM | LLM proxies |
| MCP | MCP proxies |

This sample sends LLM traffic, so **LLM** is the dashboard to open.

A default 60-second `./load.sh` produces around 240 requests and 48k tokens, which the
tiles report alongside the estimated cost. Each event behind those totals carries:

| Field | Contents |
|-------|----------|
| `metadata.aiMetadata.model` | the model that served the request |
| `metadata.aiMetadata.vendorName` | the provider |
| `metadata.aiMetadata.llmCost` | the cost of that request |
| `metadata.aiTokenUsage.promptTokens` | tokens in |
| `metadata.aiTokenUsage.completionTokens` | tokens out |
| `metadata.aiTokenUsage.totalTokens` | the total |
| `metadata.apiName` | the proxy that served it |

Build two charts to see the point of the sample:

- **Total tokens, grouped by `metadata.aiMetadata.model`.** Both models serve the same
  number of successful requests, but `gpt-4.1` returns longer responses, so it accounts
  for about 80% of the tokens.
- **The same chart on `metadata.aiMetadata.llmCost`.** `gpt-4.1` costs roughly thirteen
  times more per token, so it accounts for about 98% of the spend. Half the requests,
  almost all of the bill.

Usage is attributed per proxy and per model. Splitting it per consuming application
requires subscriptions, which this sample does not set up: an inbound API key is a
credential, not an identity, so `applicationName` is absent from the events.

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

Delivery to Moesif itself is confirmed in the Moesif UI; verifying it from a script
would require a management API key that the sample otherwise does not need.

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
