# AI Gateway: Single LLM Proxy + Token-Based Rate Limiting

This sample demonstrates how the WSO2 AI Gateway enforces token-based rate limiting on an LLM proxy. A mock OpenAI backend is wired up locally so no real API key or cloud account is needed. The gateway tracks token usage from each response and blocks further requests once the quota is exhausted — returning a `429 Too Many Requests`.

## Prerequisites

- Docker and Docker Compose
- `curl`, `wget`, or `unzip` on your host

## Getting Started

Copy the environment file and start the stack:

```bash
cp .env.example .env
sh setup.sh
```

`setup.sh` does the following in order:

1. Downloads and unzips the official WSO2 AI Gateway distribution
2. Starts a WireMock container that stands in for the OpenAI API, serving a fixed mock response with 30 total tokens
3. Starts the WSO2 AI Gateway stack (controller + runtime) via Docker Compose
4. Waits for the gateway controller to become healthy
5. Registers the LLM provider, LLM proxy, and inbound API key via the management API
6. Polls the traffic endpoint until routes are live before exiting

Once `setup.sh` completes, the gateway is fully ready to accept requests.

## Try It Out

**Call 1 — first request, within quota (expect `200`):**

```bash
curl -sk -X POST https://localhost:8443/assistant/chat/completions \
  -H "api_key: demo-api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}'
```

Expected response:

```json
{
  "id": "chatcmpl-mock777",
  "object": "chat.completion",
  "model": "gpt-4o-mini",
  "choices": [{"message": {"role": "assistant", "content": "Hello! This response came through your local WSO2 AI Gateway — no real OpenAI call was made."}}],
  "usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}
}
```

**Call 2 — immediately after, quota exhausted (expect `429`):**

```bash
curl -sk -X POST https://localhost:8443/assistant/chat/completions \
  -H "api_key: demo-api-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}'
```

Expected response:

```json
{"error": "Too Many Requests", "message": "Rate limit exceeded. Please try again later."}
```

> The rate limit resets at the start of each fixed 1-minute window.

## Verify with test.sh

`test.sh` fires two back-to-back requests with the registered API key and asserts the first returns `200` and the second returns `429`, proving the rate limit is correctly enforced.

```bash
sh test.sh
```

Expected output:

```
=== WSO2 AI Gateway — LLM Proxy Test ===

Target  : https://localhost:8443/assistant/chat/completions
API Key : demo ****
Payload : {"model":"gpt-4o-mini","messages":[{"role":"user","content":"Test call"}]}

=== Test 1: Valid API key — expect HTTP 200 ===
Status   : HTTP 200
✔  HTTP 200 received as expected

=== Test 2: Token quota exceeded — expect HTTP 429 ===
Status   : HTTP 429
✔  HTTP 429 received as expected

✔  PASSED — Rate limiting is working correctly.
```

## How It Works

The mock LLM backend returns exactly 30 total tokens in every response. The `token-based-ratelimit` policy on the provider is configured with a quota of 30 total tokens per minute. The first request consumes the entire quota and succeeds. The second request finds the quota exhausted and is rejected with `429`.

## Configuration

| Variable          | Default                     | Description                                               |
|-------------------|-----------------------------|-----------------------------------------------------------|
| `INBOUND_API_KEY` | `demo-api-key`  | API key callers must send in the `api_key` header         |
| `MGMT_PORT`       | `9090`                      | Gateway management API port                               |
| `HEALTH_PORT`     | `9094`                      | Gateway health check port                                 |
| `TRAFFIC_PORT`    | `8443`                      | Gateway traffic port                                      |
| `MAX_RETRIES`     | `30`                        | Max readiness poll attempts before giving up (2s interval)|

## What's Running

| Container          | Role                            | Port  |
|--------------------|---------------------------------|-------|
| `wso2-gateway-runtime` | WSO2 AI Gateway (traffic) + policy engine | 8443  |
| `wso2-controller`      | Gateway control plane                     | 9090  |
| `mock-llm-openai`      | WireMock standing in for OpenAI           | 8082  |

## Teardown

```bash
sh teardown.sh
```
