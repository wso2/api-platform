# LLM Gateway — Failover & AI Policy Sample

> **Sample source:** [wso2/api-platform — samples/llm-failover](https://github.com/wso2/api-platform/tree/main/samples/llm-failover)
>
> ```bash
> git clone https://github.com/wso2/api-platform.git
> cd api-platform/samples/llm-failover
> ```

## Overview

This guide is for platform engineers and API developers who want to add resilience, cost control, and data-safety policies to AI-powered applications — without changing application code.

When your application routes LLM traffic through the WSO2 AI Gateway, you can attach policies that handle failover, caching, and PII protection at the gateway layer. This sample sets up a local Docker stack that demonstrates all three policies working together against an OpenAI-compatible endpoint, giving you a reproducible environment to understand and test each behavior before deploying to production.

---

## What You Will Learn

By working through this sample you will understand how to:

- Enable **semantic caching** — serve repeated or semantically similar questions from a Redis cache, reducing latency and API cost
- Apply **PII masking** — strip sensitive identifiers (emails, phone numbers) from request payloads before they reach the upstream LLM provider

---

## Scenarios Covered

### Scenario 1 — Semantic Cache

**Problem:** Identical or near-identical questions (paraphrased, reworded) are sent to the LLM repeatedly, incurring cost and latency on every call.

**What this scenario does:** The gateway generates an embedding of each incoming prompt using Mistral and stores the LLM response in Redis. If a subsequent request is ≥ 85% semantically similar to a cached prompt, the cached response is returned immediately — no LLM call is made.

### Scenario 2 — PII Masking

**Problem:** Users sometimes include personal data (email addresses, phone numbers) in prompts. Sending this data to a third-party LLM provider may violate privacy policies or compliance requirements.

**What this scenario does:** Before the request leaves the gateway, a regex-based redaction policy replaces detected email addresses and phone numbers with masked placeholders. The upstream model never sees the original values.

---

## Expected Results

After running the test scripts you should observe the following for each scenario.

### Scenario 1 — Semantic Cache

The same question is sent twice. On the second request, the gateway should return a cached response. **The test detects a cache hit via:**

- A `HIT` value in any `X-Cache*` response header, **or**
- The second response arriving ≥ 3× faster than the first (LLM baseline is typically > 500 ms)

```
[PASS] Semantic cache: cache hit detected (header match / response time)
```

> If `embedding_provider_api_key` is not set in `additional-config.toml`, cache lookups silently fall through to OpenAI and the test will warn rather than fail.

### Scenario 2 — PII Masking

A prompt containing a unique email address and phone number is sent, asking the model to repeat them verbatim. Because the gateway redacts the values before forwarding the request, **neither the original email nor the phone number should appear in the response**.

```
[PASS] PII masking: original values not found in response
```

---

## Prerequisites

| Tool | Purpose |
|---|---|
| Docker + Docker Compose | Runs the gateway stack |
| `wget` | Downloads the gateway distribution |
| `unzip` | Extracts the distribution |
| `python3` + `pyyaml` | Used by setup scripts to merge YAML/TOML files ([install guide](https://www.python.org/downloads/) · `pip install pyyaml`) |
| `curl` | Calls the gateway management API and proxy endpoint |
| `jq` | Used by `test.sh` to parse API responses (`brew install jq`) |

---

## Required Configuration

### 1. OpenAI API Key

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

### 2. Mistral API Key (required for Scenario 2)

Generate a key from the Mistral console: <https://console.mistral.ai/api-keys>

The setup script injects the key into `additional-config.toml` at deploy time — you do not need to edit the sample files. Provide it via:

```bash
# Option A — environment variable (recommended)
export MISTRAL_API_KEY="..."

# Option B — interactive prompt (key is hidden)
./setup.sh
```

The setup script exits if this key is not provided.

---

## Files

```
llm-provider.yaml       LLM provider definition (OpenAI upstream, access control)
llm-proxy.yaml          LLM proxy definition (three policies wired to /chat/completions)
redis-service.yaml      Redis Stack service, merged into docker-compose at setup time
additional-config.toml  Embedding + vector DB config, appended to gateway config.toml
setup.sh                Automated setup (download → configure → start → deploy)
teardown.sh             Automated teardown (delete resources → stop stack)
test-semantic-cache.sh  Verifies semantic cache hits via Redis + Mistral embeddings
test-pii-masking.sh     Verifies email/phone redaction before requests reach OpenAI
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

### Endpoints After Setup

| Endpoint | URL |
|---|---|
| Gateway proxy (HTTP) | `http://localhost:8080/openai-proxy` |
| Gateway health | `http://localhost:9094/health` |
| Management API | `http://localhost:9090/api/management/v0.9` |

---

## Running the Tests

Each policy has its own script so you can run them independently. All scripts require `jq` and call the gateway proxy directly — no API key is needed at test time (the gateway uses its stored credentials).

```bash

# Scenario 1 — semantic cache
./test-semantic-cache.sh

# Scenario 2 — PII masking
./test-pii-masking.sh
```

### Customising the PII test

`test-pii-masking.sh` prompts for an email and phone number, or you can pass them via environment variables to run non-interactively:

```bash
TEST_EMAIL="you@example.com" TEST_PHONE="+15551234567" ./test-pii-masking.sh
```

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
| Scenario 1: no cache hit detected | `embedding_provider_api_key` is empty in `additional-config.toml`, or Redis is not reachable |
| Scenario 2: original values appear in response | PII regex did not match — verify the regex patterns in `llm-proxy.yaml` |
| HTTP 401 on management API | Basic auth header mismatch. `setup.sh` provisions the gateway admin credential (defaults to `admin`/`admin`); if you set `ADMIN_USERNAME`/`ADMIN_PASSWORD`, use the same values here. |
