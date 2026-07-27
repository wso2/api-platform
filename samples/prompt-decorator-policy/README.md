# LLM Gateway — Prompt Decorator Sample

> Companion sample for the [Prompt Decorator](https://wso2.com/api-platform/docs/ai-gateway/llm-proxy/prompt-management/prompt-decorator/) policy docs.

## Overview

This sample stands up a local WSO2 AI Gateway and shows the **Prompt Decorator** policy modifying prompts *before* they reach the LLM — without changing any application code. You run one script to set everything up, then one script per scenario to see the results.

The upstream is Anthropic's **OpenAI-compatible** endpoint (`https://api.anthropic.com/v1`), so the gateway's `openai` provider template works unchanged and the model used is a Claude model (`claude-sonnet-4-6`).

The Prompt Decorator policy prepends or appends content to fields in the request payload. This sample demonstrates both decoration modes against an OpenAI-compatible endpoint, giving you a reproducible environment to understand each behavior.

---

## What You Will Learn

By working through this sample you will understand how to:

- Apply **chat prompt decoration** — prepend a system message (a persona / standing instructions) to the `messages` array so the model behaves consistently, even when the caller sends no system message.
- Apply **text prompt decoration** — append an instruction to a message's `content` string so every reply follows a required format.

---

## Scenarios Covered

### Scenario 1 — Chat Prompt Decoration (prepend)

**What it does:** The `persona-proxy` prepends a `system` message to the `messages` array, giving the model a hotel-receptionist persona for the imaginary "ABC Horizon Resort". The caller sends only a plain user message.

**JSONPath:** `$.messages` · **append:** `false` (prepend)

### Scenario 2 — Text Prompt Decoration (append)

**What it does:** The `suffix-proxy` appends an instruction to the last message's `content`, telling the model to end every reply with the exact tag `[ABC-HORIZON-OK]`. The caller never asks for that tag.

**JSONPath:** `$.messages[-1].content` · **append:** `true`

---

## Expected Results

### Scenario 1 — Chat Prompt Decoration

A bare user message (`"Hi, I would like to book a room."`) is sent with **no** system message. Because the gateway injects the persona, the reply should welcome the guest to **ABC Horizon Resort** — a name the caller never typed.

```
[PASS] Chat decoration applied — persona injected ('ABC Horizon' present though the caller never sent it).
```

### Scenario 2 — Text Prompt Decoration

A plain question (`"What is the capital of France?"`) is sent with no formatting instructions. Because the gateway appends the suffix instruction, the reply should end with the tag `[ABC-HORIZON-OK]`.

```
[PASS] Text decoration applied — appended instruction honored ('[ABC-HORIZON-OK]' present though the caller never sent it).
```

---

## Prerequisites

| Tool | Purpose |
|---|---|
| Docker + Docker Compose | Runs the gateway stack |
| `wget` | Downloads the gateway distribution |
| `unzip` | Extracts the distribution |
| `python3` + `pyyaml` | Used by `teardown.sh` to read resource names from YAML (`pip install pyyaml`) |
| `curl` | Calls the gateway management API and proxy endpoint |
| `jq` | Used by the test scripts to build/parse JSON (`brew install jq`) |

---

## Required Configuration

### Anthropic API Key

The setup script injects the key into the LLM provider at deploy time. The
Anthropic account must have available credits, or requests return HTTP 400
(`Your credit balance is too low`). Provide the key via:

```bash
# Option A — environment variable (recommended)
export ANTHROPIC_API_KEY="sk-ant-..."

# Option B — script argument
./setup.sh sk-ant-...

# Option C — interactive prompt (key is hidden)
./setup.sh
```

The key is never written to disk; it is substituted into the provider payload at runtime and discarded.

---

## Files

```
llm-provider.yaml          LLM provider definition (Anthropic OpenAI-compatible upstream, access control)
llm-proxy-persona.yaml     Proxy with chat prompt decoration (prepend system persona)
llm-proxy-suffix.yaml      Proxy with text prompt decoration (append suffix instruction)
setup.sh                   Automated setup (download → start → deploy provider + proxies)
teardown.sh                Automated teardown (delete resources → stop stack)
test-chat-decoration.sh    Verifies the system persona is prepended
test-text-decoration.sh    Verifies the suffix instruction is appended
```

---

## Setup

```bash
./setup.sh
```

The script performs these steps in order:

1. Downloads `wso2apip-ai-gateway-1.1.0.zip`
2. Extracts the distribution
3. Starts the Docker Compose stack
4. Waits for the gateway to become healthy (polls up to 150 s)
5. Deploys the LLM provider
6. Deploys both LLM proxies

All steps are idempotent — re-running the script on an already-configured environment is safe.

### Endpoints After Setup

| Endpoint | URL |
|---|---|
| Chat decoration proxy | `http://localhost:8080/persona-proxy/chat/completions` |
| Text decoration proxy | `http://localhost:8080/suffix-proxy/chat/completions` |
| Gateway health | `http://localhost:9094/health` |
| Management API | `http://localhost:9090/api/management/v0.9` |

---

## Running the Tests

Each scenario has its own script and can be run independently. No API key is needed at test time — the gateway uses its stored credentials.

```bash
# Scenario 1 — chat prompt decoration (system persona)
./test-chat-decoration.sh

# Scenario 2 — text prompt decoration (appended suffix)
./test-text-decoration.sh
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
| Scenario 1: persona not detected | The model paraphrased away the hotel name — re-run; or verify the `prompt-decorator` policy in `llm-proxy-persona.yaml` |
| Scenario 2: tag not detected | The model dropped the tag — re-run; or verify the `prompt-decorator` policy in `llm-proxy-suffix.yaml` |
| HTTP 401 on management API | Basic auth header mismatch. `setup.sh` provisions the gateway admin credential (defaults to `admin`/`admin`); if you set `ADMIN_USERNAME`/`ADMIN_PASSWORD`, use the same values here. |
| Request errors with auth failure | Check that your Anthropic API key is valid |
| `HTTP 400` — `credit balance is too low` | The Anthropic account has no credits — add credits in the Anthropic console |
| Proxy returns `HTTP 500` — `Internal Server Error` | The `prompt-decorator` config is invalid; its `promptDecoratorConfig` must use `text` or `messages` (check the gateway-runtime logs) |
