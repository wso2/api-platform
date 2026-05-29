# LLM Gateway — Failover Sample

Demonstrates three AI gateway policies running together on a local Docker stack:

| Policy | What it does |
|---|---|
| **Model round-robin** | Cycles requests across `gpt-4`, `gpt-3.5-turbo`, and `gpt-4-turbo`. Suspends a model for 60 s if it returns an error, providing automatic failover. |
| **Semantic cache** | Caches LLM responses in Redis using Mistral embeddings. Requests with ≥ 85 % semantic similarity are served from cache. |
| **PII masking** | Strips email addresses and phone numbers from request payloads before they reach OpenAI. |

---

## Prerequisites

| Tool | Purpose |
|---|---|
| Docker + Docker Compose | Runs the gateway stack |
| `wget` | Downloads the gateway distribution |
| `unzip` | Extracts the distribution |
| `python3` + `pyyaml` | Used by setup scripts to merge YAML/TOML files |
| `curl` | Calls the gateway management API and proxy endpoint |
| `jq` | Used by `test.sh` to parse API responses (`brew install jq`) |

---

## Required configuration

### 1. OpenAI API key — required for setup

The setup script injects the key into the LLM provider at deploy time. Provide it via:

```bash
# Option A — environment variable (recommended)
export OPENAI_API_KEY="sk-..."

# Option B — script argument
./setup.sh sk-...

# Option C — interactive prompt (key is hidden)
./setup.sh
```

The key is never written to disk; it is substituted into the provider payload at runtime and discarded.

### 2. Mistral API key — required for semantic cache

The semantic cache uses Mistral to generate embeddings. Open `additional-config.toml` and fill in the key:

```toml
embedding_provider_api_key = "your-mistral-api-key"
```

> Without this key the gateway starts successfully, but cache lookups silently fall through to OpenAI on every request. The `test.sh` semantic cache test will warn rather than fail in this case.

---

## Files

```
llm-provider.yaml       LLM provider definition (OpenAI upstream, access control)
llm-proxy.yaml          LLM proxy definition (three policies wired to /chat/completions)
redis-service.yaml      Redis Stack service, merged into docker-compose at setup time
additional-config.toml  Embedding + vector DB config, appended to gateway config.toml
setup.sh                Automated setup (download → configure → start → deploy)
teardown.sh             Automated teardown (delete resources → stop stack)
test.sh                 Policy verification tests (round-robin, cache, PII masking)
```

---

## Setup

```bash
./setup.sh
```

The script performs these steps in order:

1. Downloads `wso2apip-ai-gateway-1.1.0.zip`
2. Extracts the distribution
3. Appends `additional-config.toml` into `configs/config.toml`
4. Merges the Redis service into `docker-compose.yaml`
5. Starts the full Docker Compose stack
6. Waits for the gateway to become healthy (polls up to 150 s)
7. Deploys the LLM provider
8. Deploys the LLM proxy

All steps are idempotent — re-running the script on an already-configured environment is safe.

### Endpoints after setup

| Endpoint | URL |
|---|---|
| Gateway proxy (HTTP) | `http://localhost:8080/openai-proxy` |
| Gateway health | `http://localhost:9094/health` |
| Management API | `http://localhost:9090/api/management/v0.9` |

---

## Testing

```bash
./test.sh
```

Requires `jq`. The test script calls the gateway proxy directly — no API key needed at test time (the gateway uses its stored credentials).

### What each test verifies

**Test 1 — Model round-robin**
Sends three requests and reads the `model` field from each OpenAI response. Passes when at least two distinct models appear, confirming the gateway is cycling through `gpt-4 → gpt-3.5-turbo → gpt-4-turbo`.

**Test 2 — Semantic cache**
Sends the same question twice. Detects a cache hit via:
- A `HIT` value in any `X-Cache*` response header, **or**
- The second response being ≥ 3× faster than the first (LLM baseline is typically > 500 ms).

Requires `embedding_provider_api_key` in `additional-config.toml`.

**Test 3 — PII masking**
Sends a message containing a unique email and phone number and asks the model to repeat them verbatim. Because `redactPII: true` replaces the values before the request reaches OpenAI, the original strings should not appear anywhere in the response.

---

## Teardown

```bash
# Stop the stack and delete deployed resources
./teardown.sh

# Also remove the extracted directory and downloaded zip
./teardown.sh --clean
```

---

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| `setup.sh` fails at health check | Docker images are still pulling — wait and retry |
| Round-robin test: all requests use the same model | Gateway not yet handling the proxy; check `docker compose logs gateway-controller` |
| Semantic cache test: no hit detected | `embedding_provider_api_key` is empty in `additional-config.toml`, or Redis is not reachable |
| PII test: original values appear in response | PII regex did not match — verify the regex patterns in `llm-proxy.yaml` |
| HTTP 401 on management API | Basic auth header mismatch; default credentials are `admin:admin` |
