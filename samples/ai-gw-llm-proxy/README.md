# AI Gateway: Single LLM Proxy + API Key + Rate Limiting

Spins up a WSO2 AI Gateway that fronts a mock OpenAI backend, enforces API key auth, and hard-caps token usage to **30 total tokens per minute** — the mock returns exactly 30 tokens, so the second call gets a `429`.

### Prerequisites
- Docker and Docker Compose
- `curl`, `wget`, and `unzip` on your host

### Quick Start

```bash
cp .env.example .env
sh run.sh
```

### Verify It Works

```bash
sh test.sh
```

**Expected output:**
```
========================================
 WSO2 AI Gateway — LLM Proxy Test
========================================

Target  : https://localhost:8443/assistant/chat/completions
API Key : demo-unlocked-sample-key
Payload : {"model":"gpt-4o-mini","messages":[{"role":"user","content":"Test call"}]}

----------------------------------------
 Test 1: Valid API key — expect HTTP 200
----------------------------------------
Status   : HTTP 200
[OK]    ✔  HTTP 200 received as expected

----------------------------------------
 Test 2: Token quota exceeded — expect HTTP 429
----------------------------------------
Status   : HTTP 429
[OK]    ✔  HTTP 429 received as expected

========================================
 ✅ PASSED — API key auth and rate limiting are working correctly.
========================================
```

### How It Works

- **API key auth** is enforced at the proxy level — requests without a valid `api_key` header are rejected with `401`.
- **Rate limiting** is enforced at the provider level via `token-based-ratelimit` — the mock returns 30 total tokens per response, and the quota is set to 30 per minute, so the first call exhausts it and the second gets a `429`.

| Container          | Role                            | Port  |
|--------------------|---------------------------------|-------|
| `wso2-gateway`     | WSO2 AI Gateway (traffic)       | 8443  |
| `wso2-controller`  | WSO2 management + policy engine | 9090  |
| `mock-llm-openai`  | WireMock standing in for OpenAI | 8082  |

### Teardown

```bash
sh teardown.sh
```
