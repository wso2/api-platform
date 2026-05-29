# AI Gateway: Single LLM Proxy + Token-Based Rate Limiting

Spin up a WSO2 AI Gateway that fronts a mock OpenAI backend and hard-caps token usage to **30 total tokens per minute** — the mock returns exactly 30 tokens per response, so the second call gets a `429`.

## Prerequisites

- Docker and Docker Compose
- `curl`, `wget`, and `unzip` on your host

## Getting Started

```bash
cp .env.example .env
sh run.sh
```

## Usage Examples

**Call 1 — within quota (expect `200`):**
```bash
curl -sk -X POST https://localhost:8443/assistant/chat/completions \
  -H "api_key: demo-unlocked-sample-key" \
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

**Call 2 immediately after — quota exhausted (expect `429`):**
```bash
curl -sk -X POST https://localhost:8443/assistant/chat/completions \
  -H "api_key: demo-unlocked-sample-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}'
```

Expected response:
```json
{"error": "Too Many Requests", "message": "Rate limit exceeded. Please try again later."}
```

## Verify with test.sh

```bash
sh test.sh
```

Expected output:
```
=== WSO2 AI Gateway — LLM Proxy Test ===

Target  : https://localhost:8443/assistant/chat/completions
API Key : demo-unlocked-****
Payload : {"model":"gpt-4o-mini","messages":[{"role":"user","content":"Test call"}]}

=== Test 1: Valid API key — expect HTTP 200 ===
Status   : HTTP 200
✔  HTTP 200 received as expected

=== Test 2: Token quota exceeded — expect HTTP 429 ===
Status   : HTTP 429
✔  HTTP 429 received as expected

✔  PASSED — Rate limiting is working correctly.
```

## Configuration

| Variable         | Default                    | Description                        |
|------------------|----------------------------|------------------------------------|
| `INBOUND_API_KEY`| `demo-unlocked-sample-key` | API key callers must send          |
| `MGMT_PORT`      | `9090`                     | Gateway management API port        |
| `HEALTH_PORT`    | `9094`                     | Gateway health check port          |
| `TRAFFIC_PORT`   | `8443`                     | Gateway traffic port               |

## Teardown

```bash
sh teardown.sh
```
